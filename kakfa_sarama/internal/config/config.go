package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf
	UploadFile struct {
		MaxFileNum  int
		MaxFileSize int
		SavePath    string
	}
	Kafka struct {
		Brokers []string
	}
}
