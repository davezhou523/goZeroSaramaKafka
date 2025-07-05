package kafka

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// 分片状态管理
type ChunkState struct {
	TotalChunks  int            // 总分片数
	Received     map[int][]byte // 已接收分片 (索引->数据)
	LastUpdated  time.Time      // 最后更新时间
	HasLastChunk bool           // 是否收到最后一片（含TotalChunks）
}

type ChunkAssembler struct {
	logger     *zap.Logger
	chunkState sync.Map      // fileId -> *ChunkState
	outputDir  string        // 文件输出目录
	timeout    time.Duration // 超时时长

	// 用于超时清理的跟踪结构
	fileTimestamps  map[string]time.Time
	timestampMutex  sync.Mutex
	cleanupTicker   *time.Ticker
	cleanupStopChan chan struct{}
}

func NewChunkAssembler(logger *zap.Logger, outputDir string, timeout time.Duration) *ChunkAssembler {
	assembler := &ChunkAssembler{
		logger:          logger,
		outputDir:       outputDir,
		timeout:         timeout,
		fileTimestamps:  make(map[string]time.Time),
		cleanupTicker:   time.NewTicker(10 * time.Second), // 每10秒检查一次超时
		cleanupStopChan: make(chan struct{}),
	}

	// 启动后台清理协程
	go assembler.runCleanupRoutine()
	return assembler
}

// 处理分片消息
func (a *ChunkAssembler) ProcessMessage(msg *sarama.ConsumerMessage) {
	var payload ChunkPayload
	if err := json.Unmarshal(msg.Value, &payload); err != nil {
		a.logger.Error("解码消息失败", zap.Error(err))
		return
	}

	fileId := payload.FileId
	//chunkIndex := payload.Index

	// 加载或初始化状态
	state, _ := a.chunkState.LoadOrStore(fileId, &ChunkState{
		Received:    make(map[int][]byte),
		LastUpdated: time.Now(),
	})
	currentState := state.(*ChunkState)

	// 更新状态
	a.timestampMutex.Lock()
	a.fileTimestamps[fileId] = time.Now()
	a.timestampMutex.Unlock()

	// 处理分片数据
	a.handleChunkData(payload, currentState)
	// 检查是否应触发文件组装
	if shouldAssemble := a.checkAssemblyCondition(payload, currentState); shouldAssemble {
		a.assembleFile(fileId, currentState)
	}
}

// 处理分片数据
func (a *ChunkAssembler) handleChunkData(payload ChunkPayload, state *ChunkState) {
	state.LastUpdated = time.Now()
	a.logger.Info("收到分片",
		zap.String("fileId", payload.FileId),
		zap.Int("index", payload.Index),
		zap.Int("TotalChunks", payload.TotalChunks))
	// 检查是否重复分片
	if _, exists := state.Received[payload.Index]; exists {
		a.logger.Error("收到重复分片",
			zap.String("fileId", payload.FileId),
			zap.Int("index", payload.Index))
		return
	}

	// 保存分片数据
	state.Received[payload.Index] = payload.Data

	// 更新总分片数（如果收到最后一片）
	if payload.TotalChunks > 0 {
		state.TotalChunks = payload.TotalChunks
		state.HasLastChunk = true
		a.logger.Info("收到最后一片",
			zap.String("fileId", payload.FileId),
			zap.Int("total", payload.TotalChunks))
	}
}

// 检查组装条件
func (a *ChunkAssembler) checkAssemblyCondition(payload ChunkPayload, state *ChunkState) bool {
	// 条件1: 已收到最后一片（包含TotalChunks）
	if !state.HasLastChunk {
		return false
	}

	// 条件2: 总分片数有效
	if state.TotalChunks <= 0 {
		a.logger.Error("无效总分片数",
			zap.String("fileId", payload.FileId),
			zap.Int("total", state.TotalChunks))
		return false
	}

	// 条件3: 已收到全部分片
	return len(state.Received) >= state.TotalChunks
}

