package handler

import (
	"context"
	log "go-micro.dev/v4/logger"
	"time"
	pb "time-service/proto"
)

type TimeService struct{}

func (e *TimeService) GetTime(ctx context.Context, req *pb.GetTimeRequest, rsp *pb.GetTimeReply) error {
	log.Infof("Received TimeService.Call request: %v", req)

	timezone := "Local"
	if req.Timezone != "" {
		timezone = req.Timezone
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return err
	}
	now := time.Now().In(loc)
	rsp.Message = now.String()

	return nil
}
