package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	"public/declare"
)

func (m Msg) ContinuedRemoteRPC(ctx context.Context) (result chan *declare.LogChain, err error) {
	ch, err := conn.Channel()
	if err != nil {
		zap.S().Debug("初始化channel失败")
		return
	}
	ch, err = conn.Channel()
	if err != nil {
		return
	}
	err = ch.ExchangeDeclare(
		m.Exchange, // name
		"direct",   // type
		true,       // durable
		false,      // auto-deleted
		false,      // internal
		false,      // no-wait
		nil,        // arguments
	)
	if err != nil {
		return
	}
	//这里是定义接收处理结果的随机名queue
	q, err := ch.QueueDeclare(
		"",    // name
		false, // durable
		true,  // delete when unused
		true,  // exclusive
		false, // noWait
		nil,   // arguments
	)
	if err != nil {
		return
	}
	//持续监听这个随机名queue中的数据
	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return
	}
	corrId := RandomString(32)
	//fmt.Println(corrId)
	body, err := json.Marshal(m)
	if err != nil {
		return
	}
	err = ch.Publish(
		m.Exchange,   // exchange
		m.RoutingKey, // routing key
		false,        // mandatory
		false,        // immediate
		amqp.Publishing{
			ContentType:   "application/json",
			CorrelationId: corrId,
			ReplyTo:       q.Name, //定义将rpc调用发出去后，由那个队列名接收响应
			Body:          body,
		})
	if err != nil {
		return
	}
	//log.Printf("Rabbitmq发送的信息为：%s", body)
	result = make(chan *declare.LogChain, 10000)

	go func() {
		for {
			select {
			case <-ctx.Done():
				err = errors.New("查看超时")
				err = ch.Close()
				if err != nil {
					zap.S().Debug(err)
				}
				zap.S().Debug("查看日志超时，关闭生产者日志channel")
				return
			case d := <-msgs:
				if corrId == d.CorrelationId {
					result <- &declare.LogChain{
						Msg: d.Body,
					}
				}
			}
		}
	}()
	return
}
