package admin

import (
	"context"
	"sort"
	"strings"
	"time"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	domainadmin "github.com/csw/console/services/admin-api/internal/domain/admin"
	"github.com/csw/console/services/admin-api/internal/domain/common"
)

// memState 是 auth 模块的内存数据快照，仅用于进程内 httptest 全链路覆盖
// （transport -> app -> domain），不依赖真实 PG。InTx 通过克隆/回填实现真实回滚语义。
type memState struct {
	users      map[int64]*domainadmin.AdminUser
	identities map[int64]*domainadmin.AdminIdentity
	roles      map[int64]*domainadmin.Role
	perms      map[int64]*domainadmin.Permission
	userRoles  map[int64][]int64 // userID -> roleIDs
	rolePerms  map[int64][]int64 // roleID -> permIDs

	seqUser, seqIdent, seqRole, seqPerm int64
}

func newMemState() *memState {
	return &memState{
		users:      map[int64]*domainadmin.AdminUser{},
		identities: map[int64]*domainadmin.AdminIdentity{},
		roles:      map[int64]*domainadmin.Role{},
		perms:      map[int64]*domainadmin.Permission{},
		userRoles:  map[int64][]int64{},
		rolePerms:  map[int64][]int64{},
	}
}

func (s *memState) clone() *memState {
	c := newMemState()
	for k, v := range s.users {
		cp := *v
		c.users[k] = &cp
	}
	for k, v := range s.identities {
		cp := *v
		c.identities[k] = &cp
	}
	for k, v := range s.roles {
		cp := *v
		c.roles[k] = &cp
	}
	for k, v := range s.perms {
		cp := *v
		c.perms[k] = &cp
	}
	for k, v := range s.userRoles {
		c.userRoles[k] = append([]int64(nil), v...)
	}
	for k, v := range s.rolePerms {
		c.rolePerms[k] = append([]int64(nil), v...)
	}
	c.seqUser, c.seqIdent, c.seqRole, c.seqPerm = s.seqUser, s.seqIdent, s.seqRole, s.seqPerm
	return c
}

// memStore 实现 adminapp.TxManager。
type memStore struct{ state *memState }

func newMemStore() *memStore { return &memStore{state: newMemState()} }

func reposFor(st *memState) adminapp.Repositories {
	return adminapp.Repositories{
		Users:       &memUserRepo{st},
		Identities:  &memIdentityRepo{st},
		Roles:       &memRoleRepo{st},
		Permissions: &memPermRepo{st},
	}
}

func (s *memStore) Repositories() adminapp.Repositories { return reposFor(s.state) }

func (s *memStore) InTx(ctx context.Context, fn func(adminapp.Repositories) error) error {
	clone := s.state.clone()
	if err := fn(reposFor(clone)); err != nil {
		return err // 丢弃 clone = 回滚
	}
	s.state = clone
	return nil
}

// ===== users =====

type memUserRepo struct{ st *memState }

func (r *memUserRepo) Create(_ context.Context, u *domainadmin.AdminUser) error {
	for _, ex := range r.st.users {
		if ex.UserName == u.UserName {
			return adminapp.ErrConflict
		}
	}
	r.st.seqUser++
	u.ID = r.st.seqUser
	now := time.Now()
	u.CreatedAt, u.UpdatedAt = now, now
	cp := *u
	r.st.users[u.ID] = &cp
	return nil
}

func (r *memUserRepo) Update(_ context.Context, u *domainadmin.AdminUser) error {
	if _, ok := r.st.users[u.ID]; !ok {
		return adminapp.ErrNotFound
	}
	u.UpdatedAt = time.Now()
	cp := *u
	r.st.users[u.ID] = &cp
	return nil
}

