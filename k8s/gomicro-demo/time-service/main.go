package main

import (
	"time-service/handler"
	pb "time-service/proto"

	"go-micro.dev/v4"
	log "go-micro.dev/v4/logger"
)

var (
	service = "time-service"
	version = "latest"
)

func main() {
	// Create service
	srv := micro.NewService(
		micro.Name(service),
		micro.Version(version),
	)
	srv.Init()

	// Register handler
	pb.RegisterTimeServiceHandler(srv.Server(), new(handler.TimeService))

	// Run service
	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}
