package task

import (
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	"net/http"
	"public/declare"
	"sync"
	"time"
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

// 封装websocket连接
type WsConnection struct {
	wsSocket             *websocket.Conn // 底层websocket
	inChan               chan *WsMessage // 读取队列
	outChan              chan *WsMessage // 发送队列
	mutex                sync.Mutex      // 避免重复关闭管道
	isClosed             bool
	TaskName             string
	CloseWsReadLoopChan  chan string          // 关闭通知
	WsReadLoopOkChan     chan string          // 关闭通知
	WsReadLoopFalseChan  chan string          // 关闭通知
	CloseWsWriteLoopChan chan string          // 关闭通知
	CloseChan            chan string          // 关闭通知
	Ch                   *amqp.Channel        //创建一个websocket对象时，同时创建一个rabbitmq通道
	receiveMsg           <-chan amqp.Delivery //一个channel，在发送消息后，还需要监听一个队列回复回来的消息
	receiveQueueName     string               //监听接收数据的queue名
	corrId               string               //客户端回复的ID
	RoutingKey           string
}

// 读取协程
func (wsConn *WsConnection) wsReadLoop() {
	var (
		msgType int
		data    []byte
		msg     *WsMessage
		err     error
	)
	for {
		// 读一个message
		msgType, data, err = wsConn.wsSocket.ReadMessage()
		if err != nil {
			zap.S().Info("【WS任务】读取消息失败：", err.Error())
			wsConn.WsReadLoopFalseChan <- err.Error()
		} else {
			wsConn.WsReadLoopOkChan <- wsConn.TaskName
		}
		select {
		case m := <-wsConn.WsReadLoopFalseChan:
			zap.S().Info("【WS任务】接收到wsConn.WsReadLoopFalseChan通道消息，WS读取消息失败：", m)
			wsConn.CloseChan <- wsConn.TaskName
		case <-wsConn.CloseWsReadLoopChan:
			zap.S().Debug("【WS任务】", wsConn.TaskName, "任务接收到关闭WS读协程")
			wsConn.WsClose()
			goto END
		case <-wsConn.WsReadLoopOkChan:
			tmp := []byte{}
			tmpData := xtermMessage{}
			json.Unmarshal(data, &tmpData)
			msg = &WsMessage{
				msgType,
				tmpData,
			}
			// 放入请求队列
			tmp, err = json.Marshal(msg)
			if err != nil {
				zap.S().Debug("解析失败,", err)
				return
			}
			err = wsConn.Ch.Publish(
				agentConfig.Rabbitmq.TaskExchange, // exchange
				wsConn.RoutingKey,  // routing key
				false,              // mandatory
				false,              // immediate
				amqp.Publishing{
					ContentType: "text/plain",
					//ContentType:   "application/json",
					CorrelationId: wsConn.corrId,
					ReplyTo:       wsConn.receiveQueueName, //定义将rpc调用发出去后，由那个队列名接收响应
					Body:          tmp,
				})
			if err != nil {
				zap.S().Error("向exec_command_queue队列投放消息失败：", err.Error())
				return
			}
			zap.S().Debug("【WS任务】Client------>Server读取用户输入，将消息发exec_command_queue队列信息:", string(tmp))
		}
	}

END:
	zap.S().Info("【WS任务】任务名", wsConn.TaskName, "WS读协程关闭成功")

}

// 发送协程
func (wsConn *WsConnection) wsWriteLoop() {
	var (
		err error
	)
	for {
		select {
		case tmp := <-wsConn.receiveMsg:
			if tmp.CorrelationId == wsConn.corrId {
				zap.S().Debug("【WS任务】Server----->Client:响应队列接收到的信息为，直接发送到客户端", string(tmp.Body))
				err = wsConn.wsSocket.WriteMessage(websocket.TextMessage, tmp.Body)
				if err != nil {
					zap.S().Debug("向浏览器发送消息失败：", err)
					wsConn.CloseChan <- wsConn.TaskName
					break
				}
				err = tmp.Ack(false)
				if err != nil {
					zap.S().Debug("接收到消息ACK失败：", err)
				}
			}
		case <-wsConn.CloseWsWriteLoopChan:
			zap.S().Debug("【WS任务】接收到关闭WS写协程：", wsConn.TaskName)
			wsConn.WsClose()
			goto END
		}
	}
END:
	zap.S().Info("【WS任务】任务名", wsConn.TaskName, "WS写协程关闭成功")
}

/************** 并发安全 API **************/

func StartWebsocket(resp http.ResponseWriter, req *http.Request, clientRequestparams *declare.ClientParams) (err error) {
	var wsConn = new(WsConnection)
	taskName := RandomString(32)
	wsSocket := new(websocket.Conn)
	if wsSocket, err = wsUpgrader.Upgrade(resp, req, nil); err != nil {
		zap.S().Debug("Upgrade", err)
		return
	}
	wsConn = &WsConnection{
		wsSocket:             wsSocket,
		TaskName:             taskName,
		inChan:               make(chan *WsMessage, 5000),
		outChan:              make(chan *WsMessage, 5000),
		WsReadLoopFalseChan:  make(chan string, 1),
		WsReadLoopOkChan:     make(chan string, 1),
		CloseWsReadLoopChan:  make(chan string, 1),
		CloseWsWriteLoopChan: make(chan string, 1),
		CloseChan:            make(chan string, 2),
		isClosed:             false,
	}

	var send = new(declare.RabbitmqMsg)

	send = &declare.RabbitmqMsg{
		Loop:       true,
		Exchange:   agentConfig.Rabbitmq.Exchange,
		RoutingKey: clientRequestparams.ActionRoutingKey,
		TaskName:   taskName,
		TaskAction: true,
		Action:     clientRequestparams.RequestResource,
		Params:     *clientRequestparams,
		ReceiveMsg: declare.ReceiveMsg{
			Exchange:         agentConfig.Rabbitmq.TaskExchange,
			ReceiveQueueName: taskName,
			RoutingKey:       taskName,
			CorrId:           taskName,
		},
	}
	msg := InitTask(send)

	_, err = msg.SendCreateTask(wsConn)
	if err != nil {
		zap.S().Debug(err)
		return
	}
	zap.S().Info("初始化WEB SOCKET完成,等待接收到关闭信号")
	for {
		select {
		case receive := <-wsConn.CloseChan:
			//receive := <-wsConn.CloseChan
			zap.S().Info("【WS任务】关闭的任务名为：", receive)
			err = msg.CloseTask()
			if err != nil {
				zap.S().Debug("【WS任务】关闭任务失败", err)
			}
			goto END
		default:
			time.Sleep(time.Millisecond * 100)
		}
	}
END:
	zap.S().Info("【WS任务】关闭任务成功", send.TaskName)
	return
}

// 关闭连接
func (wsConn *WsConnection) WsClose() {
	wsConn.mutex.Lock()
	defer wsConn.mutex.Unlock()
	if !wsConn.isClosed {
		wsConn.isClosed = true
		err := wsConn.wsSocket.Close()
		if err != nil {
			//zap.S().Debug("关闭ws通道：", err)
			zap.S().Info("【WS任务】wsSocket关闭失败", wsConn.TaskName, err.Error())
			return
		}
		zap.S().Info("【WS任务】wsSocket关闭成功", wsConn.TaskName)
	} else {
		zap.S().Info("【WS任务】ws通道已经被关闭无需重复关闭：", wsConn.TaskName)
	}
}
