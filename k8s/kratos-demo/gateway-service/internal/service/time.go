package service

import (
	"context"

	pb "gateway-service/api/time"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	transgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
)

type TimeService struct {
	pb.UnimplementedTimeServer
}

func NewTimeService() *TimeService {
	return &TimeService{}
}

func (s *TimeService) GetTime(ctx context.Context, req *pb.GetTimeRequest) (*pb.GetTimeReply, error) {
	resp := &pb.GetTimeReply{}

	conn, err := transgrpc.DialInsecure(
		context.Background(),
		transgrpc.WithEndpoint("127.0.0.1:9000"),
		transgrpc.WithMiddleware(
			recovery.Recovery(),
		),
	)
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	client := pb.NewTimeClient(conn)
	reply, err := client.GetTime(context.Background(), &pb.GetTimeRequest{Timezone: req.Timezone})
	if err != nil {
		return resp, err
	}

	resp.Message = reply.Message
	return resp, nil
}
