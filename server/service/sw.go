package service

import (
	"github.com/streadway/amqp"
	"public/declare"
)

type Rabbitmq struct {
	User            string `ini:"user"`
	Password        string `ini:"password"`
	HostPort        string `ini:"hostPort"`
	ProductionQueue string `ini:"productionQueue"`
}

type SWConfig struct {
	RabbitmqInfo    Rabbitmq     `ini:"rabbitmq" json:"rabbitmq"`
	SWDestImageInfo declare.Repo `ini:"swDistRegistry" json:"swDistRegistry"`
	FFDestImageInfo declare.Repo `ini:"ffDistRegistry"`
}

var (
	conn *amqp.Connection
	ch   *amqp.Channel
	//agentConfig = SWConfig{}
)
