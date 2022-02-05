package main

import (
	"fmt"
	"github.com/gin-gonic/contrib/commonlog"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"microservice/pkg/config"
	"microservice/pkg/signal"
	"net/http"
)

var cfg config.Config

func main() {
	initConf()
	var cleanupHandlers []signal.CleanupHandler
	cleanupHandlers = append(cleanupHandlers, startHttpService())
	signal.OnSignal(cleanupHandlers...)
}

func initConf() {
	var err error
	cfg, err = config.InitYamlConfig("/conf/all.yml")
	if err != nil {
		panic(err)
	}
}

func startHttpService() signal.CleanupHandler {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(commonlog.New())

	router.GET("/ip", routerIP)

	server := &http.Server{Addr: cfg.IPHttpAddr, Handler: router}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}
	}()

	return func() {
		server.Close()
	}
}

func handleError(ctx *gin.Context, err error) {
	fmt.Println(err)
	ctx.String(500, err.Error())
	// ctx.AbortWithError(500, err)
}

func routerIP(ctx *gin.Context) {
	req, err := http.NewRequest(http.MethodGet, "http://ip.sb", nil)
	if err != nil {
		handleError(ctx, err)
		return
	}
	req.Header.Add("User-Agent", "curl/7.58.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		handleError(ctx, err)
		return
	}
	defer resp.Body.Close()
	blob, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		handleError(ctx, err)
		return
	}
	ctx.String(200, string(blob))
}