// 组装文件
func (a *ChunkAssembler) assembleFile(fileId string, state *ChunkState) {
	// 检查分片完整性
	if len(state.Received) < state.TotalChunks {
		a.logger.Warn("分片不完整，等待更多分片",
			zap.String("fileId", fileId),
			zap.Int("received", len(state.Received)),
			zap.Int("expected", state.TotalChunks))
		return
	}

	// 按索引排序
	indices := make([]int, 0, len(state.Received))
	for index := range state.Received {
		indices = append(indices, index)
	}
	sort.Ints(indices)

	// 验证索引连续性
	if indices[len(indices)-1] != state.TotalChunks-1 {
		a.logger.Error("分片索引不连续",
			zap.String("fileId", fileId),
			zap.Int("max_index", indices[len(indices)-1]),
			zap.Int("expected_max", state.TotalChunks-1))
		return
	}

	// 合并文件内容
	var buffer bytes.Buffer
	for i := 0; i < state.TotalChunks; i++ {
		data, exists := state.Received[i]
		if !exists {
			a.logger.Error("缺失分片",
				zap.String("fileId", fileId),
				zap.Int("index", i))
			return
		}
		buffer.Write(data)
	}

	// 写入文件
	outputPath := filepath.Join(a.outputDir, fileId)

	if err := os.WriteFile(outputPath, buffer.Bytes(), 0644); err != nil {
		a.logger.Error("文件写入失败",
			zap.String("fileId", fileId),
			zap.String("path", outputPath),
			zap.Error(err))
		return
	}

	a.logger.Info("文件组装完成",
		zap.String("fileId", fileId),
		zap.String("path", outputPath),
		zap.Int("total_chunks", state.TotalChunks))

	// 清理状态
	a.chunkState.Delete(fileId)
	a.timestampMutex.Lock()
	delete(a.fileTimestamps, fileId)
	a.timestampMutex.Unlock()
}

// 后台清理协程
func (a *ChunkAssembler) runCleanupRoutine() {
	for {
		select {
		case <-a.cleanupTicker.C:
			a.cleanupExpiredChunks()
		case <-a.cleanupStopChan:
			a.cleanupTicker.Stop()
			return
		}
	}
}

// 清理过期分片状态
func (a *ChunkAssembler) cleanupExpiredChunks() {
	a.timestampMutex.Lock()
	defer a.timestampMutex.Unlock()

	now := time.Now()
	for fileId, lastUpdated := range a.fileTimestamps {
		if now.Sub(lastUpdated) > a.timeout {
			a.logger.Warn("清理超时分片状态",
				zap.String("fileId", fileId),
				zap.Duration("timeout", a.timeout))

			// 获取当前状态
			if state, ok := a.chunkState.Load(fileId); ok {
				chunkState := state.(*ChunkState)
				a.logger.Warn("分片状态详情",
					zap.Int("received", len(chunkState.Received)),
					zap.Int("expected", chunkState.TotalChunks),
					zap.Bool("has_last", chunkState.HasLastChunk))
			}

			// 清理状态
			a.chunkState.Delete(fileId)
			delete(a.fileTimestamps, fileId)
		}
	}
}

// 关闭资源
func (a *ChunkAssembler) Close() {
	close(a.cleanupStopChan)
}

// Kafka消费者集成示例
func StartKafkaConsumer(brokers []string, topics []string, groupID string) {
	config := sarama.NewConfig()
	config.Consumer.Offsets.AutoCommit.Enable = false     // 禁用自动提交
	config.Consumer.Offsets.Initial = sarama.OffsetNewest // 从最新偏移开始
	config.Version = sarama.V2_1_0_0
	config.Consumer.MaxProcessingTime = 5 * time.Minute
	config.Consumer.Fetch.Max = 10 * 1024 * 1024
	// 创建消费者
	consumer, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		panic(fmt.Sprintf("创建消费者组失败: %v", err))
	}

	logger, _ := zap.NewProduction()
	defer func(logger *zap.Logger) {
		err := logger.Sync()
		if err != nil {

		}
	}(logger)

	assembler := NewChunkAssembler(logger, "./upload", 5*time.Minute)
	defer assembler.Close()

	// 消费处理
	ctx := context.Background()
	for {
		err := consumer.Consume(ctx, topics, &ConsumerHandler{assembler: assembler})
		if err != nil {
			logger.Error("消费错误", zap.Error(err))
			time.Sleep(5 * time.Second)
		}
	}
}

// 消费者组处理器
type ConsumerHandler struct {
	assembler *ChunkAssembler
}

func (h *ConsumerHandler) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *ConsumerHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

func (h *ConsumerHandler) ConsumeClaim(
	session sarama.ConsumerGroupSession,
	claim sarama.ConsumerGroupClaim) error {

	for msg := range claim.Messages() {
		h.assembler.ProcessMessage(msg)
		// 处理完成后提交位移（至少一次语义）
		session.MarkMessage(msg, "")
		session.Commit()
	}
	return nil
}
