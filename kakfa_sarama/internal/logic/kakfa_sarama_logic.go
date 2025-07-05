package logic

import (
	"context"
	"gozero1/kakfa_sarama/kafka"

	"github.com/zeromicro/go-zero/core/logx"
	"gozero1/kakfa_sarama/internal/svc"
)

type SendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendLogic {
	return &SendLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *SendLogic) Send() error {
	return kafka.SendMessage(l.svcCtx.KafkaProducer, "test-topic", "key111", "")
}
