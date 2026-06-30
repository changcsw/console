package file

import (
	"errors"
	"strings"
)

// LocalRefService 是文件引用服务的最小实现：仅做非空清洗，保留 storage key。
type LocalRefService struct{}

func NewLocalRefService() *LocalRefService { return &LocalRefService{} }

func (s *LocalRefService) NormalizeReference(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New("empty file reference")
	}
	return trimmed, nil
}
