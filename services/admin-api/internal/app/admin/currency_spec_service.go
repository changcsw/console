package admin

import (
	"context"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// CurrencySpecReader 平台级 currency_specs 只读端口（infra 实现，依赖方向向内）。
// 仅暴露列举已启用规格，供字典下拉/金额预览使用（00 §5.1）。
type CurrencySpecReader interface {
	ListEnabled(ctx context.Context) ([]common.CurrencySpec, error)
}

// CurrencySpecService 平台币种字典只读用例（仅读 platform.currency_specs，不写）。
type CurrencySpecService struct {
	reader CurrencySpecReader
}

// NewCurrencySpecService 构造服务。
func NewCurrencySpecService(reader CurrencySpecReader) *CurrencySpecService {
	return &CurrencySpecService{reader: reader}
}

// ListCurrencySpecs 列出已启用的币种规格（按 currency_code 稳定排序，由仓储保证）。
func (s *CurrencySpecService) ListCurrencySpecs(ctx context.Context) ([]dto.CurrencySpecView, error) {
	specs, err := s.reader.ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]dto.CurrencySpecView, 0, len(specs))
	for i := range specs {
		items = append(items, dto.CurrencySpecView{
			CurrencyCode:   specs[i].CurrencyCode,
			CurrencyName:   specs[i].CurrencyName,
			DecimalPlaces:  specs[i].DecimalPlaces,
			MinAmountMinor: specs[i].MinAmountMinor,
			RoundingMode:   specs[i].RoundingMode,
			Enabled:        specs[i].Enabled,
		})
	}
	return items, nil
}
