package router

import (
	"autoupdate/logger"
	"autoupdate/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func Router() *gin.Engine {
	err := logger.InitLogger()
	if err != nil {
		zap.S().Error("FAILED")
	}

	//r := gin.New()
	r := gin.Default()

	//r.Use(logger.GinLogger(), logger.GinRecovery(true))
	r.StaticFile("/favicon.ico", "./html/image/favicon.ico")

	r.LoadHTMLGlob("html/template/**/*")
	r.Static("/static", "html/static")
	k8s := r.Group("/k8s")
	{
		k8s.GET("/", service.K8sIndex())
		k8s.GET("/podDelete", service.K8sDeletePod())
		k8s.GET("/terminal", service.TerminalController())
		k8s.GET("/k8sssh", service.WsHandler())
		k8s.GET("/podLogTerminal", service.LogTerminalController())
		k8s.GET("/podLog", service.WsHandler())
		k8s.GET("/deployUpdate", service.K8sDeployUpdate())
		k8s.GET("/scaleDeployment", service.K8sScale())
		k8s.GET("/ping", service.Ping())

		//feifan.GET("/podDelete", router.FeifanDeletePod())
	}
	docker := r.Group("/docker")
	{
		docker.GET("/", service.DockerIndex())
		docker.GET("/terminal", service.DockerTerminalController())
		docker.GET("/dockerssh", service.WsHandler())
		docker.GET("/pushImage", service.PullImageAndPushImage())
		docker.GET("/restartDocker", service.RestartDocker())
		docker.GET("/deleteDocker", service.DeleteDocker())
		docker.GET("/dockerLogTerminal", service.DockerGetLog())
		docker.GET("/dockerLog", service.WsHandler())
		docker.GET("/updateDockerTerminal", service.UpdateDocker())
		docker.GET("/updateDocker", service.WsHandler())

	}
	r.GET("/home", service.Home())
	r.GET("/", service.ManagerIndex())
	return r
}
