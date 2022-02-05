package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/contrib/commonlog"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
	"google.golang.org/grpc"
	"io/ioutil"
	"microservice/pbgo/timepb"
	"microservice/pkg/config"
	"microservice/pkg/signal"
	"net/http"
	"time"
)

var (
	cfg        config.Config
	redisCli   *redis.ClusterClient
	timeClient struct {
		client timepb.TimeServiceClient
	}
)

func main() {
	initConf()
	initRedisClient(cfg.RedisAddrs)
	var cleanupHandlers []signal.CleanupHandler
	cleanupHandlers = append(cleanupHandlers, initTimeClient())
	cleanupHandlers = append(cleanupHandlers, startHttpService())
	signal.OnSignal(cleanupHandlers...)
}

func initConf() {
	var err error
	cfg, err = config.InitYamlConfig("/conf/all.yml")
	if err != nil {
		fmt.Println(err.Error())
		time.Sleep(time.Second * 30)
		panic(err)
	}
	fmt.Printf("conf:%+v\n", cfg)
}

func initRedisClient(addrs []string) {
	redisCli = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: addrs,
	})
}

func initTimeClient() signal.CleanupHandler {
	conn, err := grpc.Dial(cfg.TimeSvcAddr, grpc.WithInsecure())
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
	router.Use(commonlog.New())
	router.Use(NewCounter())

	router.GET("/ping", routerPing)
	router.GET("/time", routerTime)
	router.GET("/ip", routerIP)

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

func handleError(ctx *gin.Context, err error) {
	fmt.Println(err)
	ctx.String(500, err.Error())
	// ctx.AbortWithError(500, err)
}

func NewCounter() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		result, err := redisCli.Incr("counter").Result()
		fmt.Println(result, err)
	}
}

func routerPing(ctx *gin.Context) {
	ctx.String(200, "pong")
}

func routerTime(ctx *gin.Context) {
	timezone := ctx.Param("zone")
	req := &timepb.GetTimeReq{
		TimeZone: timezone,
	}
	resp, err := timeClient.client.GetTime(context.TODO(), req)
	if err != nil {
		handleError(ctx, err)
		return
	}
	timeStr := resp.Time.AsTime().String()
	ctx.String(200, timeStr)
}

func routerIP(ctx *gin.Context) {
	resp, err := http.Get(cfg.IPSvcAddr + "/ip")
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
