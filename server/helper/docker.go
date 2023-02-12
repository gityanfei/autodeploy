package helper

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
	"io"
	"public/declare"
)

type Docker declare.DockerClient

const (
	DockerClientVersion = "v20.10.14"
)

func NewDockerClient() *Docker {

	cli, err := client.NewClientWithOpts(client.WithAPIVersionNegotiation())
	if err != nil {
		zap.S().Error("初始化Docker失败", err.Error())
		return nil
	}

	return &Docker{
		Client: cli,
	}
}

type ImageOpslogs struct {
	LogCh chan []byte
}

func (i *ImageOpslogs) Write(p []byte) (n int, err error) {
	n = len(p)
	//fmt.Println(string(p))
	i.LogCh <- p
	return
}

// PushImage --> pull image to harbor仓库
func (d *Docker) PushImage(image string, log *ImageOpslogs, f context.CancelFunc) {
	var option = types.ImagePushOptions{}

	authConfig := types.AuthConfig{
		Username: d.Repo.DestRepo.User,
		Password: d.Repo.DestRepo.Password,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	option = types.ImagePushOptions{
		RegistryAuth: authStr,
	}

	fmt.Println(option)
	out, err := d.Client.ImagePush(context.TODO(), image, option)
	if err != nil {
		return
	}
	defer out.Close()
	_, err = io.Copy(log, out)
	if err != nil {
		fmt.Println(err)
		return
	}
	//fmt.Println("进入pullImage方法4")
	close(log.LogCh)
	f()
	if err != nil {
		return
	}
	return
}
func (d *Docker) PullImage(name string, log *ImageOpslogs, f context.CancelFunc) {
	//fmt.Println("进入pullImage方法1")

	var option = types.ImagePullOptions{}

	authConfig := types.AuthConfig{
		Username: d.Repo.SrcRepo.User,
		Password: d.Repo.SrcRepo.Password,
	}
	encodedJSON, err := json.Marshal(authConfig)
	if err != nil {
		return
	}
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	option = types.ImagePullOptions{
		RegistryAuth: authStr,
	}
	//fmt.Println("进入pullImage方法2")
	resp, err := d.Client.ImagePull(context.TODO(), name, option)
	if err != nil {
		return
	}
	defer resp.Close()

	//fmt.Println("进入pullImage方法3")
	_, err = io.Copy(log, resp)
	if err != nil {
		fmt.Println(err)
		return
	}
	//fmt.Println("进入pullImage方法4")
	close(log.LogCh)
	f()
	return
}
func (d *Docker) DeleteImage(imageName string) {
	_, _ = d.Client.ImageRemove(context.TODO(), imageName, types.ImageRemoveOptions{Force: true})
}
