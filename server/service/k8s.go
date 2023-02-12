package service

import (
	"autoupdate/middlewares/rpc"
	"autoupdate/middlewares/task"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"log"
	"net/http"
	"public/declare"
	"runtime/pprof"
	"strings"
	"time"
)

func K8sIndex() func(c *gin.Context) {
	return func(c *gin.Context) {
		var deploymentInfo []declare.DeploymentInfo
		var podInfo []declare.DeploymentByPod
		var flag = true
		//go func() {
		tmp := Analysis(c)
		deploymentInfo, err = K8sGetDeploymentByNamespace(&tmp)
		if err != nil {
			flag = false
			c.HTML(http.StatusOK, "index-k8s.tmpl", declare.ResultMsg{
				Code: declare.StatusFailed,
				Desc: "获取信息失败",
				Env:  GetEnv(c.Query(agentConfig.Env)),
				Result: map[string]interface{}{
					"error": err,
				},
			})
			return
		}

		podInfo, err = K8sGetPodByNamespace(&tmp)
		if err != nil {
			flag = false
			c.HTML(200, "index-k8s.tmpl", declare.ResultMsg{
				Code: declare.StatusFailed,
				Desc: "获取信息失败",
				Env:  GetEnv(c.Query(agentConfig.Env)),
				Result: map[string]interface{}{
					"error": err,
				},
			})
			return
		}

		if flag {
			c.HTML(200, "index-k8s.tmpl", declare.ResultMsg{
				Code: declare.StatusOk,
				Desc: "成功",
				Env:  GetEnv(c.Query(agentConfig.Env)),
				Result: map[string]interface{}{
					"deployinfo": deploymentInfo,
					"podinfo":    podInfo,
				},
			})
			return
		}
		pprof.StopCPUProfile()
	}
}

func Analysis(c *gin.Context) (params declare.ClientParams) {
	err = c.Bind(&params)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":   100000,
			"desc":   "从FORM中解析到成JSON错误",
			"result": err,
			"params": params,
		})
	}
	params.RequestResource = GetUri(c)
	return
}

func Ping() func(c *gin.Context) {
	return func(c *gin.Context) {

		var deploymentInfo []declare.DeploymentInfo
		var podInfo []declare.DeploymentByPod

		tmp := Analysis(c)
		deploymentInfo, err = K8sGetDeploymentByNamespace(&tmp)
		if err != nil {
			c.JSON(200, declare.ResultMsg{
				Code: declare.StatusFailed,
				Desc: "获取信息失败",
				Env:  GetEnv(c.Query(agentConfig.Env)),
				Result: map[string]interface{}{
					"error": err,
				},
			})
			return
		}

		c.JSON(200, declare.ResultMsg{
			Code: declare.StatusOk,
			Desc: "成功",
			Env:  GetEnv(c.Query(agentConfig.Env)),
			Result: map[string]interface{}{
				"deployinfo": deploymentInfo,
				"podinfo":    podInfo,
			},
		})

	}
}
func K8sGetDeploymentByNamespace(params *declare.ClientParams) (result []declare.DeploymentInfo, err error) {
	var send = new(declare.RabbitmqMsg)
	taskName := task.RandomString(32)

	send = &declare.RabbitmqMsg{
		Exchange:   agentConfig.Rabbitmq.Exchange,
		RoutingKey: params.RoutingKey,
		TaskName:   taskName,
		TaskAction: true,
		Action:     "GetK8sDeployment",
		Params:     *params,
		ReceiveMsg: declare.ReceiveMsg{
			Exchange:         agentConfig.Rabbitmq.TaskExchange,
			ReceiveQueueName: taskName,
			RoutingKey:       taskName,
			CorrId:           taskName,
		},
	}
	msg := task.InitTask(send)
	var tmp = new([]byte)
	tmp, err = msg.SendCreateTask(&task.WsConnection{})
	if err != nil {
		zap.S().Debug(err)
		return
	}
	err = json.Unmarshal(*tmp, &result)
	if err != nil {
		zap.S().Debug("JSON解析结果：", err)
		return
	}
	return
}
func K8sGetPodByNamespace(params *declare.ClientParams) (result []declare.DeploymentByPod, err error) {
	var send = new(declare.RabbitmqMsg)
	taskName := task.RandomString(32)
	send = &declare.RabbitmqMsg{
		Exchange:   agentConfig.Rabbitmq.Exchange,
		RoutingKey: params.RoutingKey,
		TaskName:   taskName,
		TaskAction: true,
		Action:     "GetK8sPod",
		Params:     *params,
		ReceiveMsg: declare.ReceiveMsg{
			Exchange:         agentConfig.Rabbitmq.TaskExchange,
			ReceiveQueueName: taskName,
			RoutingKey:       taskName,
			CorrId:           taskName,
		},
	}
	msg := task.InitTask(send)
	var tmp = new([]byte)
	tmp, err = msg.SendCreateTask(&task.WsConnection{})
	if err != nil {
		zap.S().Debug(err)
		return
	}
	err = json.Unmarshal(*tmp, &result)
	if err != nil {
		zap.S().Debug(err)
		return
	}
	return
}

