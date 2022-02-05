package main

import (
	"context"
	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/timestamppb"
	"microservice/pbgo/timepb"
	"microservice/pkg/config"
	"microservice/pkg/signal"
	"net"
	"time"
)

var (
	cfg          config.Config
	zapLogger, _ = zap.NewProduction()
)

func main() {
	initConf()
	var cleanupHandlers []signal.CleanupHandler
	cleanupHandlers = append(cleanupHandlers, startGrpcService())
	signal.OnSignal(cleanupHandlers...)
}

func initConf() {
	var err error
	cfg, err = config.InitYamlConfig("/conf/all.yml")
	if err != nil {
		panic(err)
	}
}

func startGrpcService() signal.CleanupHandler {
	lis, err := net.Listen("tcp", cfg.TimeGrpcAddr)
	if err != nil {
		panic(err)
	}

	s := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_zap.UnaryServerInterceptor(zapLogger),
			grpc_recovery.UnaryServerInterceptor(),
		)),
	)

	timepb.RegisterTimeServiceServer(s, NewTimeServer())
	reflection.Register(s)

	go func() {
		err = s.Serve(lis)
		if err != nil {
			panic(err)
		}
	}()

	return func() {
		s.GracefulStop()
	}
}

func NewTimeServer() *TimeService {
	return &TimeService{}
}

type TimeService struct{}

func (t TimeService) GetTime(ctx context.Context, req *timepb.GetTimeReq) (*timepb.GetTimeResp, error) {
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
