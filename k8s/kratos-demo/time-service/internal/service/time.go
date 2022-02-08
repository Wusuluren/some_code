package service

import (
	"context"

	pb "time-service/api/time"
	"time"
)

type TimeService struct {
	pb.UnimplementedTimeServer
}

func NewTimeService() *TimeService {
	return &TimeService{}
}

func (s *TimeService) GetTime(ctx context.Context, req *pb.GetTimeRequest) (*pb.GetTimeReply, error) {
	resp:=&pb.GetTimeReply{}

	timezone := "Local"
	if req.Timezone != "" {
		timezone = req.Timezone
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return resp, err
	}
	now := time.Now().In(loc)

	resp = &pb.GetTimeReply{
		Message: now.String(),
	}

	return resp, nil
}
