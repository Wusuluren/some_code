package main

import (
	"context"
	"time"

	pb "time/proto"

	"go-micro.dev/v4"
	log "go-micro.dev/v4/logger"
)

var (
	service = "time"
	version = "latest"
)

func main() {
	// Create service
	srv := micro.NewService()
	srv.Init()

	// Create client
	c := pb.NewHelloworldService(service, srv.Client())

	for {
		// Call service
		rsp, err := c.Call(context.Background(), &pb.CallRequest{Name: "John"})
		if err != nil {
			log.Fatal(err)
		}

		log.Info(rsp)

		time.Sleep(1 * time.Second)
	}
}
