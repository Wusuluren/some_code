package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"microservice/pbgo/timepb"
	"microservice/pkg/config"
	"microservice/pkg/signal"
	"net/http"
)

var timeClient struct {
	client timepb.TimeServiceClient
}

var cfg config.Config

func main() {
	initConf()
	var cleanupHandlers []signal.CleanupHandler
	cleanupHandlers = append(cleanupHandlers, initTimeClient())
	cleanupHandlers = append(cleanupHandlers, startHttpService())
	signal.OnSignal(cleanupHandlers...)
}

func initConf() {
	var err error
	cfg, err = config.InitYamlConfig("all.yml")
	if err != nil {
		panic(err)
	}
}

func initTimeClient() signal.CleanupHandler {
	conn, err := grpc.Dial(cfg.TimeGrpcAddr, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	timeClient.client = timepb.NewTimeServiceClient(conn)
	return func() {
		conn.Close()
	}
}

func startHttpService() signal.CleanupHandler {
	router := gin.New()
	router.Use(gin.Recovery())

	timeRouter := router.Group("/time")
	{
		timeRouter.GET("/getTime", timeGetTime)
	}

	server := &http.Server{Addr: cfg.GateWayHttpAddr, Handler: router}
	go func() {
		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}
	}()

	return func() {
		server.Close()
	}
}

func timeGetTime(ctx *gin.Context) {
	timezone := ctx.Param("timezone")
	req := &timepb.GetTimeReq{
		TimeZone: timezone,
	}
	resp, err := timeClient.client.GetTime(context.TODO(), req)
	if err != nil {
		ctx.AbortWithError(500, err)
		return
	}
	timeStr := resp.Time.AsTime().String()
	ctx.JSON(200, timeStr)
}
