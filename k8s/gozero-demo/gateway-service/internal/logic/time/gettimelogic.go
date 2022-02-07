package time

import (
	"context"

	"gozero-demo/gateway-service/internal/svc"
	"gozero-demo/gateway-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"gozero-demo/time-service/timeservice"
)

type GetTimeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetTimeLogic(ctx context.Context, svcCtx *svc.ServiceContext) GetTimeLogic {
	return GetTimeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetTimeLogic) GetTime(req types.GetTimeReq) (resp string, err error) {
	cli, err := zrpc.NewClientWithTarget("127.0.0.1:8080")
	if err != nil {
		panic(err)
	}
	srv := timeservice.NewTimeService(cli)
	in := &timeservice.GetTimeReq{
		TimeZone: req.TimeZone,
	}
	out, err := srv.GetTime(context.TODO(), in)
	if err != nil {
		panic(err)
	}
	return out.Time.AsTime().String(), nil
}
