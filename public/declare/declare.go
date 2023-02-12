package declare

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/streadway/amqp"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"time"
)

type K8sDeploymentInfo struct {
	Namespace      string `ini:"namespace"`
	DeploymentName string `ini:"deploymentName"`
}

//定义rabbitmq发送消息的结构体
type RabbitmqMsg struct {
	Exchange   string       `ini:"exchange" json:"exchange"`
	RoutingKey string       `ini:"routingKey" json:"routingKey"`
	TaskName   string       `ini:"taskName" json:"taskName"`
	TaskAction bool         `ini:"taskAction" json:"taskAction"`
	Loop       bool         `ini:"loop" json:"loop"`
	Action     string       `ini:"action" json:"action"`
	Params     ClientParams `ini:"params" json:"params"`
	ReceiveMsg ReceiveMsg   `ini:"receiveMsg" json:"receiveMsg"`
}

type ReceiveMsg struct {
	Exchange         string `ini:"exchange" json:"exchange"`
	BackQueueName    string `ini:"backQueueName" json:"backQueuename"`
	ReceiveQueueName string `ini:"receiveQueueName" json:"receiveQueueName"`
	RoutingKey       string `ini:"routingKey" json:"routingKey"`
	CorrId           string `ini:"corrId" json:"corrId"`
}

type ClientParams struct {
	RequestResource    string `form:"requestResource" json:"requestResource"`
	RoutingKey         string `form:"routingKey" json:"routingKey"`
	ActionRoutingKey   string `form:"actionRoutingKey" json:"actionRoutingKey"`
	LogRoutingKey      string `form:"logRoutingKey" json:"logRoutingKey"`
	ServiceTag         string `form:"serviceTag" json:"serviceTag"`
	Timeout            int    `form:"timeout" json:"timeout"`
	PodName            string `form:"podName" json:"podName"`
	Namespace          string `form:"namespace" json:"namespace"`
	DeploymentName     string `form:"deploymentName" json:"deploymentName"`
	ContainerName      string `form:"containerName" json:"containerName"`
	DeploymentIp       string `form:"deploymentIp" json:"deploymentIp"`
	UpdateImage        string `form:"updateImage" json:"updateImage"`
	DeploymentReplicas int    `form:"deploymentReplicas" json:"deploymentReplicas"`
	DestRepo string    `form:"destRepo" json:"destRepo"`
}

type ParamsRepo struct {
	SrcName    string `form:"srcName" json:"srcName"`
	DestName   string `form:"destName" json:"destName"`
	ServiceTag string `form:"serviceTag" json:"serviceTag"`
}
type Repo struct {
	Name       string `yaml:"name" ini:"name""`
	Domain     string `yaml:"domain" ini:"domain"`
	DockerRepo string `yaml:"dockerRepo" ini:"dockerRepo"`
	User       string `yaml:"user" ini:"user"`
	Password   string `yaml:"password" ini:"password"`
}
type ImagePushConfig struct {
	SrcRepo  Repo `ini:"srcRepo"`
	DestRepo Repo `ini:"destRepo"`
}
type Rabbitmq struct {
	User             string `ini:"user"`
	Password         string `ini:"password"`
	HostPort         string `ini:"hostPort"`
	Exchange         string `ini:"exchange"`
	TaskExchange     string `ini:"taskExchange"`
	ActionQueueName  string `ini:"actionQueueName"`
	LogQueueName     string `ini:"logQueueName"`
	ActionRoutingKey string `ini:"actionRoutingKey"`
	LogRoutingKey    string `ini:"logRoutingKey"`
	Timeout          int    `ini:"timeout"`
}

type SWConfig struct {
	RabbitmqInfo    Rabbitmq `ini:"rabbitmq" json:"rabbitmq"`
	SWDestImageInfo Repo     `ini:"swDistRegistry" json:"swDistRegistry"`
	FFDestImageInfo Repo     `ini:"ffDistRegistry"`
}

type YamlServiceagentConfig struct {
	RepoInfo []Repo   `yaml: "repoInfo"`
	Rabbitmq Rabbitmq `yaml:"rabbitmq"`
	Project  []Env    `yaml:"project"`
	Env      string   `yaml:"env"`
	Log      Config   `yaml:"log"`
}

