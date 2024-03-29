// Code generated by Kitex v0.1.4. DO NOT EDIT.

package timeservice

import (
	"context"
	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/client/callopt"
	"kitex-demo/time-service/kitex_gen/pbgo/timepb"
)

// Client is designed to provide IDL-compatible methods with call-option parameter for kitex framework.
type Client interface {
	GetTime(ctx context.Context, Req *timepb.GetTimeReq, callOptions ...callopt.Option) (r *timepb.GetTimeResp, err error)
}

// NewClient creates a client for the service defined in IDL.
func NewClient(destService string, opts ...client.Option) (Client, error) {
	var options []client.Option
	options = append(options, client.WithDestService(destService))

	options = append(options, opts...)

	kc, err := client.NewClient(serviceInfo(), options...)
	if err != nil {
		return nil, err
	}
	return &kTimeServiceClient{
		kClient: newServiceClient(kc),
	}, nil
}

// MustNewClient creates a client for the service defined in IDL. It panics if any error occurs.
func MustNewClient(destService string, opts ...client.Option) Client {
	kc, err := NewClient(destService, opts...)
	if err != nil {
		panic(err)
	}
	return kc
}

type kTimeServiceClient struct {
	*kClient
}

func (p *kTimeServiceClient) GetTime(ctx context.Context, Req *timepb.GetTimeReq, callOptions ...callopt.Option) (r *timepb.GetTimeResp, err error) {
	ctx = client.NewCtxWithCallOptions(ctx, callOptions)
	return p.kClient.GetTime(ctx, Req)
}
