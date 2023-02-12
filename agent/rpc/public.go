package rpc

import (
	"fmt"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	"log"
	"public/conf"
	"public/declare"
	"time"
)

var conn *amqp.Connection
var ManagerChannel *amqp.Channel
var agentConfig declare.YamlAgentagentConfig

func InitRabbitmqConn() {
	var err error
	agentConfig, err = conf.GetAgentYamlConfig()
	if err != nil {
		panic("rpc包读取配置文件失败！！")
	}
	defer func() {
		if errDefer := recover(); errDefer != nil {
			time.Sleep(3 * time.Second)
			zap.S().Debug("休息3秒后重新连接Rabbitmq")
			InitRabbitmqConn()
		}
	}()
	dsn := fmt.Sprintf("amqp://%s:%s@%s/", agentConfig.Rabbitmq.User, agentConfig.Rabbitmq.Password, agentConfig.Rabbitmq.HostPort)
	zap.S().Debug(dsn)
	conn, err = amqp.Dial(dsn)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()
	ManagerChannel, err = conn.Channel()
	if err != nil {
		fmt.Println(err)
	}
	defer ManagerChannel.Close()
	closeChan := make(chan *amqp.Error, 2)
	InitManager()
	notifyClose := ManagerChannel.NotifyClose(closeChan) //一旦消费者的channel有错误，产生一个amqp.Error，channel监听并捕捉到这个错误
	closeFlag := false
	for {
		select {
		case e := <-notifyClose:
			zap.S().Debug("捕获异常...等待5秒后重新建立连接")
			log.Printf("chan通道错误,e:%v\n", e.Error())
			close(closeChan)
			time.Sleep(5 * time.Second)
			//StartAMQPConsume()
			closeFlag = true
		default:
			time.Sleep(5 * time.Second)
		}
		if closeFlag {
			zap.S().Debug("closeFlag为真，跳出死循环重新连接")
			break
		}
	}
}
