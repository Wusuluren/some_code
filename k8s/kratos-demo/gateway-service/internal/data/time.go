package data

import (
	"context"
	"gateway-service/internal/biz"
	"github.com/go-kratos/kratos/v2/log"
)

type timeRepo struct {
	data *Data
	log  *log.Helper
}

// NewTimeRepo .
func NewTimeRepo(data *Data, logger log.Logger) biz.TimeRepo {
	return &timeRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *timeRepo) GetTime(ctx context.Context, g *biz.Time) error {
	return nil
}
