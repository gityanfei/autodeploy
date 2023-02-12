package tools

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"public/declare"
	"strconv"
	"time"
)

type agentConfig struct {
	Server       `ini:"server"`
	Rabbitmq     `ini:"rabbitmq"`
	DingdingConf `ini:"dingding"`
	SrcImage     ImageAddr `ini:"srcImageAddr"`
	DestImage    ImageAddr `ini:"destImageAddr"`
}
type Server struct {
	Port string `ini:"port"`
}
type Rabbitmq struct {
	User            string `ini:"user"`
	Password        string `ini:"password"`
	HostPort        string `ini:"hostPort"`
	ProductionQueue string `ini:"productionQueue"`
}
type DingdingConf struct {
	URL    string `ini:"url"`
	Secret string `ini:"secret"`
}
type ImageAddr struct {
	Host     string `json:"host" ini:"host"`
	Repo     string `json:"repo" ini:"repo"`
	Server   string `json:"server" ini:"server"`
	Tag      string `json:"tag" ini:"tag"`
	User     string `json:"user" ini:"user"`
	Password string `json:"password" ini:"password"`
}

type Image struct {
	SrcImageInfo  ImageAddr `json:"srcImageAddr"`
	DestImageInfo ImageAddr `json:"destImageAddr"`
}

func makeMessage(msg2, atAll string) string {
	templ := `{
    "at": {
        "180xxxxxx":[
            "%s"
        ],
        "atUserIds":[
            "user123"
        ],
        "isAtAll": false
    },
    "text": {
        "content":"%s"
    },
    "msgtype":"text"
}`
	toMsg := fmt.Sprintf(templ, atAll, msg2)
	//fmt.Println(msg)
	return toMsg
}

type DingdingConfig declare.DingdingConfig

func (d DingdingConfig) SamplePost(msg1 string) (respBytes []byte, err error) {
	fullMsg := makeMessage(msg1, "true")
	timestamp := time.Now().UnixMilli()
	tmp := strconv.Itoa(int(timestamp)) + "\n" + d.DingdingSign
	enMsg := GenHmacSha256(tmp, d.DingdingSign)
	reader := bytes.NewReader([]byte(fullMsg))
	UrlAddress := fmt.Sprintf("%s?access_token=%s&timestamp=%d&sign=%s", d.DingdingURL, d.DingdingAccessToken, timestamp, enMsg)
	request, err := http.NewRequest("POST", UrlAddress, reader)
	//fmt.Println(UrlAddress)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	respBytes, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	return
}
func GenHmacSha256(message string, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return Base64UrlSafeEncode(h.Sum(nil))
}
func Base64UrlSafeEncode(source []byte) string {
	byteArr := base64.StdEncoding.EncodeToString(source)
	return byteArr
}
