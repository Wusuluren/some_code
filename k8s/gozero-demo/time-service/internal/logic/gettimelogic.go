package logic

import (
	"context"

	"gozero-demo/pb/pbgo/timepb"
	"gozero-demo/time-service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
)

type GetTimeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTimeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTimeLogic {
	return &GetTimeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetTimeLogic) GetTime(in *timepb.GetTimeReq) (*timepb.GetTimeResp, error) {
	req := in
	timezone := "Local"
	if req.TimeZone != "" {
		timezone = req.TimeZone
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(loc)

	resp := &timepb.GetTimeResp{
		Time: &timestamppb.Timestamp{
			Seconds: now.Unix(),
			Nanos:   int32(now.UnixNano()),
		},
	}
	return resp, nil
}
