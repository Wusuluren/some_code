package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type JokeDesc struct {
	Id      string
	Content string
}

type PicDesc struct {
	File string
	Url  string
	Desc string
}

func main() {
	setLogFile := func() error {
		logFile, err := os.OpenFile("log.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
		ckErr(err)
		log.SetOutput(logFile)
		return nil
	}
	_ = setLogFile
	setLogFile()

	urlHome := "http://www.qiushibaike.com/hot"
	jokePattern := `
<div class="article block untagged mb15" id='[\S]+?'>
[\s\S]+?
<a href="/article/(\d+?)" target="_blank" class='contentHerf' >
<div class="content">
\s*?
<span>([\PN]+?)</span>
\s*?
</div>
</a>
\s*?
<div class="stats">
<span class="stats-vote"><i class="number">\d+</i> 好笑</span>
<span class="stats-comments">`
	picPattern := `
<div class="article block untagged mb15" id='[\S]+?'>
[\s\S]+?
<a href="/article/(\d+?)" target="_blank" class='contentHerf' >
<div class="content">
\s*?
<span>([\PN]+?)</span>
\s*?
</div>
</a>
\s*?
<div class="thumb">
\s*?
<a href="/article/\d+?" target="_blank">
<img src="(http://[\S]+?.jpg)" alt="[\PN]+?" />
</a>
\s*?
</div>
\s*?
<div class="stats">
<span class="stats-vote"><i class="number">\d+</i> 好笑</span>
<span class="stats-comments">`

	jokeReg := regexp.MustCompile(jokePattern)
	picReg := regexp.MustCompile(picPattern)
	_, _ = picReg, jokeReg

	date := time.Now().Format("2006-01-02")
	jokeFile, err := os.OpenFile(fmt.Sprintf("joke/joke-%s.txt", date), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	ckErr(err)
	defer jokeFile.Close()
	picFile, err := os.OpenFile(fmt.Sprintf("pic/pic-%s.txt", date), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	ckErr(err)
	defer picFile.Close()
	jokeDesc := JokeDesc{}
	picDesc := PicDesc{}
	for page := 1; page <= 1; page++ {
		url := fmt.Sprintf("%s/page/%d", urlHome, page)
		html := getPage(url)
		jokes := jokeReg.FindAllStringSubmatch(html, -1)
		if len(jokes) > 1 {
			for _, joke := range jokes {
				jokeDesc.Id = joke[1]
				jokeDesc.Content = joke[2]
				jokeDesc.Content = strings.Replace(jokeDesc.Content, "<br/>", "", -1)
				out, err := json.Marshal(jokeDesc)
				ckErr(err)
				jokeFile.WriteString(string(out) + "\n")
				//log.Println(string(out) + "\n")
			}
		}
		pics := picReg.FindAllStringSubmatch(html, -1)
		if len(pics) > 1 {
			for _, pic := range pics {
				s := strings.Split(pic[3], "/")
				fileName := s[len(s)-1]
				picDesc.File = fileName
				picDesc.Desc = pic[2]
				picDesc.Url = pic[3]
				downloadPic("pic/"+picDesc.File, picDesc.Url)
				picDesc.Desc = strings.Replace(picDesc.Desc, "<br/>", "", -1)
				out, err := json.Marshal(picDesc)
				ckErr(err)
				picFile.WriteString(string(out) + "\n")
				//log.Println(string(out) + "\n")
			}
		}
	}
}

func downloadPic(file, url string) {
	resp, err := http.Get(url)
	ckErr(err)
	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	ckErr(err)
	f, err := os.OpenFile(file, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	ckErr(err)
	defer f.Close()
	f.Write(buf)
}

func getPage(url string) string {
	resp, err := http.Get(url)
	ckErr(err)
	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	ckErr(err)
	return string(buf)
}

func ckErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
