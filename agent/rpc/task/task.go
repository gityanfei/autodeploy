package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/remotecommand"
	"log"
	"public/declare"
	"sync"
	"time"
)

type RpcTask struct {
	TaskName string
	Channel  *amqp.Channel
	//ConsumerQueueChan <-chan amqp.Delivery
	//Exchange string
	//BackQueueName     string
	//RoutingKey        string
	//ReceiveQueueName  string
	//CorrId            string
	Loop               bool
	StreamHandler      *StreamHandler
	GeneralTaskHandler *GeneralTaskHandler
	ManagerClose       chan string
	Close              chan string
	ReadClose          chan string
	WriteClose         chan string
	ClientParams       declare.RabbitmqMsg
	WriteRusult        chan *[]byte
	mutex              sync.Mutex // 避免重复关闭管道
	isClosed           bool
}
type Client declare.ClientParams

// 关闭连接
func (t *RpcTask) TaskClose() {

	t.mutex.Lock()
	defer t.mutex.Unlock()
	if t.isClosed { // 如果已经关闭了就不要重复关闭，否则要painc
		zap.S().Debug("【关闭任务通道】这个任务的通道已经关闭无需重复关闭: ", t.TaskName)
	} else { // 如果没有关闭就进入正常流程
		t.isClosed = true

		switch t.ClientParams.Params.RequestResource {
		case "k8sssh":
			zap.S().Debug("【关闭任务通道】Loop任务的通道已经关闭", t.TaskName)
			err := t.Channel.Close()
			if err != nil {
				zap.S().Debug("【关闭任务通道】", err)
			}
		case "podLog":
			t.GeneralTaskHandler.WriteClose <- t.TaskName
			t.GeneralTaskHandler.ReadClose <- t.TaskName
			time.Sleep(time.Second * 2)
			err := t.Channel.Close()
			if err != nil {
				zap.S().Debug("关闭通道失败", err)
			}
			close(t.GeneralTaskHandler.WriteClose)
			close(t.GeneralTaskHandler.ReadClose)
			zap.S().Debug("【关闭任务通道】podLog成功: ", t.TaskName)
		default:
			t.GeneralTaskHandler.WriteClose <- t.TaskName
			t.GeneralTaskHandler.ReadClose <- t.TaskName
			//关闭任务通道
			err := t.Channel.Close()
			if err != nil {
				zap.S().Debug("关闭通道失败", err)
			}
			zap.S().Debug("【关闭任务通道】成功: ", t.TaskName)
			close(t.WriteClose)
			close(t.ReadClose)
			close(t.StreamHandler.ReadClose)
			close(t.StreamHandler.WriteClose)
		}
	}
}
func CreateTask(R declare.RabbitmqMsg) (task *RpcTask, err error) {
	var taskChannel *amqp.Channel
	taskChannel, err = conn.Channel()
	if err != nil {
		zap.S().Debug("初始化channel失败:", err)
		return
	}
	zap.S().Debug("开启协程定义--->交换机和消费者队列", R.ReceiveMsg.Exchange, R.ReceiveMsg.ReceiveQueueName, R.ReceiveMsg.BackQueueName)
	err = taskChannel.ExchangeDeclare(
		R.ReceiveMsg.Exchange, // name
		"direct",              // type
		true,                  // durable
		false,                 // auto-deleted
		false,                 // internal
		false,                 // no-wait
		nil,                   // arguments
	)
	if err != nil {
		zap.S().Debug("任务交换机定义失败：", err)
		return
	}

	q, err := taskChannel.QueueDeclare(
		R.ReceiveMsg.ReceiveQueueName, // name
		false,                         // durable
		true,                          // delete when unused
		false,                         // exclusive
		false,                         // no-wait
		nil,                           // arguments
	)
	if err != nil {
		zap.S().Debug("任务队列定义失败：", err)
		return
	}
	err = taskChannel.QueueBind(
		R.ReceiveMsg.ReceiveQueueName, // queue name
		R.ReceiveMsg.RoutingKey,       // routing key
		R.ReceiveMsg.Exchange,         // exchange
		false,
		nil)
	if err != nil {
		zap.S().Debug("任务交换机和队列绑定失败：", err)
		return
	}
	err = taskChannel.Qos(
		1000,  // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		zap.S().Debug("任务QOS定义失败：", err)
		return
	}
	consumer, err := taskChannel.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		zap.S().Debug("监听消费者失败,", err)
		return
	}
	// 配置与容器之间的数据流处理回调
	task = &RpcTask{
		TaskName: R.TaskName,
		Channel:  taskChannel,
		//ConsumerQueueChan: consumer,
		//Exchange:          R.ReceiveMsg.Exchange,
		//BackQueueName:     R.ReceiveMsg.BackQueueName,
		//RoutingKey:        R.ReceiveMsg.RoutingKey,
		//ReceiveQueueName:  R.ReceiveMsg.ReceiveQueueName,
		//CorrId:            R.ReceiveMsg.CorrId,
		Loop: R.Loop,
		StreamHandler: &StreamHandler{
			ResizeEvent:       make(chan remotecommand.TerminalSize),
			LoopClose:         make(chan string, 3),
			ReadClose:         make(chan string, 2),
			WriteClose:        make(chan string, 2),
			Channel:           taskChannel,
			ConsumerQueueChan: consumer,
			Exchange:          R.ReceiveMsg.Exchange,
			BackQueueName:     R.ReceiveMsg.BackQueueName,
			ReceiveQueueName:  R.ReceiveMsg.ReceiveQueueName,
			RoutingKey:        R.ReceiveMsg.RoutingKey,
			CorrId:            R.ReceiveMsg.CorrId,
		},
		GeneralTaskHandler: &GeneralTaskHandler{
			Namespace:         R.Params.Namespace,
			PodName:           R.Params.PodName,
			Container:         R.Params.ContainerName,
			Channel:           taskChannel,
			LogMessageChan:    make(chan []byte, 1000),
			ConsumerQueueChan: consumer,
			LoopClose:         make(chan string, 2),
			ReadClose:         make(chan string, 2),
			WriteClose:        make(chan string, 2),
			Exchange:          R.ReceiveMsg.Exchange,
			BackQueueName:     R.ReceiveMsg.BackQueueName,
			ReceiveQueueName:  R.ReceiveMsg.ReceiveQueueName,
			RoutingKey:        R.ReceiveMsg.RoutingKey,
			CorrId:            R.ReceiveMsg.CorrId,
		},
		ManagerClose: make(chan string, 2),
		Close:        make(chan string, 2),
		ReadClose:    make(chan string, 1),
		WriteClose:   make(chan string, 1),
		ClientParams: R,
		WriteRusult:  make(chan *[]byte, 1000),
		mutex:        sync.Mutex{},
		isClosed:     false,
	}
	return

}
func (log *GeneralTaskHandler) GeneralReadLoop() {
	for {
		select {
		case readMsg := <-log.ConsumerQueueChan:
			zap.S().Debug("【普通任务】ReadLoop接收到的信息为：%s", string(readMsg.Body))
			RabbitmqReceive := declare.RabbitmqMsg{}
			msg := Client{}
			err := json.Unmarshal(readMsg.Body, &RabbitmqReceive)
			tmpMsg, _ := json.Marshal(RabbitmqReceive.Params)
			err = json.Unmarshal(tmpMsg, &msg)
			//log.Println(msg)
			if err != nil {
				zap.S().Error("json序列化失败,%#s", err)
				return
			}
			var result = []byte{}
			switch RabbitmqReceive.Action {
			case "GetK8sDeployment":
				result, err = msg.GetK8sDeployment()
			case "GetK8sPod":
				result, err = msg.GetK8sPod()
			case "DeleteK8sPod":
				result, err = msg.DeleteK8sPod()
			case "DeploymentK8sUpdate":
				result, err = msg.DeploymentK8sUpdate()
			case "DeployUpdate":
				result, err = msg.DeploymentK8sUpdate()
			case "GetDockerInfo":
				result, err = msg.GetDockerInfo()
			case "RestartDocker":
				result, err = msg.RestartDocker()
			case "DeleteDocker":
				result, err = msg.DeleteDocker()
			case "GetPodByDeployment":
				result, err = msg.GetPodByDeployment()
			case "DeploymentScale":
				result, err = msg.DeploymentScale()
			//case "CreateTask":
			//	result, err = msg.InitExecCommand()
			default:
				err = errors.New("操作动作不允许")
			}
			log.LogMessageChan <- result
			err = readMsg.Ack(false)
			if err != nil {
				zap.S().Debug("任务ACK确认失败,", err)
				return
			}
		case <-log.ReadClose:
			//log.TaskClose()
			goto END
		}
	}
END:
	zap.S().Debug("【关闭任务】关闭General任务读协程完成")
}