func (r *memUserRepo) FindByID(_ context.Context, id int64) (*domainadmin.AdminUser, error) {
	u, ok := r.st.users[id]
	if !ok {
		return nil, adminapp.ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (r *memUserRepo) FindByUserName(_ context.Context, name string) (*domainadmin.AdminUser, error) {
	for _, u := range r.st.users {
		if u.UserName == name {
			cp := *u
			return &cp, nil
		}
	}
	return nil, adminapp.ErrNotFound
}

func (r *memUserRepo) List(_ context.Context, f domainadmin.AdminUserFilter) ([]domainadmin.AdminUser, int, error) {
	all := make([]domainadmin.AdminUser, 0, len(r.st.users))
	for _, u := range r.st.users {
		if f.Keyword != "" {
			kw := strings.ToLower(f.Keyword)
			if !strings.Contains(strings.ToLower(u.UserName), kw) &&
				!strings.Contains(strings.ToLower(u.DisplayName), kw) &&
				!strings.Contains(strings.ToLower(u.Email), kw) {
				continue
			}
		}
		if f.Status != "" && u.Status != f.Status {
			continue
		}
		all = append(all, *u)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	total := len(all)
	page, pageSize := domainadmin.NormalizePage(f.Page, f.PageSize)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}

func (r *memUserRepo) ReplaceRoles(_ context.Context, userID int64, roleIDs []int64) error {
	r.st.userRoles[userID] = append([]int64(nil), roleIDs...)
	return nil
}

func (r *memUserRepo) RolesByUser(_ context.Context, userID int64) ([]domainadmin.Role, error) {
	out := []domainadmin.Role{}
	for _, rid := range r.st.userRoles[userID] {
		if role, ok := r.st.roles[rid]; ok {
			out = append(out, *role)
		}
	}
	return out, nil
}

// ===== identities =====

type memIdentityRepo struct{ st *memState }

func (r *memIdentityRepo) FindByTypeKey(_ context.Context, t string, key string) (*domainadmin.AdminIdentity, error) {
	for _, id := range r.st.identities {
		if string(id.IdentityType) == t && id.IdentityKey == key {
			cp := *id
			return &cp, nil
		}
	}
	return nil, adminapp.ErrNotFound
}

func (r *memIdentityRepo) ListByUser(_ context.Context, userID int64) ([]domainadmin.AdminIdentity, error) {
	out := []domainadmin.AdminIdentity{}
	for _, id := range r.st.identities {
		if id.UserIDRef == userID {
			out = append(out, *id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (r *memIdentityRepo) Upsert(_ context.Context, identity *domainadmin.AdminIdentity) error {
	for _, ex := range r.st.identities {
		if ex.IdentityType == identity.IdentityType && ex.IdentityKey == identity.IdentityKey {
			ex.CredentialCiphertext = identity.CredentialCiphertext
			ex.UserIDRef = identity.UserIDRef
			ex.UpdatedAt = time.Now()
			return nil
		}
	}
	r.st.seqIdent++
	identity.ID = r.st.seqIdent
	now := time.Now()
	identity.CreatedAt, identity.UpdatedAt = now, now
	cp := *identity
	r.st.identities[identity.ID] = &cp
	return nil
}

// ===== roles =====

type memRoleRepo struct{ st *memState }

func (r *memRoleRepo) Create(_ context.Context, role *domainadmin.Role) error {
	for _, ex := range r.st.roles {
		if ex.RoleCode == role.RoleCode {
			return adminapp.ErrConflict
		}
	}
	r.st.seqRole++
	role.ID = r.st.seqRole
	now := time.Now()
	role.CreatedAt, role.UpdatedAt = now, now
	cp := *role
	r.st.roles[role.ID] = &cp
	return nil
}

func (r *memRoleRepo) Update(_ context.Context, role *domainadmin.Role) error {
	if _, ok := r.st.roles[role.ID]; !ok {
		return adminapp.ErrNotFound
	}
	role.UpdatedAt = time.Now()
	cp := *role
	r.st.roles[role.ID] = &cp
	return nil
}

func (r *memRoleRepo) Delete(_ context.Context, id int64) error {
	if _, ok := r.st.roles[id]; !ok {
		return adminapp.ErrNotFound
	}
	delete(r.st.roles, id)
	delete(r.st.rolePerms, id)
	for uid, rids := range r.st.userRoles {
		kept := rids[:0]
		for _, rid := range rids {
			if rid != id {
				kept = append(kept, rid)
			}
		}
		r.st.userRoles[uid] = kept
	}
	return nil
}

func (r *memRoleRepo) FindByID(_ context.Context, id int64) (*domainadmin.Role, error) {
	role, ok := r.st.roles[id]
	if !ok {
		return nil, adminapp.ErrNotFound
	}
	cp := *role
	return &cp, nil
}

func (r *memRoleRepo) List(_ context.Context, f domainadmin.RoleFilter) ([]domainadmin.Role, int, error) {
	all := make([]domainadmin.Role, 0, len(r.st.roles))
	for _, role := range r.st.roles {
		if f.Keyword != "" {
			kw := strings.ToLower(f.Keyword)
			if !strings.Contains(strings.ToLower(role.RoleCode), kw) &&
				!strings.Contains(strings.ToLower(role.RoleName), kw) {
				continue
			}
		}
		all = append(all, *role)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	total := len(all)
	page, pageSize := domainadmin.NormalizePage(f.Page, f.PageSize)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}

func (r *memRoleRepo) ReplacePermissions(_ context.Context, roleID int64, permIDs []int64) error {
	r.st.rolePerms[roleID] = append([]int64(nil), permIDs...)
	return nil
}

func (r *memRoleRepo) CountUsers(_ context.Context, roleID int64) (int, error) {
	n := 0
	for _, rids := range r.st.userRoles {
		for _, rid := range rids {
			if rid == roleID {
				n++
				break
			}
		}
	}
	return n, nil
}

func (r *memRoleRepo) PermissionsByRole(_ context.Context, roleID int64) ([]domainadmin.Permission, error) {
	out := []domainadmin.Permission{}
	for _, pid := range r.st.rolePerms[roleID] {
		if p, ok := r.st.perms[pid]; ok {
			out = append(out, *p)
		}
	}
	return out, nil
}

// ===== permissions =====

type memPermRepo struct{ st *memState }

func (r *memPermRepo) Create(_ context.Context, p *domainadmin.Permission) error {
	for _, ex := range r.st.perms {
		if ex.PermissionCode == p.PermissionCode {
			return adminapp.ErrConflict
		}
	}
	r.st.seqPerm++
	p.ID = r.st.seqPerm
	now := time.Now()
	p.CreatedAt, p.UpdatedAt = now, now
	cp := *p
	r.st.perms[p.ID] = &cp
	return nil
}

func (r *memPermRepo) Delete(_ context.Context, id int64) error {
	if _, ok := r.st.perms[id]; !ok {
		return adminapp.ErrNotFound
	}
	delete(r.st.perms, id)
	for rid, pids := range r.st.rolePerms {
		kept := pids[:0]
		for _, pid := range pids {
			if pid != id {
				kept = append(kept, pid)
			}
		}
		r.st.rolePerms[rid] = kept
	}
	return nil
}

func (r *memPermRepo) FindByID(_ context.Context, id int64) (*domainadmin.Permission, error) {
	p, ok := r.st.perms[id]
	if !ok {
		return nil, adminapp.ErrNotFound
	}
	cp := *p
	return &cp, nil
}

func (r *memPermRepo) List(_ context.Context, f domainadmin.PermissionFilter) ([]domainadmin.Permission, int, error) {
	all := make([]domainadmin.Permission, 0, len(r.st.perms))
	for _, p := range r.st.perms {
		if f.Keyword != "" {
			kw := strings.ToLower(f.Keyword)
			if !strings.Contains(strings.ToLower(p.PermissionCode), kw) &&
				!strings.Contains(strings.ToLower(p.PermissionName), kw) {
				continue
			}
		}
		all = append(all, *p)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	total := len(all)
	if f.All {
		return all, total, nil
	}
	page, pageSize := domainadmin.NormalizePage(f.Page, f.PageSize)
	start := (page - 1) * pageSize
	if start > total {
		start = total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return all[start:end], total, nil
}

func (r *memPermRepo) FindByIDs(_ context.Context, ids []int64) ([]domainadmin.Permission, error) {
	seen := map[int64]struct{}{}
	out := []domainadmin.Permission{}
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if p, ok := r.st.perms[id]; ok {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (r *memPermRepo) ListCodesByUser(_ context.Context, userID int64) ([]string, error) {
	groups := [][]string{}
	for _, rid := range r.st.userRoles[userID] {
		codes := []string{}
		for _, pid := range r.st.rolePerms[rid] {
			if p, ok := r.st.perms[pid]; ok {
				codes = append(codes, p.PermissionCode)
			}
		}
		groups = append(groups, codes)
	}
	return domainadmin.MergePermissionCodes(groups...), nil
}

func (r *memPermRepo) CountRoles(_ context.Context, permID int64) (int, error) {
	n := 0
	for _, pids := range r.st.rolePerms {
		for _, pid := range pids {
			if pid == permID {
				n++
				break
			}
		}
	}
	return n, nil
}

// ===== fakes =====

// fakeAudit 记录审计调用，供 S7/S8 断言。
type fakeAudit struct{ entries []adminapp.AuditEntry }

func (a *fakeAudit) Write(_ context.Context, e adminapp.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

func (a *fakeAudit) byAction(action string) (adminapp.AuditEntry, bool) {
	for _, e := range a.entries {
		if e.Action == action {
			return e, true
		}
	}
	return adminapp.AuditEntry{}, false
}

// fakeFeishu 实现 adminapp.FeishuClient：code=mock:<unionID> -> 该 union_id。
type fakeFeishu struct{}

func (fakeFeishu) ExchangeCode(_ context.Context, code, _ string) (adminapp.FeishuUser, error) {
	const prefix = "mock:"
	if !strings.HasPrefix(code, prefix) {
		return adminapp.FeishuUser{}, adminapp.ErrUnauthenticated
	}
	key := strings.TrimPrefix(code, prefix)
	return adminapp.FeishuUser{UnionID: key, OpenID: key, Name: key}, nil
}

var _ = common.EnvDevelop
