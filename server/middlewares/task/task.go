package task

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	"time"

	"public/declare"
	"sync"
)

var GlobalTaskManager = &RpcTaskManager{
	CloseTask:     make(chan *declare.RabbitmqMsg),
	NoticeRpcTask: make(map[string]*NoticeRpcTask, 10000),
	RpcTask:       make(map[string]*RpcTask, 10000),
}

type RpcTaskManager struct {
	NoticeChannel *amqp.Channel
	CloseTask     chan *declare.RabbitmqMsg
	NoticeRpcTask map[string]*NoticeRpcTask
	RpcTask       map[string]*RpcTask
}
type NoticeRpcTask struct {
	ConsumerQueueChan <-chan amqp.Delivery
	Exchange          string
	BackQueueName     string
	CorrId            string
}
type RpcTask struct {
	TaskName          string
	Channel           *amqp.Channel
	ConsumerQueueChan <-chan amqp.Delivery
	Exchange          string
	BackQueueName     string
	RoutingKey        string
	ReceiveQueueName  string
	CorrId            string
	IsSSH             bool
	Close             chan string
	WriteRusult       chan []byte
	mutex             sync.Mutex // 避免重复关闭管道
	isClosed          bool
	WsConnection      *WsConnection
}
type RabbitmqMsg declare.RabbitmqMsg

func InitTask(clientMsg *declare.RabbitmqMsg) (msg *RabbitmqMsg) {
	msg = &RabbitmqMsg{
		Loop:       clientMsg.Loop,
		Exchange:   clientMsg.Exchange,
		RoutingKey: clientMsg.RoutingKey,
		TaskName:   clientMsg.TaskName,
		TaskAction: clientMsg.TaskAction,
		Action:     clientMsg.Action,
		Params:     clientMsg.Params,
		ReceiveMsg: clientMsg.ReceiveMsg,
	}
	return
}
func InitNotice(name string) {
	err := NoticeChannel.ExchangeDeclare(
		agentConfig.Rabbitmq.Exchange, // name
		"direct",                    // type
		true,                        // durable
		false,                       // auto-deleted
		false,                       // internal
		false,                       // no-wait
		nil,                         // arguments
	)
	if err != nil {
		zap.S().Debug("定义通知交换机失败", err)
		return
	}
	//这里是定义接收处理结果的随机名queue
	q, err := NoticeChannel.QueueDeclare(
		"",    // name
		false, // durable
		true,  // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		zap.S().Debug("定义通知响应queue失败", err)
		return
	}
	//持续监听这个随机名queue中的数据
	noticeRusults, err := NoticeChannel.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		zap.S().Debug("定义通知服务提供者失败", err)
		return
	}
	corrId := RandomString(32)
	GlobalTaskManager.NoticeRpcTask[name] = &NoticeRpcTask{
		ConsumerQueueChan: noticeRusults,
		Exchange:          agentConfig.Rabbitmq.Exchange,
		BackQueueName:     q.Name,
		CorrId:            corrId,
	}
	zap.S().Debug("【创建任务通知】创建", name, "创建成功")
}

