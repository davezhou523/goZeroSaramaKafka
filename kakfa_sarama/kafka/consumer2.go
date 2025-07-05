package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/zeromicro/go-zero/core/logx"
	"log"
	"os"
	"time"
)

type Consumer struct{}

func (c *Consumer) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		fmt.Printf("key:%v\n", string(msg.Key))
		data := new(ChunkPayload)
		err := json.Unmarshal(msg.Value, &data)
		//fmt.Println(data)
		if err != nil {
			fmt.Println(err)
			return err
		}
		if string(data.Category) == "image" {

			go saveImage(msg.Value, string(msg.Key)) // 异步保存
			log.Printf("Consumed message:Partition=%v  topic=%s key=%s  value length:%v", msg.Partition, msg.Topic, string(msg.Key), len(msg.Value))

		} else {
			//log.Printf("Consumed message:Partition=%v  topic=%s key=%s value=%s", msg.Partition, msg.Topic, string(msg.Key), string(msg.Value))

		}

		session.MarkMessage(msg, "")
	}
	return nil
}

func saveImage(data []byte, fileName string) {
	// 生成唯一文件名（避免冲突）
	filePath := fmt.Sprintf("./images/%s_%d.jpg", fileName, time.Now().UnixNano())
	// 原子写入（防止写入中断）
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		logx.Errorf("写入失败: %v", err)
		return
	}
	err := os.Rename(tmpPath, filePath)
	if err != nil {
		fmt.Println(err)
		return
	} // 原子操作更名
}
func StartConsumerGroup(brokers []string, groupID string, topics []string) {
	config := sarama.NewConfig()
	config.Version = sarama.V2_1_0_0
	config.Consumer.MaxProcessingTime = 5 * time.Minute
	config.Consumer.Fetch.Max = 10 * 1024 * 1024
	// 单分区单线程消费（避免乱序）
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	consumer := &Consumer{}
	client, err := sarama.NewConsumerGroup(brokers, groupID, config)
	if err != nil {
		log.Fatalf("Error creating consumer group client: %v", err)
	}
	fmt.Printf("kafka consumer start\n")

	ctx := context.Background()
	for {
		if err := client.Consume(ctx, topics, consumer); err != nil {
			log.Printf("Error from consumer: %v", err)
		}
	}
}