func Mid(action string, c *gin.Context) (result []byte, err error) {
	var params declare.ClientParams
	err = c.Bind(&params)
	log.Printf("%#v\n", params)
	if err != nil {
		//log.Printf("%#v\n", params)
		c.JSON(http.StatusOK, gin.H{
			"code":   100000,
			"desc":   "从FORM中解析到成JSON错误",
			"result": err,
			"params": params,
		})
		return
	}
	s := rpc.RandomString(32)
	sendRabbitmqStruct := rpc.Msg{
		Exchange:   agentConfig.Rabbitmq.Exchange,
		RoutingKey: params.RoutingKey,
		TaskName:   s,
		TaskAction: true,
		Action:     action,
		Params:     params,
	}
	if sendRabbitmqStruct.Params.Timeout == 0 {
		sendRabbitmqStruct.Params.Timeout = agentConfig.Rabbitmq.Timeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(sendRabbitmqStruct.Params.Timeout)*time.Second)
	defer cancel()
	result, err = sendRabbitmqStruct.RemoteRPC(ctx)
	zap.S().Debug("接收到的信息是：", string(result))
	return
}
func GetUri(c *gin.Context) (lastUrl string) {
	url := c.Request.URL.Path
	lastUrls := strings.Split(url, "/")
	lastUrl = lastUrls[len(lastUrls)-1]
	return
	//zap.S().Debug(lastUrl)
}

func K8sDeletePod() func(c *gin.Context) {
	return func(c *gin.Context) {
		result := new(declare.Result)
		params := Analysis(c)
		var send = new(declare.RabbitmqMsg)
		taskName := task.RandomString(32)
		send = &declare.RabbitmqMsg{
			Exchange:   agentConfig.Rabbitmq.Exchange,
			RoutingKey: params.RoutingKey,
			TaskName:   taskName,
			TaskAction: true,
			Action:     "DeleteK8sPod",
			Params:     params,
			ReceiveMsg: declare.ReceiveMsg{
				Exchange:         agentConfig.Rabbitmq.TaskExchange,
				ReceiveQueueName: taskName,
				RoutingKey:       taskName,
				CorrId:           taskName,
			},
		}
		msg := task.InitTask(send)
		var tmp = new([]byte)
		tmp, err = msg.SendCreateTask(&task.WsConnection{})
		if err != nil {
			zap.S().Debug(err)
			return
		}
		err = json.Unmarshal(*tmp, result)
		if err != nil {
			c.HTML(http.StatusOK, "general.tmpl", declare.Result{
				Code:   "300000",
				Desc:   "JSON解析失败",
				Result: err,
			})
			return
		}
		c.HTML(http.StatusOK, "general.tmpl", result)
	}
}
func K8sDeployUpdate() func(c *gin.Context) {
	return func(c *gin.Context) {
		result := new(declare.ResultMsg)
		params := Analysis(c)
		var send = new(declare.RabbitmqMsg)
		taskName := task.RandomString(32)
		send = &declare.RabbitmqMsg{
			Exchange:   agentConfig.Rabbitmq.Exchange,
			RoutingKey: params.RoutingKey,
			TaskName:   taskName,
			TaskAction: true,
			Action:     "DeployUpdate",
			Params:     params,
			ReceiveMsg: declare.ReceiveMsg{
				Exchange:         agentConfig.Rabbitmq.TaskExchange,
				ReceiveQueueName: taskName,
				RoutingKey:       taskName,
				CorrId:           taskName,
			},
		}
		msg := task.InitTask(send)
		var tmp = new([]byte)
		tmp, err = msg.SendCreateTask(&task.WsConnection{})
		if err != nil {
			zap.S().Debug(err)
			return
		}
		err = json.Unmarshal(*tmp, result)
		if err != nil {
			c.HTML(http.StatusOK, "general.tmpl", declare.Result{
				Code:   "300000",
				Desc:   "JSON解析失败",
				Result: err,
			})
			return
		}
		sendToDingDing(params.DestRepo,params.ServiceTag)
		c.HTML(http.StatusOK, "general.tmpl", result)
	}
}

func K8sScale() func(c *gin.Context) {
	return func(c *gin.Context) {
		result := new(declare.ResultMsg)
		params := Analysis(c)
		var send = new(declare.RabbitmqMsg)
		taskName := task.RandomString(32)
		send = &declare.RabbitmqMsg{
			Exchange:   agentConfig.Rabbitmq.Exchange,
			RoutingKey: params.RoutingKey,
			TaskName:   taskName,
			TaskAction: true,
			Action:     "DeploymentScale",
			Params:     params,
			ReceiveMsg: declare.ReceiveMsg{
				Exchange:         agentConfig.Rabbitmq.TaskExchange,
				ReceiveQueueName: taskName,
				RoutingKey:       taskName,
				CorrId:           taskName,
			},
		}
		msg := task.InitTask(send)
		var tmp = new([]byte)
		tmp, err = msg.SendCreateTask(&task.WsConnection{})
		if err != nil {
			zap.S().Debug(err)
			return
		}
		err = json.Unmarshal(*tmp, result)
		if err != nil {
			c.HTML(http.StatusOK, "general.tmpl", declare.Result{
				Code:   "300000",
				Desc:   "JSON解析失败",
				Result: err,
			})
			return
		}
		c.HTML(http.StatusOK, "general.tmpl", result)
	}
}
