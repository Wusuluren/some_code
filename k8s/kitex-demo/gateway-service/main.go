package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"github.com/cloudwego/kitex/client"
	"kitex-demo/time-service/kitex_gen/pbgo/timepb/timeservice"
	"kitex-demo/time-service/kitex_gen/pbgo/timepb"
)


func main() {
	startHttpService()
}


func startHttpService() {
	router := gin.New()
	router.Use(gin.Recovery())

	router.GET("/time/getTime", routerTime)

	server := &http.Server{Addr: "0.0.0.0:8000", Handler: router}
		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}
}

func handleError(ctx *gin.Context, err error) {
	fmt.Println(err)
	ctx.String(500, err.Error())
	// ctx.AbortWithError(500, err)
}


func routerTime(ctx *gin.Context) {
	client, err := timeservice.NewClient("TimeService", client.WithHostPorts("127.0.0.1:8888"))
	if err != nil {
		handleError(ctx, err)
		return
	}

	timezone := ctx.Param("timezone")
	req := &timepb.GetTimeReq{TimeZone: timezone}
	resp, err := client.GetTime(context.Background(), req)
	if err != nil {
		handleError(ctx, err)
		return
	}
	timeStr := resp.Time.AsTime().String()
	ctx.String(200, timeStr)
}

