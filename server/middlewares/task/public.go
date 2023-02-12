package task

import (
	"fmt"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	"math/rand"
	"public/conf"
	"public/declare"
	"time"
)

var conn *amqp.Connection
var NoticeChannel *amqp.Channel
var agentConfig declare.YamlServiceagentConfig

func RandomString(l int) string {
	rand.Seed(time.Now().UTC().UnixNano())
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(randInt(65, 90))
	}
	return string(bytes)
}

func randInt(min int, max int) int {
	return min + rand.Intn(max-min)
}

func InitRabbitmqConn() {
	var err error
	agentConfig, err = conf.GetServiceYamlConfig()
	if err != nil {
		panic("rpc包读取配置文件失败！！")
	}
	defer func() {
		if err := recover(); err != nil {
			time.Sleep(3 * time.Second)
			zap.S().Error("休息3秒后重新连接Rabbitmq")
			InitRabbitmqConn()
		}
	}()
	dsn := fmt.Sprintf("amqp://%s:%s@%s/", agentConfig.Rabbitmq.User, agentConfig.Rabbitmq.Password, agentConfig.Rabbitmq.HostPort)
	conn, err = amqp.Dial(dsn)
	if err != nil {
		fmt.Println(err)
	}

	NoticeChannel, err = conn.Channel()
	if err != nil {
		fmt.Println(err)
	}
	defer NoticeChannel.Close()
	closeChan := make(chan *amqp.Error, 1)
	notifyClose := NoticeChannel.NotifyClose(closeChan) //一旦消费者的channel有错误，产生一个amqp.Error，channel监听并捕捉到这个错误
	closeFlag := false
	for {
		select {
		case e := <-notifyClose:
			zap.S().Debug("捕获异常...等待3秒后重新建立连接")
			zap.S().Debug("chan通道错误,e:%s", e.Error())
			close(closeChan)
			time.Sleep(3 * time.Second)
			//StartAMQPConsume()
			closeFlag = true
		default:
			//zap.S().Debug("进入默认分支，等待5秒后继续检测")
			time.Sleep(3 * time.Second)
		}
		//zap.S().Debug("判断closeFlag是否为真，判断是否需要进行重新连接")
		if closeFlag {
			zap.S().Debug("closeFlag为真，跳出死循环重新连接")
			break
		}
	}
}