// Config 整个项目的配置
type Config struct {
	Mode      string     `json:"mode"`
	Port      int        `json:"port"`
	LogConfig *LogConfig `json:"log"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level      string `json:"level"`
	Filename   string `json:"filename"`
	MaxSize    int    `json:"maxsize"`
	MaxAge     int    `json:"max_age"`
	MaxBackups int    `json:"max_backups"`
}
type YamlAgentagentConfig struct {
	Rabbitmq       Rabbitmq       `yaml:"rabbitmq"`
	K8sConfig      K8sConfig      `yaml:"k8sConfig"`
	DockerSrcRepo  Repo           `yaml:"dockerSrcRepo"`
	ShellScripts   []ShellScripts `yaml:"shellScripts"`
	Env            string         `yaml:"env"`
	DingdingConfig DingdingConfig `yaml:"dingdingConfig"`
	Log            Config         `yaml:"log"`
	DeploymentType string         `yaml:"deploymentType"`
}
type DingdingConfig struct {
	DingdingMessage     bool   `yaml:"dingdingMessage"`
	DingdingURL         string `yaml:"dingdingURL"`
	DingdingAccessToken string `yaml:"dingdingAccessToken"`
	DingdingSign        string `yaml:"dingdingSign"`
	DingdingMessageHead string `yaml:"dingdingMessageHead"`
	DingdingMessageTail string `yaml:"dingdingMessageTail"`
	DingdingAtAll       string `yaml:"dingdingAtAll"`
}
type ShellScripts struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}
type DeploymentInfo struct {
	Namespace         string
	DeploymentName    string
	Image             string
	Replicas          int32
	AvailableReplicas int32
}

type Container struct {
	ID      string       `json:"conterner"`
	Image   string       `json:"image"`
	Command string       `json:"command"`
	Create  string       `json:"create"`
	Ports   []types.Port `json:"ports"`
	State   string       `json:"state"`
	Status  string       `json:"status"`
	Name    string       `json:"name"`
}
type K8sContainer struct {
	ContainerNo  int    `json:"containerNo" `
	Ready        bool   `json:"ready" `
	RestartCount int32  `json:"restartCount" `
	Name         string `json:"name"`
}
type PodInfo struct {
	PodNo         int
	PodName       string
	PodNamespace  string
	PodStatus     v1.PodPhase
	PodRestart    int32
	PodCreateTime string
	PodAge        time.Duration
	Containers    []K8sContainer
}
type DeploymentByPod struct {
	DeploymentName string
	ReplicaSet     []ReplicaSet
	RsCount        int
	UnitCount      int
}
type ReplicaSet struct {
	ReplicaSetNo    int
	ReplicaSetCount int
	ContainerCount  int
	ReplicaSetName  string
	Pods            []PodInfo
}
type DeploymentOnly struct {
	DeploymentName string
	Pods           []PodInfoOnly
}

type PodInfoOnly struct {
	PodName       string
	PodNamespace  string
	PodStatus     v1.PodPhase
	PodRestart    int32
	PodCreateTime string
	PodAge        time.Duration
}

type K8s struct {
	ConfigPath   string
	Dockerset    *client.Client
	Clientset    *kubernetes.Clientset
	RabbitmqMsg  RabbitmqMsg
	Ctx          context.Context
	RabbitmqBase RabbitmqBase
}
type RabbitmqBase struct {
	Conn          *amqp.Connection
	Ch            *amqp.Channel
	CorrelationId string
	ReplyTo       string
}
type K8sConfig struct {
	ConfigPath          string   `yaml:"configPath"`
	DestImageInfo       Repo     `yaml:"destImageInfo`
	SrcRepo             Repo     `yaml:"srcRepo"`
	ContainerDeployment []string `form:"containerDeployment" json:"containerDeployment" yaml:"containerDeployment"`
}

type Result struct {
	Code   string
	Desc   string
	Result interface{}
}

const (
	StatusOk     = "000000"
	StatusFailed = "999999"
)

type Env struct {
	Name                string         `form:"name" json:"name" yaml:"name"`
	Desc                string         `form:"desc" json:"desc" yaml:"desc"`
	ActionRoutingKey    string         `form:"actionRoutingKey" json:"actionRoutingKey" yaml:"actionRoutingKey"`
	LogRoutingKey       string         `form:"logRoutingKey" json:"logRoutingKey" yaml:"logRoutingKey"`
	Repo                string         `form:"repo" json:"repo" yaml:"repo"`
	DeploymentIP        []string       `form:"deploymentIP" json:"deploymentIP" yaml:"deploymentIP"`
	Namespace           string         `form:"namespace" json:"namespace" yaml:"namespace"`
	DingdingConfig      DingdingConfig `form:"dingdingConfig" json:"dingdingConfig" yaml:"dingdingConfig"`
	ContainerDeployment []string       `form:"containerDeployment" json:"containerDeployment" yaml:"containerDeployment"`
}
type ResultMsg struct {
	Code   string
	Desc   string
	Env    Env
	Result map[string]interface{}
}
type K8sDeployInfo struct {
	ProjectName     string `json:"projectName"`
	DestImageDomain string `json:"destImageDomain"`
	DestImageRepo   string `json:"destImageRepo"`
}
type K8sIndex struct {
	ResultMsg     ResultMsg
	K8sDeployInfo K8sDeployInfo
}
type LogChain struct {
	Msg []byte
}

type DockerClient struct {
	Client *client.Client
	Repo   ImagePushConfig
	LogCh  chan []byte
}
type ImageOpslogs struct {
	c  []byte
	ch chan []byte
}
