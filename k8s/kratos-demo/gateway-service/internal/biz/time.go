package biz

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
)

type Time struct {
	Hello string
}

type TimeRepo interface {
	GetTime(context.Context, *Time) error
}

type TimeUsecase struct {
	repo TimeRepo
	log  *log.Helper
}

func NewTimeUsecase(repo TimeRepo, logger log.Logger) *TimeUsecase {
	return &TimeUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *TimeUsecase) Get(ctx context.Context, g *Time) error {
	return uc.repo.GetTime(ctx, g)
}
