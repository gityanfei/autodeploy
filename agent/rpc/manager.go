package rpc

import (
	"agent/rpc/task"
	"encoding/json"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	"public/declare"
	"time"
)

var GlobalTaskManager *RpcTaskManager

type RpcTaskManager struct {
	CloseTask     chan string
	NoticeRpcTask NoticeRpcTask
	RpcTask       map[string]*task.RpcTask
}
type NoticeRpcTask struct {
	ConsumerQueueChan <-chan amqp.Delivery
	Exchange          string
	ReceiveQueueName  string
	RoutingKey        string
}

func InitManager() {
	err := ManagerChannel.ExchangeDeclare(
		agentConfig.Rabbitmq.Exchange, // name
		"direct",                    // type
		true,                        // durable
		false,                       // auto-deleted
		false,                       // internal
		false,                       // no-wait
		nil,                         // arguments
	)
	if err != nil {
		zap.S().Debug("定义创建任务通知Exchange失败:", err)
		return
	}

	q, err := ManagerChannel.QueueDeclare(
		agentConfig.Rabbitmq.ActionQueueName, // name
		true,                               // durable
		false,                              // delete when unused
		false,                              // exclusive
		false,                              // no-wait
		nil,                                // arguments
	)
	if err != nil {
		zap.S().Debug("定义创建任务通知Queue失败:", err)
		return
	}
	err = ManagerChannel.QueueBind(
		q.Name,                              // queue name
		agentConfig.Rabbitmq.ActionRoutingKey, // routing key
		agentConfig.Rabbitmq.Exchange,         // exchange
		false,
		nil)
	if err != nil {
		zap.S().Debug("定义创建任务通知Bind失败:", err)
		return
	}
	err = ManagerChannel.Qos(
		1000,  // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		zap.S().Debug("定义创建任务通知Qos失败:", err)
		return
	}

	msgs, err := ManagerChannel.Consume(
		agentConfig.Rabbitmq.ActionQueueName, // queue
		"",                                 // consumer
		true,                               // auto-ack
		false,                              // exclusive
		false,                              // no-local
		false,                              // no-wait
		nil,                                // args
	)
	if err != nil {
		zap.S().Debug("定义创建任务通知Consume失败:", err)
		return
	}
	GlobalTaskManager = &RpcTaskManager{
		CloseTask: make(chan string, 1000),
		NoticeRpcTask: NoticeRpcTask{
			ConsumerQueueChan: msgs,
			Exchange:          agentConfig.Rabbitmq.Exchange,
			ReceiveQueueName:  agentConfig.Rabbitmq.ActionQueueName,
			RoutingKey:        agentConfig.Rabbitmq.ActionRoutingKey,
		},
		RpcTask: make(map[string]*task.RpcTask, 1000),
	}
	zap.S().Debug("初始化管理端完成")
	return
}

