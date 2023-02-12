package task

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"public/declare"
	"strings"
	"time"
)

type K8s declare.K8s

//var agentConfig declare.YamlAgentagentConfig

func DeployUpdate(namespace, deploymentName, updateImage string) error {
	// 获取名为nginx的deployment信息
	deploymentList, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metaV1.GetOptions{})
	deploymentList.Spec.Template.Spec.Containers[0].Image = updateImage
	_, err = clientset.AppsV1().Deployments(namespace).Update(context.TODO(), deploymentList, metaV1.UpdateOptions{})
	if err != nil {
		return err
	}
	return err
}
func DeployScale(namespace string, deploymentName string, count int) error {
	tmpCount := int32(count)
	// 获取名为nginx的deployment信息
	deploymentList, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metaV1.GetOptions{})
	deploymentList.Spec.Replicas = &tmpCount
	_, err = clientset.AppsV1().Deployments(namespace).Update(context.TODO(), deploymentList, metaV1.UpdateOptions{})
	if err != nil {
		return err
	}
	return err
}

func DeploymentStatus(namespace string) (result []declare.DeploymentInfo, err error) {

	DeploymentClient := clientset.AppsV1().Deployments(namespace)
	DeploymentList, err := DeploymentClient.List(context.TODO(), metaV1.ListOptions{
		//metaV1.ListOptions可定义一个查询过滤条件，此条件根据需要填写，可以不写
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		//Watch: true, //启用监听，一直监控deployment状态
	})
	if err != nil {
		zap.S().Error(err)
		return nil, err
	}
	for _, deployment := range DeploymentList.Items {
		for _, containerDeployment := range agentConfig.K8sConfig.ContainerDeployment {
			if deployment.Name == containerDeployment {
				tmp := declare.DeploymentInfo{
					Namespace:         deployment.Namespace,
					DeploymentName:    deployment.Name,
					Replicas:          deployment.Status.Replicas,
					Image:             deployment.Spec.Template.Spec.Containers[0].Image,
					AvailableReplicas: deployment.Status.AvailableReplicas,
				}

				result = append(result, tmp)
				break
			}
		}
	}

	return
}

type TmpReplicaSet struct {
	ReplicaSetName string
	Pods           []v1.Pod
}

func GetPodByDeployment(namespace string, deploymentName string) (result []*TmpReplicaSet, err error) {
	//t1 := time.Now()

	// 0、 找到指定deployment的标签
	DeploymentClient := clientset.AppsV1().Deployments(namespace)
	DeploymentList, err := DeploymentClient.List(context.TODO(), metaV1.ListOptions{
		//metaV1.ListOptions可定义一个查询过滤条件，此条件根据需要填写，可以不写
		TypeMeta: metaV1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},

		//Watch: true, //启用监听，一直监控deployment状态
	})
	if err != nil {
		zap.S().Debug(err)
		return nil, err
	}
	deploymentLabel := make(map[string]string)
	for _, deployment := range DeploymentList.Items {
		if deployment.Name == deploymentName {
			deploymentLabel = deployment.Spec.Selector.MatchLabels
			break
		}
	}
	//zap.S().Debug("Deployment Selector标签：", convertMapToSelector(deploymentLabel))

	// 1、 找到标签对应的ReplicaSets，如果是在升级，可能存在多个RS
	ReplicaSetList, err1 := clientset.AppsV1().ReplicaSets(namespace).List(context.Background(), metaV1.ListOptions{
		TypeMeta: metaV1.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "extensions/v1beta1",
		},
		LabelSelector: convertMapToSelector(deploymentLabel),
	})
	if err1 != nil {
		zap.S().Debug(err)
		return nil, err1
	}
	rsS := make([]map[string]string, 0)
	replicaSettLabels := make([]string, 0)
	for _, replicaSet := range ReplicaSetList.Items {
		name := replicaSet.Name
		replicaSettLabel := make(map[string]string)
		if tmp := replicaSet.Spec.Replicas; *tmp > 0 {
			replicaSettLabel = replicaSet.Spec.Selector.MatchLabels
			tmpMap := make(map[string]string, 0)
			tmpMap[name] = convertMapToSelector(replicaSettLabel)
			rsS = append(rsS, tmpMap)
			replicaSettLabels = append(replicaSettLabels, convertMapToSelector(replicaSettLabel))
		}
	}
	//zap.S().Debug("RS标签：", replicaSettLabels, rsS)

	//2、根据replice的标签去找pod
	for _, rsM := range rsS {
		for rsName, rsLabel := range rsM {
			PodList, err2 := clientset.CoreV1().Pods(namespace).List(context.Background(), metaV1.ListOptions{
				TypeMeta: metaV1.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1",
				},
				LabelSelector: rsLabel,
			})
			if err2 != nil {
				zap.S().Debug(err)
				return nil, err2
			}
			tmpPods := make([]v1.Pod, 0)
			for _, pod := range PodList.Items {
				tmpPods = append(tmpPods, pod)
			}
			result = append(result, &TmpReplicaSet{
				ReplicaSetName: rsName,
				Pods:           tmpPods,
			})
			//zap.S().Debug("花费时间为：", time.Now().Sub(t1))
		}

	}

	return

}

