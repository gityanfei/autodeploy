package task

import (
	"context"
	"encoding/json"
	"github.com/docker/docker/api/types"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	"public/declare"

	"log"
)

func (t *RpcTask) InitDockerLog(currentMsg *declare.RabbitmqMsg) {

	zap.S().Debug("定义交换机和消费者队列", t.GeneralTaskHandler.Exchange, t.GeneralTaskHandler.ReceiveQueueName, t.GeneralTaskHandler.BackQueueName)
	//zap.S().Debug("<--------------------------准备打开命令流1:", err)

	go t.GeneralTaskHandler.DockerLogReadLoop()
	go t.GeneralTaskHandler.WriteLoop()
	//go t.GeneralTaskHandler.DockerLogWriteLoop()

}
func (t *RpcTask) InitGeneralTask(currentMsg *declare.RabbitmqMsg) {
	zap.S().Debug("定义交换机和消费者队列", t.GeneralTaskHandler.Exchange, t.GeneralTaskHandler.ReceiveQueueName, t.GeneralTaskHandler.BackQueueName)
	go t.GeneralTaskHandler.GeneralReadLoop()
	go t.GeneralTaskHandler.WriteLoop()
}
func (t *RpcTask) InitExecDockerCommand(currentMsg *declare.RabbitmqMsg) {

	zap.S().Debug("定义交换机和消费者队列", t.StreamHandler.Exchange, t.StreamHandler.ReceiveQueueName, t.StreamHandler.BackQueueName)

	// 执行exec，获取到容器终端的连接
	zap.S().Debug("执行命令的容器名为：", currentMsg.Params.ContainerName)

	hr, execId, err := execDocker(currentMsg.Params.ContainerName)
	if err != nil {
		zap.S().Error(err)
		return
	}
	t.StreamHandler.Console = hr
	t.StreamHandler.ExecID = execId
	go t.StreamHandler.DockerReadLoop()
	go t.StreamHandler.DockerWriteLoop()

}
func execDocker(container string) (hr types.HijackedResponse, execID string, err error) {

	// 执行/bin/bash命令
	ir, err := Dockerset.ContainerExecCreate(ctx, container, types.ExecConfig{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"/bin/bash"},
		Tty:          true,
	})
	if err != nil {
		log.Println("命令创建失败：", err)
		return
	}

	// 附加到上面创建的/bin/bash进程中
	hr, err = Dockerset.ContainerExecAttach(ctx, ir.ID, types.ExecStartCheck{Detach: false, Tty: true})
	if err != nil {
		return
	}
	execID = ir.ID
	return
}

func (handler *StreamHandler) DockerWriteLoop() {
	for {
		select {
		case <-handler.WriteClose:
			zap.S().Debug("handler.DockerWriteLoop-->%s\n")
			goto END
		default:
			buf := make([]byte, 50000)
			nr, err := handler.Console.Conn.Read(buf)
			if nr > 0 {
				log.Printf("写入到客户端的内容为为：%s\n", buf[0:nr])
				err = handler.Channel.Publish(
					"",                    // exchange
					handler.BackQueueName, // routing key
					false,                 // mandatory
					false,                 // immediate
					amqp.Publishing{
						ContentType:   "text/plain",
						CorrelationId: handler.CorrId,
						Body:          buf[0:nr],
					})
				if err != nil {
					zap.S().Debug("回复消息失败：,", err)
				}
				zap.S().Debug("【WS任务】Agent回复队列响应队列发送的消息，发送的内容为：", handler.BackQueueName, string(buf[0:nr]))
			}
		}
	}
END:
	zap.S().Debug("【关闭任务】DockerWriteLoop关闭成功")
}

func (handler *StreamHandler) DockerResize(size types.ResizeOptions) {
	log.Println("ExecID:", handler.ExecID)
	err := Dockerset.ContainerExecResize(context.TODO(), handler.ExecID, size)
	if err != nil {
		log.Println("检测到窗口变化，进行终端Resize:", err)
	}
}
func (handler *StreamHandler) DockerReadLoop() {
	var err error

	for {
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
				handler.DockerResize(types.ResizeOptions{
					Height: uint(tmpS.Data.Rows),
					Width:  uint(tmpS.Data.Cols),
				})
			} else if tmpS.Data.MsgType == "input" { // web ssh终端输入了字符
				// copy到p数组中
				_, err = handler.Console.Conn.Write([]byte(tmpS.Data.Input))
				if err != nil {
					zap.S().Error("将命令写入Docker失败,", err)
					return
				}
			}
		case _ = <-handler.ReadClose:
			goto END
		}
	}

END:
	zap.S().Debug("【关闭任务】DockerReadLoop关闭成功", handler.CorrId)
}
