package service

import (
	"fmt"
	"public/declare"
)

func GetEnv(env string) (result declare.Env) {
	var repoInfo string
	for k, v := range agentConfig.RepoInfo {
		//zap.S().Debug(k, v)
		if v.Name == env {
			repoInfo = fmt.Sprintf("%s/%s/", agentConfig.RepoInfo[k].Domain, agentConfig.RepoInfo[k].DockerRepo)
			//log.Printf("public.go %s/%s/", agentConfig.RepoInfo[k].Domain, agentConfig.RepoInfo[k].DockerRepo)
		}
	}
	for _, v := range agentConfig.Project {
		if v.Name == env {
			result = v
		}
		result.Repo = repoInfo
	}
	//log.Printf("%#v\n", result)
	return
}
