package task

import (
	//"bytes"
	"context"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	"io"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

var GlobalK8sConfig *kubernetes.Clientset

type GeneralTaskHandler struct {
	Namespace         string
	PodName           string
	Container         string
	Channel           *amqp.Channel
	LogMessageChan    chan []byte
	ConsumerQueueChan <-chan amqp.Delivery
	LoopClose         chan string
	ReadClose         chan string
	WriteClose        chan string
	Exchange          string
	BackQueueName     string
	ReceiveQueueName  string
	RoutingKey        string
	CorrId            string
}

func (t *RpcTask) InitK8sLog() {

	// URL长相:
	go t.GeneralTaskHandler.K8sLogReadLoop()
	go t.GeneralTaskHandler.WriteLoop()

}

func (log *GeneralTaskHandler) K8sLogReadLoop() {
	count := int64(1000)
	podLogOptions := v1.PodLogOptions{
		Container: log.Container,
		Follow:    true,
		TailLines: &count,
	}
	podLogRequest := GlobalK8sConfig.CoreV1().Pods(log.Namespace).GetLogs(log.PodName, &podLogOptions)
	stream, err := podLogRequest.Stream(context.TODO())
	if err != nil {
		return
	}

	for {
		select {
		case <-log.ReadClose:
			//zap.S().Debug("【关闭任务】ReadLoop  log.ReadClose 接收到关闭日志信号")
			goto END
		default:
			buf := make([]byte, 50000)
			numBytes, err2 := stream.Read(buf)
			if numBytes == 0 {
				continue
			}
			if err2 == io.EOF {
				log.LogMessageChan <- []byte("接收日志结束")
				break
			}
			if err2 != nil {
				log.LogMessageChan <- []byte(err2.Error())
				return
			}
			log.LogMessageChan <- buf[:numBytes]
		}
	}
END:
	err = stream.Close()
	if err != nil {
		zap.S().Debug("【关闭任务】关闭日志流失败tream.Close()")
	}
	zap.S().Debug("【关闭任务】关闭K8s日志任务读协程完成")
	return
}
