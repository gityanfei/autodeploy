package task

import (
	"context"
	"github.com/docker/docker/api/types"
	"go.uber.org/zap"
	"public/declare"
	"time"
)

func GetDockerInfo() (result []declare.Container, err error) {
	var container = make([]declare.Container, 0)

	options := types.ContainerListOptions{
		All: true,
	}
	var listContainer = make([]types.Container, 0)
	listContainer, err = Dockerset.ContainerList(context.TODO(), options)
	if err != nil {
		zap.S().Error(err)
		return
	}

	for _, v := range listContainer {

		//时间日期格式模板
		timeTemplate := "2006-01-02 15:04:05"
		tm := time.Unix(int64(v.Created), 0)
		timeStr := tm.Format(timeTemplate)
		tmp := declare.Container{
			ID:      v.ID[0:12],
			Image:   v.Image,
			Command: v.Command,
			Create:  timeStr,
			Ports:   v.Ports,
			State:   v.State,
			Status:  v.Status,
			Name:    v.Names[0],
		}
		container = append(container, tmp)
	}
	return container, nil
}
func DeleteDocker(ID string) (err error) {
	err = Dockerset.ContainerRemove(context.TODO(), ID, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
	if err != nil {
		zap.S().Error(err)
		return
	}
	return
}
func RestartDocker(ID string) (err error) {
	a := time.Duration(3)
	err = Dockerset.ContainerRestart(context.TODO(), ID, &a)
	if err != nil {
		zap.S().Error(err)
		return
	}
	return
}
