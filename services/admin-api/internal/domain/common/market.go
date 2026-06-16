package common

type Market string

const (
	MarketGlobal Market = "GLOBAL"
	MarketJP     Market = "JP"
	MarketKR     Market = "KR"
	MarketSEA    Market = "SEA"
	MarketHMT    Market = "HMT"
	MarketCN     Market = "CN"
)

func (m Market) IsCN() bool {
	return m == MarketCN
}

func (m Market) UsesGlobalFallback() bool {
	return m == MarketJP || m == MarketKR || m == MarketSEA || m == MarketHMT
}
