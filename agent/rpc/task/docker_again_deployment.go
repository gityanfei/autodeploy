package task

import (
	"bufio"
	"go.uber.org/zap"
	"io"
	"os/exec"
	"public/declare"
)

func (t *RpcTask) InitUpdateDocker(currentMsg *declare.RabbitmqMsg) {

	// 执行exec，获取到容器终端的连接
	zap.S().Debug("部署的IP：", currentMsg.Params.DeploymentIp)
	zap.S().Debug("部署的版本号：", currentMsg.Params.ServiceTag)

	go t.GeneralTaskHandler.ReDeployDockerService(currentMsg.Params.ServiceTag, currentMsg.Params.DeploymentIp)
	go t.GeneralTaskHandler.WriteLoop()

}
func (log *GeneralTaskHandler) ReDeployDockerService(tag, ip string) {
	var err error
	//读取配置文件中的脚本路径
	var scriptPath string
	for _, v := range agentConfig.ShellScripts {
		if agentConfig.Env == v.Name {
			scriptPath = v.Path
		}
	}
	// 定义执行的命令
	cmd := exec.Command("/bin/sh", "-x", scriptPath, tag, ip)
	zap.S().Debug("执行的命令为：/bin/sh \n", scriptPath, tag, ip)
	// 定义输入输出流对象
	var stdout  io.ReadCloser
	//var stdout, stderr io.ReadCloser
	stdout, err = cmd.StdoutPipe()
	//stderr, err = cmd.StderrPipe()
	if err != nil {
		zap.S().Error(err)
		return
	}
	//从对象中获取数据
	readout := bufio.NewReader(stdout)
	//readerr := bufio.NewReader(stderr)
	err = cmd.Start()
	if err != nil {
		zap.S().Error(err)
	}
	for {
		select {
		case <-log.ReadClose:
			//zap.S().Debug("【关闭任务】ReadLoop  log.ReadClose 接收到关闭日志信号")
			goto END
		default:
			outputBytes := make([]byte, 1000)
			//outputErrBytes := make([]byte, 1000)
			var nOut int
			//var nErrOut int
			nOut, err = readout.Read(outputBytes) //获取屏幕的实时输出(并不是按照回车分割)
			if err != nil {
				if err == io.EOF {
					zap.S().Debug(err)
					goto END

				}
				zap.S().Error("发送到outChan通道的数据为：%s", err.Error())
				log.LogMessageChan <- []byte(err.Error())
			}
			// 将脚本的标准错误输出一并输出
			//nErrOut, err = readerr.Read(outputErrBytes)
			//if err != nil {
			//	if err == io.EOF {
			//		zap.S().Debug(err)
			//		goto END
			//	}
			//	zap.S().Error("发送到outChan通道的数据为：%s", err.Error())
			//	log.LogMessageChan <- []byte(err.Error())
			//}
			zap.S().Debug("发送到outChan通道的数据为：%s", string(outputBytes[:nOut]))
			log.LogMessageChan <- outputBytes[:nOut]
			//log.LogMessageChan <- outputErrBytes[:nErrOut]
		}
	}
END:
	zap.S().Debug("【关闭任务】关闭UpdateDocker日志任务读协程完成")
	return
}