func (clientMsg *RabbitmqMsg) SendCreateTask(wsConn *WsConnection) (result *[]byte, err error) {
	var sendMsg []byte
	newTask := clientMsg.Task(wsConn)
	GlobalTaskManager.RpcTask[clientMsg.TaskName] = newTask
	clientMsg.ReceiveMsg.BackQueueName = GlobalTaskManager.RpcTask[clientMsg.TaskName].BackQueueName
	InitNotice(clientMsg.TaskName)
	sendMsg, err = json.Marshal(clientMsg)
	if err != nil {
		zap.S().Debug("sendMsg: ", string(sendMsg))
		return
	}
	err = NoticeChannel.Publish(
		clientMsg.Exchange,   // exchange
		clientMsg.RoutingKey, // routing key
		false,                // mandatory
		false,                // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			ReplyTo:       GlobalTaskManager.NoticeRpcTask[clientMsg.TaskName].BackQueueName,
			CorrelationId: GlobalTaskManager.NoticeRpcTask[clientMsg.TaskName].CorrId,
			Body:          sendMsg,
		})
	if err != nil {
		zap.S().Debug("发送消息失败")
		return
	}
	zap.S().Debug("【创建任务通知】", clientMsg.TaskName, "发送的内容为：", string(sendMsg))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	select {
	case d := <-GlobalTaskManager.NoticeRpcTask[clientMsg.TaskName].ConsumerQueueChan:
		zap.S().Debug("【创建任务通知】 ", clientMsg.TaskName, "接收到的", GlobalTaskManager.NoticeRpcTask[clientMsg.TaskName].BackQueueName, "队列返回的内容为：", string(d.Body))
		err = d.Ack(false)
		if err != nil {
			zap.S().Debug("消息通知Ack失败", err.Error())
		}
		if d.CorrelationId == GlobalTaskManager.NoticeRpcTask[clientMsg.TaskName].CorrId {
			if CreateStatus(d.Body) {
				zap.S().Debug("【创建任务通知结果】", clientMsg.TaskName, "接收到通知回执通道创建成功")
			} else {
				//zap.S().Debug("【创建任务通知结果】agent创建任务失败", d.ReplyTo)
				return nil, errors.New("接收到消息，但是agent创建任务失败" + d.ReplyTo)
			}
		}
	case <-ctx.Done():
		zap.S().Debug("【创建任务通知结果】接收通知响应超时", clientMsg.TaskName)
		return nil, errors.New("接收通知响应超时")
	}

	if !clientMsg.Loop {
		clientMsg.WriteMQ()
		var tmp = []byte{}
		tmp, err = clientMsg.ReadMQ()
		if err != nil {
			return
		}
		result = &tmp
		clientMsg.TaskAction = false
		err = clientMsg.CloseTask()
		if err != nil {
			return
		}
		return
	} else {
		go GlobalTaskManager.RpcTask[clientMsg.TaskName].WsConnection.wsReadLoop()
		go GlobalTaskManager.RpcTask[clientMsg.TaskName].WsConnection.wsWriteLoop()
	}

	return
}
func CreateStatus(testbyte []byte) bool {
	var tmp = declare.ResultMsg{}
	err := json.Unmarshal(testbyte, &tmp)
	if err != nil {
		zap.S().Error("Json解析失败:", err.Error())
	}
	if tmp.Code == declare.StatusOk {
		return true
	} else {
		return false
	}
}
func (clientMsg *RabbitmqMsg) Task(wsConn *WsConnection) (newTask *RpcTask) {
	newTaskChannel, err := conn.Channel()
	if err != nil {
		zap.S().Error("初始化channel失败:", err.Error())
	}
	zap.S().Debug("定义交换机和消费者队列", clientMsg.ReceiveMsg.Exchange, clientMsg.ReceiveMsg.ReceiveQueueName, clientMsg.ReceiveMsg.BackQueueName)
	err = newTaskChannel.ExchangeDeclare(
		clientMsg.ReceiveMsg.Exchange, // name
		"direct",                      // type
		true,                          // durable
		false,                         // auto-deleted
		false,                         // internal
		false,                         // no-wait
		nil,                           // arguments
	)
	if err != nil {
		zap.S().Error(err.Error())
		return
	}

	q, err := newTaskChannel.QueueDeclare(
		"",    // name
		false, // durable
		true,  // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		zap.S().Error(err.Error())
		return
	}
	err = newTaskChannel.Qos(
		1000,  // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		zap.S().Error("任务QOS定义失败：", err.Error())
		return
	}
	consumers, err := newTaskChannel.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)

	newTask = &RpcTask{
		TaskName:          clientMsg.TaskName,
		Channel:           newTaskChannel,
		ConsumerQueueChan: consumers,
		Exchange:          clientMsg.ReceiveMsg.Exchange,
		BackQueueName:     q.Name,
		RoutingKey:        clientMsg.TaskName,
		ReceiveQueueName:  clientMsg.TaskName,
		CorrId:            clientMsg.TaskName,
		IsSSH:             false,
		Close:             make(chan string),
		isClosed:          false,
		WsConnection: &WsConnection{
			wsSocket:             wsConn.wsSocket, // 底层websocket
			TaskName:             clientMsg.TaskName,
			inChan:               wsConn.inChan,  // 读取队列
			outChan:              wsConn.outChan, // 发送队列
			Ch:                   newTaskChannel,
			isClosed:             wsConn.isClosed,
			CloseWsReadLoopChan:  wsConn.CloseWsReadLoopChan,  // 关闭通知
			WsReadLoopOkChan:     wsConn.WsReadLoopOkChan,     // 关闭通知
			WsReadLoopFalseChan:  wsConn.WsReadLoopFalseChan,  // 关闭通知
			CloseWsWriteLoopChan: wsConn.CloseWsWriteLoopChan, // 关闭通知
			CloseChan:            wsConn.CloseChan,            // 关闭通知
			receiveMsg:           consumers,                   //一个channel，在发送消息后，还需要监听一个队列回复回来的消息
			receiveQueueName:     clientMsg.TaskName,          //监听接收数据的queue名
			corrId:               clientMsg.TaskName,          //客户端回复的ID
			RoutingKey:           clientMsg.TaskName,
		},
	}
	//zap.S().Debug("新任务的值为：", newTask)
	return newTask
}
func (clientMsg *RabbitmqMsg) WriteMQ() {
	var sendMsg []byte
	sendMsg, _ = json.Marshal(clientMsg)
	err := GlobalTaskManager.RpcTask[clientMsg.TaskName].Channel.Publish(
		clientMsg.ReceiveMsg.Exchange,   // exchange
		clientMsg.ReceiveMsg.RoutingKey, // routing key
		false,                           // mandatory
		false,                           // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			CorrelationId: clientMsg.ReceiveMsg.CorrId,
			ReplyTo:       GlobalTaskManager.RpcTask[clientMsg.TaskName].BackQueueName,
			Body:          sendMsg,
		})
	if err != nil {
		zap.S().Debug("推送消息失败：", err)
	}
	zap.S().Error("【任务队列】", clientMsg.TaskName, "发送的消息为：", string(sendMsg))

}
func (clientMsg *RabbitmqMsg) ReadMQ() (resultMsg []byte, err error) {

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	select {
	case d := <-GlobalTaskManager.RpcTask[clientMsg.TaskName].ConsumerQueueChan:
		zap.S().Debug("【任务队列】", clientMsg.TaskName, "接收到", GlobalTaskManager.RpcTask[clientMsg.TaskName].BackQueueName, "队列的消息为：", string(d.Body))
		if d.CorrelationId == GlobalTaskManager.RpcTask[clientMsg.TaskName].CorrId {
			resultMsg = d.Body
		}
		err = d.Ack(false)
		if err != nil {
			zap.S().Debug("任务Ack失败", err)
		}
	case <-ctx.Done():
		err = errors.New("ReadMQ Task接收响应超时")
	}
	return
}
func (clientMsg *RabbitmqMsg) CloseTask() (err error) {
	var sendMsg []byte
	clientMsg.TaskAction = false
	sendMsg, _ = json.Marshal(clientMsg)

	err = NoticeChannel.Publish(
		clientMsg.Exchange,   // exchange
		clientMsg.RoutingKey, // routing key
		false,                // mandatory
		false,                // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			CorrelationId: GlobalTaskManager.NoticeRpcTask[clientMsg.TaskName].CorrId,
			ReplyTo:       GlobalTaskManager.NoticeRpcTask[clientMsg.TaskName].BackQueueName,
			Body:          sendMsg,
		})
	if err != nil {
		zap.S().Debug("推送关闭通道消息失败：", err)
		return
	}
	if clientMsg.Loop {
		zap.S().Debug("【WS任务】向", clientMsg.TaskName, "任务发送读写协程关闭信息")
		GlobalTaskManager.RpcTask[clientMsg.TaskName].WsConnection.CloseWsReadLoopChan <- clientMsg.TaskName
		GlobalTaskManager.RpcTask[clientMsg.TaskName].WsConnection.CloseWsWriteLoopChan <- clientMsg.TaskName
		time.Sleep(time.Second * 1)
	}
	err = GlobalTaskManager.RpcTask[clientMsg.TaskName].Channel.Close()
	if err != nil {
		zap.S().Debug("关闭Task通道：", err)
		return
	}
	a, err2 := NoticeChannel.QueueDelete(GlobalTaskManager.NoticeRpcTask[clientMsg.TaskName].BackQueueName, false, false, true)
	if err2 != nil {
		zap.S().Error("关闭通知队列：", a, err.Error())
		return
	}
	delete(GlobalTaskManager.RpcTask, clientMsg.TaskName)
	delete(GlobalTaskManager.NoticeRpcTask, clientMsg.TaskName)
	zap.S().Info("【删除队列】 删除的任务名为", clientMsg.TaskName)
	return
}
