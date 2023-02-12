package task

import (
	"github.com/streadway/amqp"
	"go.uber.org/zap"
)

func (log *GeneralTaskHandler) WriteLoop() {

	for {
		select {
		case <-log.WriteClose:
			goto END
		case copyData := <-log.LogMessageChan:
			err := log.Channel.Publish(
				"",                // exchange
				log.BackQueueName, // routing key
				false,             // mandatory
				false,             // immediate
				amqp.Publishing{
					ContentType:   "text/plain",
					CorrelationId: log.CorrId,
					Body:          copyData,
				})
			zap.S().Debug("【WS任务】Agent回复%s队列响应队列发送的消息，发送的内容为：", log.BackQueueName, string(copyData))
			if err != nil {
				zap.S().Debug("回复消息失败：,", err)
				return
			}
		}
	}
END:
	zap.S().Debug("【关闭任务】关闭General任务写协程成功")
	return
}
