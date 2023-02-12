package task

import (
	"context"
	"fmt"
	"github.com/docker/docker/client"
	"github.com/streadway/amqp"
	"go.uber.org/zap"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"public/conf"
	"public/declare"
)

var conn *amqp.Connection
var agentConfig declare.YamlAgentagentConfig

var clientset *kubernetes.Clientset
var restConf *rest.Config

var Dockerset *client.Client
var ctx context.Context

func init() {
	var err error
	//读取配置文件
	agentConfig, err = conf.GetAgentYamlConfig()
	if err != nil {
		panic(err)
	}
	// 初始化任务包中的Rabbitmq连接
	dsn := fmt.Sprintf("amqp://%s:%s@%s/", agentConfig.Rabbitmq.User, agentConfig.Rabbitmq.Password, agentConfig.Rabbitmq.HostPort)
	zap.S().Debug(dsn)
	conn, err = amqp.Dial(dsn)
	if err != nil {
		zap.S().Error(err)
		zap.S().Error("处理Task连接Rabbitmq失败")
	}
	switch agentConfig.DeploymentType {
	case "k8s":
		// 初始化k8s配置 restful
		if clientset, restConf, err = InitClient(); err != nil {
			zap.S().Error(err)
		}

		var config *rest.Config
		zap.S().Debug("读取到K8s连接配置文件为",agentConfig.K8sConfig.ConfigPath)
		config, err = clientcmd.BuildConfigFromFlags("", agentConfig.K8sConfig.ConfigPath)
		if err != nil {
			zap.S().Error(err)
		}
		GlobalK8sConfig, err = kubernetes.NewForConfig(config)
		if err != nil {
			zap.S().Error(err)
		}
		zap.S().Info("初始化K8S成功")

	case "docker":
		// 初始化docker配置
		ctx = context.Background()
		Dockerset, err = client.NewClientWithOpts(client.WithAPIVersionNegotiation())
		if err != nil {
			zap.S().Error(err)
		}
		zap.S().Info("初始化Docker成功")
	}
}
// 初始化k8s客户端
func InitClient() (clientset *kubernetes.Clientset, restConf *rest.Config, err error) {

	if restConf, err = GetRestConf(); err != nil {
		zap.S().Error("初始化K8S连接失败")
		return
	}

	// 生成clientset配置
	if clientset, err = kubernetes.NewForConfig(restConf); err != nil {
		zap.S().Error("初始化K8S连接配置失败")
		goto END
	}
END:
	return
}

// 获取k8s restful client配置
func GetRestConf() (restConf *rest.Config, err error) {
	var (
		kubeconfig []byte
	)

	// 读kubeconfig文件
	if kubeconfig, err = ioutil.ReadFile(agentConfig.K8sConfig.ConfigPath); err != nil {
		goto END
	}
	// 生成rest client配置
	if restConf, err = clientcmd.RESTConfigFromKubeConfig(kubeconfig); err != nil {
		goto END
	}
END:
	return
}