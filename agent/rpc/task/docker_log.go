package task

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	"io"
)

func (log *GeneralTaskHandler) DockerLogReadLoop() {

	containerLog, err := Dockerset.ContainerLogs(context.TODO(), log.Container, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "1000",
	})
	if err != nil {
		zap.S().Debug(err)
		return
	}
	for {
		select {
		case <-log.ReadClose:
			//zap.S().Debug("【关闭任务】ReadLoop  log.ReadClose 接收到关闭日志信号")
			goto END
		default:
			buf := make([]byte, 50000)
			numBytes, err2 := containerLog.Read(buf)
			//numBytes, err2 := stream.Read(buf)
			if numBytes == 0 {
				continue
			}
			if err2 == io.EOF {
				break
			}
			if err2 != nil {
				return
			}
			err = log.Channel.Publish(
				"",                // exchange
				log.BackQueueName, // routing key
				false,             // mandatory
				false,             // immediate
				amqp.Publishing{
					ContentType:   "application/json",
					CorrelationId: log.CorrId,
					Body:          buf[:numBytes],
				})
			if err != nil {
				return
			}
		}
	}
END:

	zap.S().Debug("【关闭任务】关闭Docker日志任务读协程完成")
	return
}
