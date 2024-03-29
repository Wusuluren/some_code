// Code generated by goctl. DO NOT EDIT!
// Source: time.proto

package timeservice

import (
	"context"

	"gozero-demo/pb/pbgo/timepb"

	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

type (
	GetTimeReq  = timepb.GetTimeReq
	GetTimeResp = timepb.GetTimeResp

	TimeService interface {
		GetTime(ctx context.Context, in *GetTimeReq, opts ...grpc.CallOption) (*GetTimeResp, error)
	}

	defaultTimeService struct {
		cli zrpc.Client
	}
)

func NewTimeService(cli zrpc.Client) TimeService {
	return &defaultTimeService{
		cli: cli,
	}
}

func (m *defaultTimeService) GetTime(ctx context.Context, in *GetTimeReq, opts ...grpc.CallOption) (*GetTimeResp, error) {
	client := timepb.NewTimeServiceClient(m.cli.Conn())
	return client.GetTime(ctx, in, opts...)
}
