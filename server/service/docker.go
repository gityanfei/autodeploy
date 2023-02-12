package service

import (
	"autoupdate/helper"
	"autoupdate/middlewares/task"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"public/tools"
	"strings"

	"log"
	"net/http"
	"public/conf"
	"public/declare"
	"time"
)

var agentConfig, err = conf.GetServiceYamlConfig()

func PullImageAndPushImage() func(c *gin.Context) {
	return func(c *gin.Context) {
		var params = declare.ParamsRepo{}
		err = c.Bind(&params)
		if err != nil {
			c.String(http.StatusOK, "绑定查询参数失败")
			return
		}
		fmt.Printf("%#v\n", params)

		var srcRepo declare.Repo
		var destRepo declare.Repo
		var flag1 bool
		var flag2 bool
		for k, v := range agentConfig.RepoInfo {
			//fmt.Println(k, v)
			if v.Name == params.DestName {
				destRepo = agentConfig.RepoInfo[k]
				flag1 = true
			}
			if v.Name == params.SrcName {
				srcRepo = agentConfig.RepoInfo[k]
				flag2 = true
			}
		}
		if !flag1 || !flag2 {
			c.String(http.StatusOK, "使用了配置文件中未配置的源/目标仓库，请检查URL参数\n")
			return
		}


		d := helper.NewDockerClient()
		d = &helper.Docker{
			Client: d.Client,
			Repo: declare.ImagePushConfig{
				SrcRepo:  srcRepo,
				DestRepo: destRepo,
			},
		}
		//log.Printf("%#v\n", d.Repo)
		imageName := fmt.Sprintf("%s/%s/%s", d.Repo.SrcRepo.Domain, d.Repo.SrcRepo.DockerRepo, strings.TrimSpace(params.ServiceTag))
		newImageName := fmt.Sprintf("%s/%s/%s", d.Repo.DestRepo.Domain, d.Repo.DestRepo.DockerRepo, params.ServiceTag)

		var chanLog = helper.ImageOpslogs{}
		chanLog.LogCh = make(chan []byte, 1000)
		log.Printf("开始拉取镜像%s\n", imageName)
		//定义持续将日志打印到浏览器的响应头，不然不能持续输出
		c.Writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.String(http.StatusOK, "镜像拉取开始，如果持续没有后续动作，请检查服务名和tag是否在镜像仓库中存在......\n")
		c.String(http.StatusOK, "检查方式：docker pull %s\n", imageName)
		c.String(http.StatusOK, "镜像的源为：%s\n", imageName)
		c.String(http.StatusOK, "镜像的推送地址为：%s\n", newImageName)
		c.Writer.Flush()
		ctx, cancel := context.WithCancel(context.Background())
		go d.PullImage(imageName, &chanLog, cancel)
		zap.S().Debug(imageName)
		if err != nil {
			zap.S().Debug(err)
		}

		for {
			select {
			case a := <-chanLog.LogCh:
				c.String(http.StatusOK, string(a))
				c.Writer.Flush()
				//fmt.Println("通道取值：", string(a))
			case <-ctx.Done():
				c.String(http.StatusOK, "镜像拉取结束...........\n")
				log.Printf("拉取镜像结束%s\n", imageName)
				c.Writer.Flush()
				//fmt.Println("结束...........")
				goto next1
			default:
				//goto loop1
				time.Sleep(50 * time.Millisecond)
				//fmt.Println("等待镜像拉取发送消息...........")
			}
		}
	next1:
		c.String(http.StatusOK, "镜像推送开始......\n")
		c.Writer.Flush()
		d.Client.ImageTag(context.TODO(), imageName, newImageName)
		log.Printf("开始推送镜像%s\n", newImageName)
		var chanLog2 = helper.ImageOpslogs{}
		chanLog2.LogCh = make(chan []byte, 1000)
		ctx2, cancel2 := context.WithCancel(context.Background())
		go d.PushImage(newImageName, &chanLog2, cancel2)
		for {
			select {
			case a := <-chanLog2.LogCh:
				c.String(http.StatusOK, string(a))
				c.Writer.Flush()
				//fmt.Println("通道取值：", string(a))
			case <-ctx2.Done():
				c.String(http.StatusOK, "镜像推送结束...........\n")
				log.Printf("镜像推送结束%s\n", newImageName)

				c.Writer.Flush()
				//fmt.Println("结束...........")
				goto next2
			default:
				//goto loop1
				time.Sleep(50 * time.Millisecond)
				//fmt.Println("等待镜像推送发送消息...........")
			}
		}
	next2:
		//删除本地镜像
		d.DeleteImage(imageName)
		d.DeleteImage(newImageName)
		c.String(http.StatusOK, "删除本地镜像完成\n")
		c.Writer.Flush()
		c.String(http.StatusOK, "镜像推送完成\n")
		//如果开启了dingding消息推送则发送消息到钉钉
		c.Writer.Flush()
	}
}
func sendToDingDing(destName ,targetImageName string){
	var currentProjectInfo declare.DingdingConfig
	for k, v := range agentConfig.Project {
		//fmt.Println(k, v)
		if v.Name ==  destName {
			currentProjectInfo = agentConfig.Project[k].DingdingConfig
		}
	}
	var dingding tools.DingdingConfig
	if currentProjectInfo.DingdingMessage {
		dingding = tools.DingdingConfig{
			DingdingAccessToken: currentProjectInfo.DingdingAccessToken,
			DingdingURL:         currentProjectInfo.DingdingURL,
			DingdingMessageHead: currentProjectInfo.DingdingMessageHead,
			DingdingMessageTail: currentProjectInfo.DingdingMessageTail,
			DingdingSign:        currentProjectInfo.DingdingSign,
		}
		messageToDingding := fmt.Sprintf("%s%s%s", dingding.DingdingMessageHead, targetImageName, dingding.DingdingMessageTail)
		tmp, err1:= dingding.SamplePost(messageToDingding)
 if err1!=nil{
 	zap.S().Error("发送消息到钉钉出错",err1.Error())
 }
		zap.S().Info("发送消息到钉钉响应为：",string(tmp))

	}
}
func RestartDocker() func(c *gin.Context) {
	return func(c *gin.Context) {
		result := declare.ResultMsg{}
		params := Analysis(c)
		var send = new(declare.RabbitmqMsg)
		taskName := task.RandomString(32)
		send = &declare.RabbitmqMsg{
			Exchange:   agentConfig.Rabbitmq.Exchange,
			RoutingKey: params.RoutingKey,
			TaskName:   taskName,
			TaskAction: true,
			Action:     "RestartDocker",
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
		err = json.Unmarshal(*tmp, &result)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":   "300000",
				"desc":   "列出docker容器失败",
				"result": err,
			})
			return
		}
		c.HTML(http.StatusOK, "general.tmpl", result)
	}
}
func DeleteDocker() func(c *gin.Context) {
	return func(c *gin.Context) {
		result := declare.ResultMsg{}
		params := Analysis(c)
		var send = new(declare.RabbitmqMsg)
		taskName := task.RandomString(32)
		send = &declare.RabbitmqMsg{
			Exchange:   agentConfig.Rabbitmq.Exchange,
			RoutingKey: params.RoutingKey,
			TaskName:   taskName,
			TaskAction: true,
			Action:     "DeleteDocker",
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
		err = json.Unmarshal(*tmp, &result)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":   "300000",
				"desc":   "列出docker容器失败",
				"result": err,
			})
			return
		}

		c.HTML(http.StatusOK, "general.tmpl", result)

	}
}
func DockerIndex() func(c *gin.Context) {
	return func(c *gin.Context) {
		result := make([]declare.Container, 0)
		params := Analysis(c)
		var send = new(declare.RabbitmqMsg)
		taskName := task.RandomString(32)
		send = &declare.RabbitmqMsg{
			Exchange:   agentConfig.Rabbitmq.Exchange,
			RoutingKey: params.RoutingKey,
			TaskName:   taskName,
			TaskAction: true,
			Action:     "GetDockerInfo",
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
			zap.S().Error(err)
			c.JSON(http.StatusOK, gin.H{
				"code":   "300000",
				"desc":   "列出docker容器失败",
				"result": err,
			})
			return
		}
		err = json.Unmarshal(*tmp, &result)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":   "300000",
				"desc":   "列出docker容器失败",
				"result": err,
			})
			return
		}
		c.HTML(http.StatusOK, "index-docker.tmpl", declare.ResultMsg{
			Code: declare.StatusOk,
			Desc: "成功",
			Env:  GetEnv(c.Query(agentConfig.Env)),
			Result: map[string]interface{}{
				"containers": result,
			},
		})

	}
}
func UpdateDocker() func(c *gin.Context) {
	return func(c *gin.Context) {
		var params = declare.ClientParams{}
		err = c.Bind(&params)
		if err != nil {
			zap.S().Debug(err)
			return
		}
		sendToDingDing(params.DestRepo,params.ServiceTag)
		c.HTML(http.StatusOK, "dockerUpdateTerminal.html", params)
	}
}
func DockerGetLog() func(c *gin.Context) {
	return func(c *gin.Context) {
		var params = declare.ClientParams{}
		err = c.Bind(&params)
		if err != nil {
			zap.S().Debug(err)
			return
		}
		c.HTML(http.StatusOK, "dockerLogTerminal.html", params)
	}
}
