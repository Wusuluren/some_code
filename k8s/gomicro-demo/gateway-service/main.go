package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	pb "gomicro-demo/gateway-service/proto"
	"go-micro.dev/v4"
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
	timezone := ctx.Param("timezone")

	var (
		service = "time-service"
		// version = "latest"
	)

	// Create service
	srv := micro.NewService()
	srv.Init()

	// Create client
	c := pb.NewTimeService(service, srv.Client())

	// Call service
	rsp, err := c.GetTime(context.Background(), &pb.GetTimeRequest{Timezone: timezone})
	if err != nil {
		fmt.Println(err)
		handleError(ctx,err)
		return
	}
	fmt.Println(rsp)


	timeStr := rsp.Message
	ctx.String(200, timeStr)
}