func convertMapToSelector(labels map[string]string) string {
	var l []string
	for k, v := range labels {
		l = append(l, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(l, ",")
}

func PodStatus(namespace string) []declare.DeploymentByPod {
	DeploymentByPod := make([]declare.DeploymentByPod, 0)
	for _, deploymentName := range agentConfig.K8sConfig.ContainerDeployment {

		podCount := 0
		containerSum := 0
		podByContainerCount := 0
		rsPods, err := GetPodByDeployment(namespace, deploymentName)
		if err != nil {
			return nil
		}
		rsTmp := make([]declare.ReplicaSet, 0)
		for x, rs := range rsPods {
			info := make([]declare.PodInfo, 0)
			podByContainerCount = 0
			for i, pod := range rs.Pods {

				podCount++
				containerInfo := make([]declare.K8sContainer, 0)
				containerCount := len(pod.Status.ContainerStatuses)
				//zap.S().Debug("containerCount:    ", containerCount)
				if containerCount != 0 {
					for w, container := range pod.Status.ContainerStatuses {
						containerSum++
						podByContainerCount++
						tmp := declare.K8sContainer{
							ContainerNo:  w + 1,
							Name:         container.Name,
							Ready:        container.Ready,
							RestartCount: container.RestartCount,
						}
						containerInfo = append(containerInfo, tmp)
					}
					var restartCount int32
					for _, v := range pod.Status.ContainerStatuses {
						restartCount += v.RestartCount
					}
					tmp := declare.PodInfo{
						PodNo:        i + 1,
						PodName:      pod.ObjectMeta.Name,
						PodNamespace: pod.Namespace,
						PodStatus:    pod.Status.Phase,
						PodRestart:   restartCount,
						Containers:   containerInfo,
					}
					if pod.Status.StartTime != nil {
						tmp.PodCreateTime = pod.Status.StartTime.Format("2006-01-02 15:04:05")
						tmp.PodAge = time.Now().Sub(pod.Status.StartTime.Time)
					}
					info = append(info, tmp)
				} else {
					containerSum++
					podByContainerCount++
					tmp := declare.PodInfo{
						PodName:      pod.ObjectMeta.Name,
						PodNamespace: pod.Namespace,
						PodStatus:    pod.Status.Phase,
					}
					info = append(info, tmp)
				}
			}
			rsTmp = append(rsTmp, declare.ReplicaSet{
				ReplicaSetNo:    x + 1,
				ReplicaSetCount: podCount,
				ContainerCount:  podByContainerCount,
				ReplicaSetName:  rs.ReplicaSetName,
				Pods:            info,
			})
		}
		DeploymentByPod = append(DeploymentByPod, declare.DeploymentByPod{
			DeploymentName: deploymentName,
			ReplicaSet:     rsTmp,
			UnitCount:      containerSum,
		})
	}
	return DeploymentByPod
}
func DeletePod(namespace, podName string) (err error) {
	//var ctx context.Context
	err = clientset.CoreV1().Pods(namespace).Delete(context.TODO(), podName, metaV1.DeleteOptions{})
	if err != nil {
		return
	}
	return
}
