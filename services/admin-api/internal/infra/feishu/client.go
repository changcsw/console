// Package feishu 封装飞书 OAuth：用 code 换用户信息。含 mock（仅 develop 生效）。
package feishu

import (
	"context"
	"errors"
	"strings"
)

// UserInfo 飞书回调换得的用户信息（compact「飞书登录回调」）。
type UserInfo struct {
	UnionID string
	OpenID  string
	Name    string
	Email   string
}

// Client 飞书客户端接口；transport/app 仅依赖此抽象。
type Client interface {
	// ExchangeCode 用授权 code 换取用户信息（含 union_id）。
	ExchangeCode(ctx context.Context, code, redirectURI string) (UserInfo, error)
}

// ErrUnavailable 飞书服务不可用（映射 INTERNAL）。
var ErrUnavailable = errors.New("feishu service unavailable")

// MockClient 仅 develop 生效：接受 mock:<userName> 形式 code，直接映射 identity_key。
type MockClient struct{}

// NewMockClient 构造 mock 客户端。
func NewMockClient() MockClient { return MockClient{} }

// ExchangeCode mock 实现：code=mock:<key> 时返回 union_id=<key>。
func (MockClient) ExchangeCode(_ context.Context, code, _ string) (UserInfo, error) {
	const prefix = "mock:"
	if !strings.HasPrefix(code, prefix) {
		return UserInfo{}, errors.New("mock client only accepts mock:<userName> code")
	}
	key := strings.TrimPrefix(code, prefix)
	if key == "" {
		return UserInfo{}, errors.New("empty mock identity key")
	}
	return UserInfo{UnionID: key, OpenID: key, Name: key, Email: ""}, nil
}

// HTTPClient 真实飞书客户端占位：本期未接入真实 OpenAPI，返回不可用。
// 真实实现应：code -> app_access_token -> user_access_token -> 用户信息（union_id/open_id/name/email）。
type HTTPClient struct {
	AppID       string
	AppSecret   string
	RedirectURI string
}

// NewHTTPClient 构造真实客户端（占位）。
func NewHTTPClient(appID, appSecret, redirectURI string) *HTTPClient {
	return &HTTPClient{AppID: appID, AppSecret: appSecret, RedirectURI: redirectURI}
}

// ExchangeCode 真实换取（占位未实现）。
func (c *HTTPClient) ExchangeCode(_ context.Context, _, _ string) (UserInfo, error) {
	return UserInfo{}, ErrUnavailable
}
