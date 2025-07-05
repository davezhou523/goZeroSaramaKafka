package main

import (
	"flag"
	"fmt"
	"gozero1/kakfa_sarama/internal/config"
	"gozero1/kakfa_sarama/internal/handler"
	"gozero1/kakfa_sarama/internal/svc"
	"gozero1/kakfa_sarama/kafka"
	"os"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/kakfa_sarama-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()
	savePath := "./upload/"
	_ = os.MkdirAll(savePath, os.ModePerm)
	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)
	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	go kafka.StartKafkaConsumer(c.Kafka.Brokers, []string{"test-topic"}, "group-1")
	server.Start()
}
