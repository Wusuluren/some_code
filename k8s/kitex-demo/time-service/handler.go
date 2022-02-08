package main

import (
	"context"
	"kitex-demo/time-service/kitex_gen/google.golang.org/protobuf/types/known/timestamppb"
	"kitex-demo/time-service/kitex_gen/pbgo/timepb"
	"time"
)

// TimeServiceImpl implements the last service interface defined in the IDL.
type TimeServiceImpl struct{}

// GetTime implements the TimeServiceImpl interface.
func (s *TimeServiceImpl) GetTime(ctx context.Context, req *timepb.GetTimeReq) (resp *timepb.GetTimeResp, err error) {
	// TODO: Your code here...
	resp = &timepb.GetTimeResp{}

	timezone := "Local"
	if req.TimeZone != "" {
		timezone = req.TimeZone
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return resp, err
	}
	now := time.Now().In(loc)

	resp = &timepb.GetTimeResp{
		Time: &timestamppb.Timestamp{
			Seconds: now.Unix(),
			Nanos:   int32(now.UnixNano()),
		},
	}

	return
}
