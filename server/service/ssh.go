package service

import (
	"autoupdate/middlewares/task"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"public/declare"
)

func WsHandler() func(*gin.Context) {
	return func(c *gin.Context) {
		params := Analysis(c)
		//zap.S().Debug(params.Namespace, params.PodName, params.ContainerName)
		// 得到websocket长连接  CreateTask,GetPodLog
		err = task.StartWebsocket(c.Writer, c.Request, &params)
		if err != nil {
			zap.S().Debug("StartWebsocket:", err)
			return
		}

	}
}
func Home() func(*gin.Context) {
	return func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", "")
	}
}
func K8sTerminalController() func(*gin.Context) {
	return func(c *gin.Context) {
		var params = declare.ClientParams{}
		err = c.Bind(&params)
		if err != nil {
			zap.S().Debug(err)
			return
		}
		c.HTML(http.StatusOK, "k8sTerminal.html", params)
	}
}

func TerminalController() func(*gin.Context) {
	return func(c *gin.Context) {
		var params = declare.ClientParams{}
		err = c.Bind(&params)
		if err != nil {
			zap.S().Debug(err)
			return
		}
		c.HTML(http.StatusOK, "k8sTerminal.html", params)
	}
}
func DockerTerminalController() func(*gin.Context) {
	return func(c *gin.Context) {
		var params = declare.ClientParams{}
		err = c.Bind(&params)
		if err != nil {
			zap.S().Debug(err)
			return
		}
		c.HTML(http.StatusOK, "dockerTerminal.html", params)
	}
}
func LogTerminalController() func(*gin.Context) {
	return func(c *gin.Context) {
		var params = declare.ClientParams{}
		err = c.Bind(&params)
		if err != nil {
			zap.S().Debug(err)
			return
		}
		c.HTML(http.StatusOK, "logTerminal.html", params)
	}
}
