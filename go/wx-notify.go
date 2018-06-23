package main

import (
	"encoding/json"
	"fmt"
	"github.com/wusuluren/requests"
	"github.com/wusuluren/requests/py"
	"io/ioutil"
	"os"
)

type Config struct {
	Corpid     string
	Corpsecret string
	Agentid    string
}

var g Config

func checkError(err error) {
	if err != nil {
		panic(err)
	}
}

func LoadConfig(filename string) {
	if filename == "" {
		filename = "wx-notify.json"
	}
	data, err := ioutil.ReadFile(filename)
	checkError(err)
	err = json.Unmarshal(data, &g)
	checkError(err)
	fmt.Println(g)
}

func GetToken() string {
	type TokenResp struct {
		AccessToken string `json:"access_token"`
	}
	var respResult TokenResp
	resp := requests.Get(fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", g.Corpid, g.Corpsecret), nil)
	resp.Json(&respResult)
	fmt.Println(respResult.AccessToken)
	return respResult.AccessToken
}

func SendMsg(token, msg string) {
	type MsgText struct {
		Content string `json:"content"`
	}
	type MsgReq struct {
		Touser  string  `json:"touser"`
		Msgtype string  `json:"msgtype"`
		Agentid string  `json:"agentid"`
		Text    MsgText `json:"text"`
		Safe    int     `json:"safe"`
	}
	data := MsgReq{
		Touser:  "@all",
		Msgtype: "text",
		Agentid: g.Agentid,
		Text: MsgText{
			Content: msg,
		},
		Safe: 0,
	}
	params := make(py.Dict)
	params["json"] = data
	resp := requests.Post(fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token), params).Text()
	fmt.Println(resp)
}

func main() {
	if len(os.Args) > 1 {
		LoadConfig("")
		token := GetToken()
		for _, arg := range os.Args[1:] {
			SendMsg(token, arg)
		}
	}
}
