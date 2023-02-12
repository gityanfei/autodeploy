package main

import (
	"autoupdate/middlewares/task"
	"autoupdate/router"
	"go.uber.org/zap"
)

//type agentConfig router.agentConfig

func main() {
	//1、初始化Rabbitmq连接
	go task.InitRabbitmqConn()
	//2、启动服务
	err := router.Router().Run(":7777")
	if err != nil {
		zap.S().Error(err)
	}

}
