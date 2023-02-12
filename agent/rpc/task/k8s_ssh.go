package task

import (
	"encoding/json"
	"github.com/docker/docker/api/types"
	"github.com/gorilla/websocket"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"log"
	"net/http"
	"public/declare"
)

// http升级websocket协议的配置
var wsUpgrader = websocket.Upgrader{
	// 允许所有CORS跨域请求
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// websocket消息
type WsMessage struct {
	MessageType int          `json:"messageType"`
	Data        xtermMessage `json:"data"`
}

// web终端发来的包
type xtermMessage struct {
	MsgType string `json:"type"`  // 类型:resize客户端调整终端, input客户端输入
	Input   string `json:"input"` // msgtype=input情况下使用
	Rows    uint16 `json:"rows"`  // msgtype=resize情况下使用
	Cols    uint16 `json:"cols"`  // msgtype=resize情况下使用
}

// ssh流式处理器
type StreamHandler struct {
	ResizeEvent       chan remotecommand.TerminalSize
	Channel           *amqp.Channel
	ConsumerQueueChan <-chan amqp.Delivery
	LoopClose         chan string
	ReadClose         chan string
	WriteClose        chan string
	Exchange          string
	BackQueueName     string
	ReceiveQueueName  string
	RoutingKey        string
	CorrId            string
	//docker
	Console types.HijackedResponse
	ExecID  string
}

var stc = "\\u0003"
var std = "\\u0004"
var stt = "\\r"

// executor回调获取web是否resize
func (handler *StreamHandler) Next() (size *remotecommand.TerminalSize) {
	ret := <-handler.ResizeEvent
	size = &ret
	zap.S().Debug("【WS任务】检测到改变窗口大小，改变为：", size.Height, size.Width)
	return
}

// executor回调读取web端的输入
func (handler *StreamHandler) Read(p []byte) (size int, err error) {

	tmpS := WsMessage{}
	select {
	case d := <-handler.ConsumerQueueChan:
		zap.S().Debug("【WS任务】Agent从Rabbitmq接收到的消息为", string(d.Body))
		err = json.Unmarshal(d.Body, &tmpS)
		if err != nil {
			zap.S().Debug("consumerQueue解析失败,", err)
			return
		}
		err = d.Ack(false)
		if err != nil {
			zap.S().Debug("Agent从Rabbitmq中获取到消息后ACK失败：", err)
			return
		}
		//web ssh调整了终端大小
		if tmpS.Data.MsgType == "resize" {
			// 放到channel里，等remotecommand executor调用我们的Next取走
			handler.ResizeEvent <- remotecommand.TerminalSize{Width: tmpS.Data.Cols, Height: tmpS.Data.Rows}
		} else if tmpS.Data.MsgType == "input" { // web ssh终端输入了字符
			// copy到p数组中
			size = len(tmpS.Data.Input)
			copy(p, tmpS.Data.Input)
		}
	case x := <-handler.LoopClose:
		log.Printf("handler.LoopClose-->%s\n", x)
		size = len(tmpS.Data.Input)
		copy(p, x)
	}

	return
}

// executor回调向web端输出
func (handler *StreamHandler) Write(p []byte) (size int, err error) {
	var (
		copyData []byte
	)

	// 产生副本
	copyData = make([]byte, len(p))
	copy(copyData, p)
	size = len(p)

	err = handler.Channel.Publish(
		"",                    // exchange
		handler.BackQueueName, // routing key
		false,                 // mandatory
		false,                 // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: handler.CorrId,
			Body:          copyData,
		})
	zap.S().Debug("【WS任务】Agent回复队列响应队列发送的消息，发送的内容为：", handler.BackQueueName, string(copyData))
	if err != nil {
		zap.S().Debug("回复消息失败：,", err)
		return
	}
	return
}
func (t *RpcTask) InitExecK8sCommand(currentMsg *declare.RabbitmqMsg) {
	var (
		sshReq   *rest.Request          //
		executor remotecommand.Executor //k8s与服务端的连接
		err      error
	)
	zap.S().Debug("定义交换机和消费者队列", t.StreamHandler.Exchange, t.StreamHandler.ReceiveQueueName, t.StreamHandler.BackQueueName)
	zap.S().Debug("<--------------------------准备打开命令流1:", err)
	// URL长相:
	// https://172.18.11.25:6443/api/v1/namespaces/default/pods/nginx-deployment-5cbd8757f-d5qvx/exec?command=sh&container=nginx&stderr=true&stdin=true&stdout=true&tty=true
	sshReq = clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(currentMsg.Params.PodName).
		Namespace(currentMsg.Params.Namespace).
		SubResource("exec").
		VersionedParams(&v1.PodExecOptions{
			Container: currentMsg.Params.ContainerName,
			Command:   []string{"bash"},
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
		}, scheme.ParameterCodec)
	zap.S().Debug("<--------------------------准备打开命令流2:", err)

	// 创建到容器的连接
	if executor, err = remotecommand.NewSPDYExecutor(restConf, "POST", sshReq.URL()); err != nil {
		zap.S().Debug("NewSPDYExecutor:", err)
	}
	zap.S().Debug("<--------------------------准备打开命令流3:", err)
	go func() {
		select {
		case taskName := <-t.Close:
			zap.S().Debug("【关闭任务通道】任务关闭协程t.Close 接收到Loop关闭通知成功: ", taskName)
			//当
			t.StreamHandler.LoopClose <- stc
			t.StreamHandler.LoopClose <- stc
			t.StreamHandler.LoopClose <- stt
			t.StreamHandler.LoopClose <- std
			t.StreamHandler.LoopClose <- std
			zap.S().Debug("【关闭任务通道】Loop成功: ", taskName)
		}
	}()
	go func() {
		err = executor.Stream(remotecommand.StreamOptions{
			Stdin:             t.StreamHandler,
			Stdout:            t.StreamHandler,
			Stderr:            t.StreamHandler,
			TerminalSizeQueue: t.StreamHandler,
			Tty:               true,
		})
		if err != nil {
			zap.S().Debug("【执行命令结束】Stream:", err)
		}
		zap.S().Debug("【执行命令结束】向t.ManagerClose发送消息")
		t.ManagerClose <- t.TaskName
	}()

	zap.S().Debug("<--------------------------打开命令流成功:", err)
	return
	//
	//fmt.Println(err)
	//t.ManagerClose <- t.TaskName

}
