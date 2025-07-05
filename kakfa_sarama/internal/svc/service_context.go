package svc

import (
	"github.com/IBM/sarama"
	"gozero1/kakfa_sarama/internal/config"
	"gozero1/kakfa_sarama/kafka"
)

type ServiceContext struct {
	Config        config.Config
	KafkaProducer sarama.SyncProducer
}

func NewServiceContext(c config.Config) *ServiceContext {
	producer := kafka.NewSyncProducer(c.Kafka.Brokers)

	return &ServiceContext{
		Config:        c,
		KafkaProducer: producer,
	}
}
