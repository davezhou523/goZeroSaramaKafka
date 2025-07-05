package kafka

import (
	"github.com/IBM/sarama"
	"log"
	"time"
)

func NewSyncProducer(brokers []string) sarama.SyncProducer {
	config := sarama.NewConfig()
	config.Producer.Retry.Max = 3
	config.Producer.Partitioner = sarama.NewHashPartitioner // 关键：按 Key 哈希分区
	config.Producer.RequiredAcks = sarama.WaitForAll        // 确保所有副本确认
	config.Producer.Return.Successes = true                 // 接收发送成功通知
	config.Producer.Return.Errors = true                    //接收发送失败通知
	config.Producer.MaxMessageBytes = 10 * 1024 * 1024      //10Mb
	config.Producer.Compression = sarama.CompressionZSTD    // 改用ZSTD平衡性能
	config.Producer.Idempotent = true                       // 启用幂等
	config.Net.MaxOpenRequests = 1                          // 幂等必需
	config.Producer.Timeout = 30 * time.Second              // 避免WaitForAll超时

	producer, err := sarama.NewSyncProducer(brokers, config)
	//producer, err := sarama.NewAsyncProducer(brokers, config)
	if err != nil {
		log.Fatalf("Failed to start Sarama producer: %v", err)
	}

	return producer
}

func SendMessage(producer sarama.SyncProducer, topic string, key, value string) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.StringEncoder(value),
	}

	_, _, err := producer.SendMessage(msg)
	return err
}
func SendImage(producer sarama.SyncProducer, topic string, key string, data []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(data), // 直接使用字节数组
	}
	_, _, err := producer.SendMessage(msg)

	return err
}