func (r *RpcTaskManager) Start() {
	for {
		select {
		case d := <-r.NoticeRpcTask.ConsumerQueueChan:
			if len(d.Body)==0{
				break
			}
			zap.S().Info("Rabbitmq接收到的信息为：%s", string(d.Body))
			msg := declare.RabbitmqMsg{}
			err := json.Unmarshal(d.Body, &msg)
			if err != nil {
 				zap.S().Debug("json序列化失败,发送的格式不符合JSON格式要求")
				break
			}
			if re := TestActionAllow(msg.Action); !re {
				err = r.ManagerPushMsg([]byte(`{"Code":"999999","Desc":"当前Agent类型与Action不匹配","Result":null}`), &d)
				if err != nil {
					zap.S().Debug("推送消息失败:", err)
					return
				}
				break
			}
			if msg.TaskName == "" {
				zap.S().Debug("发送的格式不符合JSON格式要求")
				err = r.ManagerPushMsg([]byte(`{"Code":"999999","Desc":"发送的格式不符合JSON格式要求","Result":null}`), &d)
				if err != nil {
					zap.S().Debug("推送消息失败:", err)
					return
				}
				break
			}
			if msg.TaskAction {
				go r.NewTask(&d, msg)
			} else {
				zap.S().Debug("【关闭通知】从Rabbitmq中接收到关闭Agent任务1：", msg.TaskName)
				r.CloseTask <- msg.TaskName
			}
		case taskName := <-r.CloseTask:
			zap.S().Debug("【关闭通知】管理端CloseTask通道接收到关闭Agent任务2：", taskName)
			go r.ManagerCloseTask(taskName)
		default:
			time.Sleep(time.Millisecond * 100)
		}
	}
}
func (r *RpcTaskManager) NewTask(currentExecMsg *amqp.Delivery, currentMsg declare.RabbitmqMsg) {
	var result = declare.Result{}
	var newTask = new(task.RpcTask)
	for _, v := range r.RpcTask {
		zap.S().Debug("当前任务列表:", v)
	}
	var err error
	newTask, err = task.CreateTask(currentMsg)
	if err != nil {
		result = declare.Result{
			Code:   declare.StatusFailed,
			Desc:   "创建通道失败",
			Result: err,
		}
		failed, _ := json.Marshal(result)
		err = r.ManagerPushMsg(failed, currentExecMsg)
		if err != nil {
			zap.S().Debug("推送消息失败:", err)
			return
		}
		return
	} else {
		result = declare.Result{
			Code: declare.StatusOk,
			Desc: "创建通道成功",
		}
		tmp, _ := json.Marshal(result)
		err = r.ManagerPushMsg(tmp, currentExecMsg)
		if err != nil {
			zap.S().Debug("推送消息失败:", err)
			return
		}
	}
	r.RpcTask[newTask.TaskName] = newTask
	switch currentMsg.Params.RequestResource {
	case "k8sssh":
		newTask.InitExecK8sCommand(&currentMsg)
	case "podLog":
		newTask.InitK8sLog()
	case "updateDocker":
		newTask.InitUpdateDocker(&currentMsg)
	case "dockerssh":
		newTask.InitExecDockerCommand(&currentMsg)
	case "dockerLog":
		newTask.InitDockerLog(&currentMsg)
	default:
		go newTask.InitGeneralTask(&currentMsg)
		//go newTask.WriteLoop()
	}
	zap.S().Debug("【流程】等待Task结束", newTask.TaskName)
	for {
		select {
		case <-newTask.ManagerClose:
			zap.S().Debug("【关闭通知】newTask.ManagerClose接收到关闭请求", newTask.TaskName)
			switch currentMsg.Params.RequestResource {
			case "ssh":
				newTask.TaskClose()
			case "dockerssh":
				newTask.StreamHandler.WriteClose <- newTask.TaskName
				newTask.StreamHandler.ReadClose <- newTask.TaskName
				zap.S().Debug("【关闭通知】dockerssh接收到关闭请求", newTask.TaskName)
				newTask.TaskClose()
			default:

				newTask.TaskClose()
				//
				//newTask.ReadClose <- newTask.TaskName
				//newTask.WriteClose <- newTask.TaskName
			}
			goto END
		default:
			time.Sleep(time.Millisecond * 200)
		}
	}
END:
}
func (r *RpcTaskManager) ManagerPushMsg(pushMSg []byte, currentMsg *amqp.Delivery) (err error) {
	err = ManagerChannel.Publish(
		"",                 // exchange
		currentMsg.ReplyTo, // routing key
		false,              // mandatory
		false,              // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			CorrelationId: currentMsg.CorrelationId,
			Body:          pushMSg,
		})
	if err != nil {
		zap.S().Debug("推送消息失败,", err)
		return
	}

	zap.S().Debug("ManagerPushMsg()Rabbitmq响应的通知信息到", currentMsg.ReplyTo, "发送的信息为：", string(pushMSg))
	//currentMsg.Ack(false)
	return
}
func (r *RpcTaskManager) ManagerCloseTask(name string) {

	_, ok := r.RpcTask[name]
	if ok {
		//如果是普通的RPC调用
		switch r.RpcTask[name].ClientParams.Params.RequestResource {
		case "ssh":
			zap.S().Debug("【关闭通知】Close", name)
			r.RpcTask[name].Close <- name
		case "dockerssh":
			zap.S().Debug("【关闭通知】Close", name)
			r.RpcTask[name].ManagerClose <- name
		default:
			zap.S().Debug("【关闭通知】向任务的ManagerClose通道发送关闭任务3", name)
			r.RpcTask[name].ManagerClose <- name
		}
		time.Sleep(time.Second * 2)
		delete(r.RpcTask, name)
	} else {
		zap.S().Debug("接收到的关闭任务不存在：", name)
	}
}
func TestActionAllow(action string) bool {
	var flag bool
	if agentConfig.DeploymentType == "docker" {
		switch action {
		case "updateDocker":
			flag = true
		case "dockerssh":
			flag = true
		case "dockerLog":
			flag = true
		case "GetDockerInfo":
			flag = true
		case "RestartDocker":
			flag = true
		case "DeleteDocker":
			flag = true
		}
	} else if agentConfig.DeploymentType == "k8s" {
		switch action {
		case "k8sssh":
			flag = true
		case "podLog":
			flag = true
		case "GetK8sDeployment":
			flag = true
		case "GetK8sPod":
			flag = true
		case "DeleteK8sPod":
			flag = true
		case "DeploymentK8sUpdate":
			flag = true
		case "DeployUpdate":
			flag = true
		case "GetPodByDeployment":
			flag = true
		case "DeploymentScale":
			flag = true
		}
	}
	return flag
}
