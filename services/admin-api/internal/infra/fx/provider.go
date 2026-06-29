package fx

import (
	"context"
	"time"
)

// Provider 抽象汇率来源，模块内不限定具体实现。
type Provider interface {
	Rates(ctx context.Context, baseDate time.Time) (map[string]float64, error)
}

// NoopProvider 用于当前阶段的占位实现。
type NoopProvider struct{}

func (NoopProvider) Rates(context.Context, time.Time) (map[string]float64, error) {
	return map[string]float64{}, nil
}
