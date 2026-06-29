package admin

import (
	"context"
	"strings"

	"github.com/csw/console/services/admin-api/internal/app/dto"
	domainadmin "github.com/csw/console/services/admin-api/internal/domain/admin"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// AdminAuthService 登录/刷新/登出/me/权限解析编排（compact 应用服务）。
type AdminAuthService struct {
	tx     TxManager
	hasher PasswordHasher
	issuer TokenIssuer
	feishu FeishuClient
	cipher Cipher
	audit  AuditSink
	env    common.Environment
}

// AuthDeps AdminAuthService 依赖。
type AuthDeps struct {
	Tx     TxManager
	Hasher PasswordHasher
	Issuer TokenIssuer
	Feishu FeishuClient
	Cipher Cipher
	Audit  AuditSink
	Env    common.Environment
}

// NewAdminAuthService 构造鉴权服务。
func NewAdminAuthService(d AuthDeps) *AdminAuthService {
	return &AdminAuthService{
		tx: d.Tx, hasher: d.Hasher, issuer: d.Issuer,
		feishu: d.Feishu, cipher: d.Cipher, audit: d.Audit, env: d.Env,
	}
}

// Login 密码登录（compact「密码登录」算法）。
func (s *AdminAuthService) Login(ctx context.Context, cmd dto.LoginCmd) (dto.LoginResult, error) {
	if strings.TrimSpace(cmd.UserName) == "" || cmd.Password == "" {
		return dto.LoginResult{}, ErrValidation
	}
	repos := s.tx.Repositories()

	user, err := repos.Users.FindByUserName(ctx, cmd.UserName)
	if err != nil {
		if err == ErrNotFound {
			return dto.LoginResult{}, ErrUnauthenticated // 不区分用户不存在/密码错，防枚举
		}
		return dto.LoginResult{}, err
	}
	if !user.IsActive() {
		return dto.LoginResult{}, ErrUnauthenticated
	}

	identity, err := repos.Identities.FindByTypeKey(ctx, string(common.IdentityTypePassword), user.UserName)
	if err != nil {
		if err == ErrNotFound {
			return dto.LoginResult{}, ErrUnauthenticated
		}
		return dto.LoginResult{}, err
	}
	if err := s.hasher.Compare(identity.CredentialCiphertext, cmd.Password); err != nil {
		return dto.LoginResult{}, ErrUnauthenticated
	}

	pair, view, err := s.issueForUser(ctx, repos, user)
	if err != nil {
		return dto.LoginResult{}, err
	}
	s.writeAudit(ctx, user.ID, "admin.login", "admin_user", user.ID, map[string]any{"identityType": "password"})
	return dto.LoginResult{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresAt: pair.ExpiresAt, User: view}, nil
}

// FeishuCallback 飞书回调登录（compact「飞书登录回调」算法）。
func (s *AdminAuthService) FeishuCallback(ctx context.Context, cmd dto.FeishuCallbackCmd) (dto.LoginResult, error) {
	if strings.TrimSpace(cmd.Code) == "" {
		return dto.LoginResult{}, ErrValidation
	}
	if s.feishu == nil {
		return dto.LoginResult{}, ErrUnauthenticated
	}
	info, err := s.feishu.ExchangeCode(ctx, cmd.Code, cmd.RedirectURI)
	if err != nil {
		return dto.LoginResult{}, err // 飞书不可用 -> INTERNAL（handler 据非哨兵错误映射）
	}
	if info.UnionID == "" {
		return dto.LoginResult{}, ErrUnauthenticated
	}

	repos := s.tx.Repositories()
	identity, err := repos.Identities.FindByTypeKey(ctx, string(common.IdentityTypeFeishu), info.UnionID)
	if err != nil {
		if err == ErrNotFound {
			return dto.LoginResult{}, ErrUnauthenticated // 未绑定不自动开户
		}
		return dto.LoginResult{}, err
	}
	user, err := repos.Users.FindByID(ctx, identity.UserIDRef)
	if err != nil {
		return dto.LoginResult{}, err
	}
	if !user.IsActive() {
		return dto.LoginResult{}, ErrUnauthenticated
	}

	pair, view, err := s.issueForUser(ctx, repos, user)
	if err != nil {
		return dto.LoginResult{}, err
	}
	s.writeAudit(ctx, user.ID, "admin.login", "admin_user", user.ID, map[string]any{"identityType": "feishu"})
	return dto.LoginResult{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresAt: pair.ExpiresAt, User: view}, nil
}

// Refresh 刷新 access（校验 refresh -> 回库校验 status -> 重新解析权限签发）。
func (s *AdminAuthService) Refresh(ctx context.Context, cmd dto.RefreshCmd) (dto.TokenPairView, error) {
	if strings.TrimSpace(cmd.RefreshToken) == "" {
		return dto.TokenPairView{}, ErrValidation
	}
	claims, err := s.issuer.Parse(cmd.RefreshToken, domainauth.TokenTypeRefresh)
	if err != nil {
		return dto.TokenPairView{}, ErrUnauthenticated
	}
	userID, err := parseInt64(claims.Subject)
	if err != nil {
		return dto.TokenPairView{}, ErrUnauthenticated
	}

	repos := s.tx.Repositories()
	user, err := repos.Users.FindByID(ctx, userID)
	if err != nil {
		if err == ErrNotFound {
			return dto.TokenPairView{}, ErrUnauthenticated
		}
		return dto.TokenPairView{}, err
	}
	if !user.IsActive() { // 禁用即拒绝（compact 不变量 5）
		return dto.TokenPairView{}, ErrUnauthenticated
	}

	pair, _, err := s.issueForUser(ctx, repos, user)
	if err != nil {
		return dto.TokenPairView{}, err
	}
	return dto.TokenPairView{AccessToken: pair.AccessToken, RefreshToken: pair.RefreshToken, ExpiresAt: pair.ExpiresAt}, nil
}

// Logout 登出（无状态 JWT：客户端丢弃；写审计）。
func (s *AdminAuthService) Logout(ctx context.Context, actorID int64, cmd dto.LogoutCmd) error {
	s.writeAudit(ctx, actorID, "admin.logout", "admin_user", actorID, nil)
	return nil
}

// Me 当前用户信息（compact GET /me）。
func (s *AdminAuthService) Me(ctx context.Context, userID int64) (dto.MeView, error) {
	repos := s.tx.Repositories()
	user, err := repos.Users.FindByID(ctx, userID)
	if err != nil {
		return dto.MeView{}, err
	}
	roles, err := repos.Users.RolesByUser(ctx, userID)
	if err != nil {
		return dto.MeView{}, err
	}
	perms, err := repos.Permissions.ListCodesByUser(ctx, userID)
	if err != nil {
		return dto.MeView{}, err
	}
	identities, err := repos.Identities.ListByUser(ctx, userID)
	if err != nil {
		return dto.MeView{}, err
	}

	return dto.MeView{
		UserID:      user.ID,
		UserName:    user.UserName,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Status:      string(user.Status),
		Roles:       roleCodes(roles),
		Permissions: emptyIfNil(perms),
		Identities:  maskIdentities(identities),
		Environment: string(s.env),
	}, nil
}

// LoadAuthContext 权限解析为鉴权上下文（compact loadAuthContext）。
func (s *AdminAuthService) LoadAuthContext(ctx context.Context, userID int64) (domainauth.AuthContext, error) {
	repos := s.tx.Repositories()
	user, err := repos.Users.FindByID(ctx, userID)
	if err != nil {
		return domainauth.AuthContext{}, err
	}
	roles, err := repos.Users.RolesByUser(ctx, userID)
	if err != nil {
		return domainauth.AuthContext{}, err
	}
	perms, err := repos.Permissions.ListCodesByUser(ctx, userID)
	if err != nil {
		return domainauth.AuthContext{}, err
	}
	return domainauth.NewAuthContext(user.ID, user.UserName, user.DisplayName, roleCodes(roles), perms, s.env), nil
}

// issueForUser 解析权限并签发令牌，返回令牌对与登录响应用户摘要。
func (s *AdminAuthService) issueForUser(ctx context.Context, repos Repositories, user *domainadmin.AdminUser) (domainauth.TokenPair, dto.UserView, error) {
	roles, err := repos.Users.RolesByUser(ctx, user.ID)
	if err != nil {
		return domainauth.TokenPair{}, dto.UserView{}, err
	}
	perms, err := repos.Permissions.ListCodesByUser(ctx, user.ID)
	if err != nil {
		return domainauth.TokenPair{}, dto.UserView{}, err
	}
	ac := domainauth.NewAuthContext(user.ID, user.UserName, user.DisplayName, roleCodes(roles), perms, s.env)
	pair, err := s.issuer.IssuePair(ac)
	if err != nil {
		return domainauth.TokenPair{}, dto.UserView{}, err
	}
	view := dto.UserView{
		UserID:      user.ID,
		UserName:    user.UserName,
		DisplayName: user.DisplayName,
		Roles:       roleCodes(roles),
		Permissions: emptyIfNil(perms),
	}
	return pair, view, nil
}

func (s *AdminAuthService) writeAudit(ctx context.Context, actorID int64, action, resourceType string, resourceID int64, detail map[string]any) {
	if s.audit == nil {
		return
	}
	s.audit.Write(ctx, AuditEntry{
		ActorID:      actorID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   int64ToStr(resourceID),
		Detail:       detail,
	})
}
