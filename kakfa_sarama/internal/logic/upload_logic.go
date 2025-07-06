package logic

import (
	"context"
	"github.com/zeromicro/go-zero/core/errorx"
	"github.com/zeromicro/go-zero/core/jsonx"
	"gozero1/kakfa_sarama/internal/svc"
	"gozero1/kakfa_sarama/internal/types"
	"gozero1/kakfa_sarama/kafka"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/zeromicro/go-zero/core/logx"
)

type UploadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUploadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UploadLogic {
	return &UploadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}
func (l *UploadLogic) SendFileToKafka(file multipart.File, filename string) (*types.UploadResponse, error) {
	//fileId := uuid.New().String() // 生成文件唯一ID
	fileId := filename           // 生成文件唯一ID
	chunkSize := 1 * 1024 * 1024 // 1MB分片
	buffer := make([]byte, chunkSize)
	chunkIndex := 0
	payload := new(kafka.ChunkPayload)
	payload.FileId = fileId
	payload.Category = "image"

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			l.Logger.Error("发送分片读取文件错误:", err)
			return nil, err // 非EOF错误立即退出
		}
		l.Logger.Info("发送分片 ", "数量:", n, err)
		if n > 0 {
			// 构建分片消息
			payload.Index = chunkIndex
			payload.TotalChunks = -1 // 默认未设置总分片数
			// 若是最后一片，设置总分片数
			if err == io.EOF {
				payload.TotalChunks = chunkIndex + 1
			}
			l.Logger.Info("发送分片 ", "fileId:", fileId, " index:", chunkIndex, " TotalChunks:", payload.TotalChunks)
			payload.Data = buffer[:n]
			content, _ := jsonx.Marshal(payload)
			err = kafka.SendMessage(l.svcCtx.KafkaProducer, "test-topic", fileId, content)
			if err != nil {
				logx.Errorf("分片发送失败: index=%d, err=%v", chunkIndex, err)
				return nil, err
			}
			chunkIndex++
		}
		if chunkIndex > 0 && err == io.EOF {
			// 发送最后一片时设置总分片数
			payload.Index = chunkIndex
			payload.TotalChunks = chunkIndex + 1
			content, _ := jsonx.Marshal(payload)
			err = kafka.SendMessage(l.svcCtx.KafkaProducer, "test-topic", fileId, content)
			break

		} else if err != nil {
			return &types.UploadResponse{
				Code:    -1,
				Message: "不能为空文件",
			}, nil
		}

	}
	// 3. 返回成功响应
	return &types.UploadResponse{
		Code:    200,
		Message: "上传成功",
	}, nil
}

func (l *UploadLogic) Upload(file multipart.File, filename string) (*types.UploadResponse, error) {
	// 1. 创建本地文件
	savePath := filepath.Join(l.svcCtx.Config.UploadFile.SavePath, filename)
	out, err := os.Create(savePath)
	if err != nil {
		return nil, errorx.Wrap(err, "创建文件失败")
	}
	defer out.Close()

	// 2. 复制文件流
	_, err = io.Copy(out, file)
	if err != nil {
		return nil, errorx.Wrap(err, "保存文件失败")
	}

	// 3. 返回成功响应
	return &types.UploadResponse{
		Code:    200,
		Message: "上传成功",
	}, nil
}
