package main

import (
	"agent/logger"
	"agent/rpc"
	"go.uber.org/zap"
	"time"
)

func main() {
	err := logger.InitLogger()
	if err != nil {
		zap.S().Error("FAILED")
	}
	// 1、初始化Rabbitmq
	go rpc.InitRabbitmqConn()
	time.Sleep(2 * time.Second)
	// 2、开始持续接收消息

	rpc.GlobalTaskManager.Start()

}
