package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/wusuluren/requests"
	"github.com/wusuluren/requests/py"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
)

type Config struct {
	CorpId     string `json:"corpid"`
	CorpSecret string `json:"corpsecret"`
	AgentId    string `json:"agentid"`
}

var g Config

func die(err error) {
	if err != nil {
		panic(err)
	}
}

func LoadConfig(filename string) {
	if filename == "" {
		filename = "wx-notify.json"
	}
	data, err := ioutil.ReadFile(filename)
	die(err)
	err = json.Unmarshal(data, &g)
	die(err)
	fmt.Println(g)
}

func GetToken() string {
	type TokenResp struct {
		AccessToken string `json:"access_token"`
	}
	var respResult TokenResp
	resp := requests.Get(fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", g.CorpId, g.CorpSecret), nil)
	resp.Json(&respResult)
	fmt.Println(respResult.AccessToken)
	return respResult.AccessToken
}

func SendMsg(token, msg string) {
	type MsgText struct {
		Content string `json:"content"`
	}
	type MsgReq struct {
		ToUser  string  `json:"touser"`
		MsgType string  `json:"msgtype"`
		AgentId string  `json:"agentid"`
		Text    MsgText `json:"text"`
		Safe    int     `json:"safe"`
	}
	data := MsgReq{
		ToUser:  "@all",
		MsgType: "text",
		AgentId: g.AgentId,
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

func CreateMultiPart(token, file string) (*http.Response, error) {
	var err error
	var b bytes.Buffer
	var fw io.Writer
	w := multipart.NewWriter(&b)
	defer w.Close()
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fw, err = w.CreateFormFile("media", file)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(fw, f)
	if err != nil {
		return nil, err
	}
	fw, err = w.CreateFormField(file)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/media/upload?access_token=%s&type=file", token), &b)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	client := &http.Client{}
	return client.Do(req)
}

func GetTmpMediaIdRaw(token, file string) (string, error) {
	type MediaResp struct {
		ErrCode   int    `json:"errcode""`
		ErrMsg    string `json:"errmsg"`
		Type      string `json:"type"`
		MediaId   string `json:"media_id"`
		CreatedAt string `json:"created_at"`
	}
	var mediaResp MediaResp
	resp, err := CreateMultiPart(token, file)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	err = json.Unmarshal(raw, &mediaResp)
	if err != nil {
		return "", err
	}
	return mediaResp.MediaId, nil

}

func GetTmpMediaId(token, file string) (string, error) {
	type MediaResp struct {
		ErrCode   int    `json:"errcode""`
		ErrMsg    string `json:"errmsg"`
		Type      string `json:"type"`
		MediaId   string `json:"media_id"`
		CreatedAt string `json:"created_at"`
	}
	var mediaResp MediaResp
	params := make(py.Dict)
	params["files"] = map[string]string{
		"media": file,
	}
	requests.Post(fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/media/upload?access_token=%s&type=file", token), params).Json(&mediaResp)
	fmt.Println(mediaResp)
	return mediaResp.MediaId, nil
}

func SendFile(token, file string) {
	type MsgFile struct {
		MediaId     string `json:"media_id""`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	type FileReq struct {
		ToUser  string  `json:"touser"`
		ToParty string  `json:"toparty"`
		ToTag   string  `json:"totag"`
		MsgType string  `json:"msgtype"`
		AgentId string  `json:"agentid"`
		File    MsgFile `json:"file"`
		Safe    int     `json:"safe"`
	}
	mediaId, err := GetTmpMediaId(token, file)
	die(err)
	data := FileReq{
		ToUser:  "@all",
		MsgType: "file",
		AgentId: g.AgentId,
		File: MsgFile{
			MediaId:     mediaId,
			Title:       file,
			Description: file,
		},
		Safe: 0,
	}
	params := make(py.Dict)
	params["json"] = data
	resp := requests.Post(fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token), params).Text()
	fmt.Println(resp)
}

func main() {
	fileFlag := flag.Bool("f", false, "send file")
	flag.Parse()
	LoadConfig("")
	token := GetToken()
	if flag.NArg() > 0 {
		for _, arg := range flag.Args() {
			if *fileFlag {
				SendFile(token, arg)
			} else {
				SendMsg(token, arg)
			}
		}
	} else {
		var input string
		var typo int
		fmt.Println("input as following: %d[0=msg, 1=file, other=quit], %s")
		for {
			_, err := fmt.Scanf("%d, %s\n", &typo, &input)
			if err != nil {
				fmt.Println(err)
				continue
			}
			if typo == 0 {
				SendMsg(token, input)
			} else if typo == 1 {
				SendFile(token, input)
			} else {
				break
			}
		}
	}
}