func (client *Client) DeleteDocker() (result []byte, err error) {
	err = DeleteDocker(client.ContainerName)
	if err != nil {
		zap.S().Debug(err)
		return result, err
	}
	tmp := declare.Result{
		Code: "000000",
		Desc: "成功",
	}
	result, err = json.Marshal(tmp)
	if err != nil {
		return result, err
	}
	return
}
func (client *Client) RestartDocker() (result []byte, err error) {
	err = RestartDocker(client.ContainerName)
	if err != nil {
		return result, err
	}
	tmp := declare.Result{
		Code: "000000",
		Desc: "成功",
	}
	result, err = json.Marshal(tmp)
	if err != nil {
		return result, err
	}
	return
}
func (client *Client) GetDockerInfo() (result []byte, err error) {
	tmp, err := GetDockerInfo()
	if err != nil {
		return result, err
	}
	result, err = json.Marshal(tmp)
	if err != nil {
		return result, err
	}
	return
}
func (client *Client) GetK8sDeployment() (result []byte, err error) {
	zap.S().Info(client.Namespace)
	tmp, err := DeploymentStatus(client.Namespace)
	if err != nil {
		return result, err
	}
	result, err = json.Marshal(tmp)
	if err != nil {
		return result, err
	}
	return
}
func (client *Client) GetK8sPod() (result []byte, err error) {
	tmp := PodStatus(client.Namespace)
	if err != nil {
		return result, err
	}
	result, err = json.Marshal(tmp)
	if err != nil {
		return result, err
	}
	return
}
func (client *Client) DeleteK8sPod() (result []byte, err error) {
	var tmp = declare.Result{}
	err = DeletePod(client.Namespace, client.PodName)
	if err != nil {
		tmp = declare.Result{
			Code:   "999999",
			Desc:   "失败",
			Result: err,
		}
		return result, err
	}
	tmp = declare.Result{
		Code: "000000",
		Desc: "成功",
	}
	result, err = json.Marshal(tmp)
	if err != nil {
		return result, err
	}
	return
}
func (client *Client) DeploymentK8sUpdate() (result []byte, err error) {
	var tmp = declare.Result{}
	fullDestImageAddr := fmt.Sprintf("%s/%s/%s", agentConfig.K8sConfig.DestImageInfo.Domain, agentConfig.K8sConfig.DestImageInfo.DockerRepo, client.ServiceTag)
	log.Printf("%s/%s/%s", agentConfig.K8sConfig.DestImageInfo.Domain, agentConfig.K8sConfig.DestImageInfo.DockerRepo, client.ServiceTag)

	zap.S().Debug("DEploymentK8sUpdate ", fullDestImageAddr)
	err = DeployUpdate(client.Namespace, client.DeploymentName, fullDestImageAddr)
	if err != nil {
		tmp = declare.Result{
			Code:   "999999",
			Desc:   "失败",
			Result: err,
		}
	} else {
		tmp = declare.Result{
			Code: "000000",
			Desc: "成功",
		}
	}
	result, err = json.Marshal(tmp)
	return
}
func (client *Client) GetPodByDeployment() (result []byte, err error) {
	tmp, err := GetPodByDeployment(client.Namespace, client.DeploymentName)
	if err != nil {
		return result, err
	}
	result, err = json.Marshal(tmp)
	if err != nil {
		return result, err
	}
	return
}
func (client *Client) DeploymentScale() (result []byte, err error) {
	var tmp = declare.Result{}
	err = DeployScale(client.Namespace, client.DeploymentName, client.DeploymentReplicas)
	if err != nil {
		tmp = declare.Result{
			Code:   "999999",
			Desc:   "失败",
			Result: err,
		}
	} else {
		tmp = declare.Result{
			Code: "000000",
			Desc: "成功",
		}
	}
	result, err = json.Marshal(tmp)
	return
}
