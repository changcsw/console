// Package admin 是后台鉴权/RBAC 的应用层：编排、事务、JWT/bcrypt、唯一性校验、审计调用。
// command/query 方法分离；DTO 见 internal/app/dto。仓储端口在此定义，由 infra 实现（依赖方向向内）。
package admin

import (
	"context"
	"errors"
	"time"

	domainadmin "github.com/csw/console/services/admin-api/internal/domain/admin"
	domainauth "github.com/csw/console/services/admin-api/internal/domain/auth"
)

// 应用层错误哨兵，handler 据此映射全局错误码（00 §7.4）。
var (
	ErrNotFound        = errors.New("not found")         // -> NOT_FOUND
	ErrConflict        = errors.New("conflict")          // -> CONFLICT
	ErrUnauthenticated = errors.New("unauthenticated")   // -> UNAUTHENTICATED
	ErrValidation      = errors.New("validation failed") // -> VALIDATION_FAILED
)

// AdminUserRepository 窄仓储：单聚合 CRUD + 必要查询。
type AdminUserRepository interface {
	Create(ctx context.Context, u *domainadmin.AdminUser) error
	Update(ctx context.Context, u *domainadmin.AdminUser) error
	FindByID(ctx context.Context, id int64) (*domainadmin.AdminUser, error)
	FindByUserName(ctx context.Context, userName string) (*domainadmin.AdminUser, error)
	List(ctx context.Context, filter domainadmin.AdminUserFilter) ([]domainadmin.AdminUser, int, error)
	ReplaceRoles(ctx context.Context, userID int64, roleIDs []int64) error
	// RolesByUser 返回用户已分配角色（含 role_code/role_name）。
	RolesByUser(ctx context.Context, userID int64) ([]domainadmin.Role, error)
}

// AdminIdentityRepository 身份仓储。
type AdminIdentityRepository interface {
	FindByTypeKey(ctx context.Context, t string, key string) (*domainadmin.AdminIdentity, error)
	ListByUser(ctx context.Context, userID int64) ([]domainadmin.AdminIdentity, error)
	Upsert(ctx context.Context, identity *domainadmin.AdminIdentity) error
}

// RoleRepository 角色仓储。
type RoleRepository interface {
	Create(ctx context.Context, r *domainadmin.Role) error
	Update(ctx context.Context, r *domainadmin.Role) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*domainadmin.Role, error)
	List(ctx context.Context, filter domainadmin.RoleFilter) ([]domainadmin.Role, int, error)
	ReplacePermissions(ctx context.Context, roleID int64, permIDs []int64) error
	// CountUsers 统计引用该角色的用户数（删除前校验）。
	CountUsers(ctx context.Context, roleID int64) (int, error)
	// PermissionsByRole 返回角色已配置权限。
	PermissionsByRole(ctx context.Context, roleID int64) ([]domainadmin.Permission, error)
}

// PermissionRepository 权限码仓储。
type PermissionRepository interface {
	Create(ctx context.Context, p *domainadmin.Permission) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*domainadmin.Permission, error)
	List(ctx context.Context, filter domainadmin.PermissionFilter) ([]domainadmin.Permission, int, error)
	FindByIDs(ctx context.Context, ids []int64) ([]domainadmin.Permission, error)
	// ListCodesByUser 权限解析：用户所有角色权限码并集（去重）。
	ListCodesByUser(ctx context.Context, userID int64) ([]string, error)
	// CountRoles 统计引用该权限的角色数（删除前校验）。
	CountRoles(ctx context.Context, permID int64) (int, error)
}

// Repositories 一组仓储句柄（绑定到 pool 或某个事务连接）。
type Repositories struct {
	Users       AdminUserRepository
	Identities  AdminIdentityRepository
	Roles       RoleRepository
	Permissions PermissionRepository
}

// TxManager 提供事务边界，跨聚合写编排在 app 层用 InTx 包裹。
type TxManager interface {
	Repositories() Repositories
	InTx(ctx context.Context, fn func(Repositories) error) error
}

// AuditSink 审计写入端口（本模块只调用，不实现存储，见 audit 模块）。
type AuditSink interface {
	Write(ctx context.Context, entry AuditEntry)
}

// AuditEntry 审计记录（00 §8 字段；detail 已脱敏）。
type AuditEntry struct {
	ActorID      int64
	Action       string
	ResourceType string
	ResourceID   string
	Detail       map[string]any
}

// PasswordHasher bcrypt 哈希端口（infra/crypto 实现）。
type PasswordHasher interface {
	Hash(plain string) (string, error)
	Compare(hash, plain string) error
}

// TokenIssuer JWT 签发/校验端口（infra/jwt 实现）。
type TokenIssuer interface {
	IssuePair(ac domainauth.AuthContext) (domainauth.TokenPair, error)
	Parse(token, expectedType string) (domainauth.Claims, error)
	AccessExpiry() time.Time
}

// FeishuUser 飞书回调换得的用户信息。
type FeishuUser struct {
	UnionID string
	OpenID  string
	Name    string
	Email   string
}

// FeishuClient 飞书 OAuth 端口（infra/feishu 经适配实现；mock 仅 develop）。
type FeishuClient interface {
	ExchangeCode(ctx context.Context, code, redirectURI string) (FeishuUser, error)
}

// Cipher 飞书令牌 AES-GCM 加解密端口（infra/crypto 实现）；可为 nil（不加密，仅 develop 容忍）。
type Cipher interface {
	Encrypt(plain string) (string, error)
	Decrypt(encoded string) (string, error)
}
