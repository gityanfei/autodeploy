package conf

import (
	"github.com/spf13/viper"
	"public/declare"
)

func GetServiceYamlConfig() (result declare.YamlServiceagentConfig, err error) {
	vip := viper.New()
	vip.AddConfigPath("./config/") //设置读取的文件路径
	vip.SetConfigName("config")    //设置读取的文件名
	vip.SetConfigType("yaml")      //设置文件的类型
	//尝试进行配置读取
	if err = vip.ReadInConfig(); err != nil {
		panic(err)
	}

	err = vip.Unmarshal(&result)
	if err != nil {
		panic(err)
	}
	return
}

// GetAgentYamlConfig 从文件中读取agent端配置
func GetAgentYamlConfig() (result declare.YamlAgentagentConfig, err error) {
	vip := viper.New()
	vip.AddConfigPath("./config/")    //设置读取的文件路径
	vip.SetConfigName("agent-config") //设置读取的文件名
	vip.SetConfigType("yaml")         //设置文件的类型
	//尝试进行配置读取
	if err = vip.ReadInConfig(); err != nil {
		panic(err)
	}

	err = vip.Unmarshal(&result)
	if err != nil {
		panic(err)
	}
	return
}
