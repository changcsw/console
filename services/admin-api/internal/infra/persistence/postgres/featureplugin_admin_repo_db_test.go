package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	adminapp "github.com/csw/console/services/admin-api/internal/app/admin"
	"github.com/csw/console/services/admin-api/internal/app/dto"
	featurepluginapp "github.com/csw/console/services/admin-api/internal/app/featureplugin"
	domainplugin "github.com/csw/console/services/admin-api/internal/domain/plugin"
)

// 连库回归（SCENARIO_WITH_DB=1 + POSTGRES_DSN/DATABASE_URL，约定同 account_auth_repo_db_test.go）：
// 覆盖 000020 迁移落地的三张表（feature_plugin_categories/feature_plugins/feature_plugin_templates）
// 仓储 SQL 在真实 Postgres 上的行为——列/JOIN/唯一冲突/NULL 处理——而不是 memstore 等价体。
// 每个 CRUD 用例都在事务内完成并回滚，不污染共享库的持久状态。

func featurePluginDSN(t *testing.T) string {
	t.Helper()
	if os.Getenv("SCENARIO_WITH_DB") != "1" {
		t.Skip("SCENARIO_WITH_DB=1 required")
	}
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("POSTGRES_DSN or DATABASE_URL required")
	}
	return dsn
}

func featurePluginPool(t *testing.T, dsn string) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pg pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// featurePluginTx 开一个事务并注册回滚，供各 CRUD 用例在其中造数据/断言而不留痕。
func featurePluginTx(t *testing.T, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}

// withSavepoint 在 SAVEPOINT 内执行 fn 后立即回滚：真实 Postgres 一旦某条语句报错
// （如唯一键冲突），整个外层事务会被置为 aborted，后续语句一律报 25P02，即便
// mapErr 已把该错误归一化。用嵌套事务（pgx 在已有 Tx 上 Begin 会退化为 SAVEPOINT）
// 隔离「预期会报错」的断言，使外层事务在断言后仍可继续跑后续用例。
func withSavepoint(t *testing.T, parent pgx.Tx, fn func(sp pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	sp, err := parent.Begin(ctx)
	if err != nil {
		t.Fatalf("begin savepoint: %v", err)
	}
	defer func() { _ = sp.Rollback(context.Background()) }()
	fn(sp)
}

func TestFeaturePluginCategoryRepo_CRUD_DB(t *testing.T) {
	dsn := featurePluginDSN(t)
	pool := featurePluginPool(t, dsn)
	tx := featurePluginTx(t, pool)
	ctx := context.Background()
	repo := &FeaturePluginCategoryRepo{db: tx}

	created, err := repo.Insert(ctx, domainplugin.FeaturePluginCategory{
		CategoryCode: "db_test_cat", CategoryName: "DB 测试分类", Enabled: true, Sort: 5,
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if created.ID == 0 || created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("Insert should populate id/timestamps, got %+v", created)
	}

	// GetByID / GetByCode 均含 pluginCount 子查询列，插入即应可回读且计数为 0。
	byID, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if byID.PluginCount != 0 || byID.Category.CategoryCode != "db_test_cat" {
		t.Fatalf("GetByID mismatch: %+v", byID)
	}
	byCode, err := repo.GetByCode(ctx, "db_test_cat")
	if err != nil {
		t.Fatalf("GetByCode: %v", err)
	}
	if byCode.Category.ID != created.ID {
		t.Fatalf("GetByCode id mismatch: got %d want %d", byCode.Category.ID, created.ID)
	}

	// List 按 enabled 过滤且排序 sort/id 升序；至少应含刚插入的这一行。
	all, err := repo.List(ctx, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, row := range all {
		if row.Category.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("List should include inserted row, got %d rows", len(all))
	}

	// Update 部分列，updated_at 应前进。
	name := "DB 测试分类（改）"
	if err := repo.Update(ctx, created.ID, featurepluginapp.CategoryPatch{CategoryName: &name}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if updated.Category.CategoryName != name {
		t.Fatalf("Update did not persist: got %q", updated.Category.CategoryName)
	}
	if !updated.Category.UpdatedAt.After(created.UpdatedAt) && !updated.Category.UpdatedAt.Equal(created.UpdatedAt) {
		t.Fatalf("updated_at should not regress: created=%v updated=%v", created.UpdatedAt, updated.Category.UpdatedAt)
	}

	// category_code 唯一：插入同 code 第二行应映射为 ErrConflict（唯一键冲突会中止事务，
	// 用 SAVEPOINT 隔离，断言后立即回滚，外层事务不受影响）。
	withSavepoint(t, tx, func(sp pgx.Tx) {
		spRepo := &FeaturePluginCategoryRepo{db: sp}
		_, err := spRepo.Insert(ctx, domainplugin.FeaturePluginCategory{CategoryCode: "db_test_cat", CategoryName: "重复", Enabled: true})
		if !errors.Is(err, adminapp.ErrConflict) {
			t.Fatalf("duplicate category_code: want ErrConflict, got %v", err)
		}
	})

	// Update/Delete 对不存在的 id 应映射为 ErrNotFound（RowsAffected=0）。
	if err := repo.Update(ctx, 999999999, featurepluginapp.CategoryPatch{CategoryName: &name}); !errors.Is(err, adminapp.ErrNotFound) {
		t.Fatalf("update missing id: want ErrNotFound, got %v", err)
	}
	if err := repo.Delete(ctx, 999999999); !errors.Is(err, adminapp.ErrNotFound) {
		t.Fatalf("delete missing id: want ErrNotFound, got %v", err)
	}

	// Delete 正常路径。
	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, created.ID); !errors.Is(err, adminapp.ErrNotFound) {
		t.Fatalf("GetByID after delete: want ErrNotFound, got %v", err)
	}
}

func TestFeaturePluginAdminRepo_PluginCRUD_DB(t *testing.T) {
	dsn := featurePluginDSN(t)
	pool := featurePluginPool(t, dsn)
	tx := featurePluginTx(t, pool)
	ctx := context.Background()
	catRepo := &FeaturePluginCategoryRepo{db: tx}
	repo := &FeaturePluginAdminRepo{db: tx}
	tplRepo := &FeaturePluginTemplateAdminRepo{db: tx}

	cat, err := catRepo.Insert(ctx, domainplugin.FeaturePluginCategory{CategoryCode: "db_test_plugin_cat", CategoryName: "DB 测试分类", Enabled: true})
	if err != nil {
		t.Fatalf("category Insert: %v", err)
	}

	catID := cat.ID
	if err := repo.Insert(ctx, domainplugin.FeaturePlugin{
		PluginID: "db_test_plugin", PluginName: "DB 测试插件", CategoryIDRef: &catID, Region: "domestic", Enabled: true, Sort: 3,
	}); err != nil {
		t.Fatalf("plugin Insert: %v", err)
	}

	row, err := repo.GetByPluginID(ctx, "db_test_plugin")
	if err != nil {
		t.Fatalf("GetByPluginID: %v", err)
	}
	if row.CategoryCode != "db_test_plugin_cat" || row.TemplateCount != 0 {
		t.Fatalf("GetByPluginID join mismatch: %+v", row)
	}

	// List 按 keyword/categoryId/region/enabled 过滤，分页参数正确传导。
	page, total, err := repo.List(ctx, dto.ListFeaturePluginsQuery{Keyword: "db_test_plugin", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total < 1 || len(page) < 1 {
		t.Fatalf("List should find the inserted plugin, got total=%d len=%d", total, len(page))
	}

	// CountByCategory：刚插入的插件应计入其分类。
	count, err := repo.CountByCategory(ctx, catID)
	if err != nil {
		t.Fatalf("CountByCategory: %v", err)
	}
	if count != 1 {
		t.Fatalf("CountByCategory: want 1, got %d", count)
	}

	// Update：categoryId 清空（NullableInt64.Value=nil）应写 NULL，join 后分类冗余字段清空。
	if err := repo.Update(ctx, "db_test_plugin", featurepluginapp.FeaturePluginPatch{
		CategoryIDRef: dto.NullableInt64{Present: true, Value: nil},
	}); err != nil {
		t.Fatalf("Update clear category: %v", err)
	}
	cleared, err := repo.GetByPluginID(ctx, "db_test_plugin")
	if err != nil {
		t.Fatalf("GetByPluginID after clear: %v", err)
	}
	if cleared.Plugin.CategoryIDRef != nil || cleared.CategoryCode != "" {
		t.Fatalf("category should be cleared, got %+v", cleared)
	}

	// CountReferences：先验证「无引用」路径可正常跑通四个子查询（含游戏侧业务表，当前 env 已建表）。
	refs, err := repo.CountReferences(ctx, row.Plugin.ID)
	if err != nil {
		t.Fatalf("CountReferences: %v", err)
	}
	if refs.Total() != 0 {
		t.Fatalf("fresh plugin should have zero references, got %+v", refs)
	}

	// 挂一个模板版本后，CountReferences.Templates 应变为 1，且插件被引用应拒绝真实删除
	// （这里只验证仓储层计数，删除前置校验在 app 层 Service.DeletePlugin）。
	if _, err := tplRepo.Insert(ctx, domainplugin.FeaturePluginTemplate{
		PluginIDRef: row.Plugin.ID, TemplateVersion: "v1",
		FormSchema: []domainplugin.PluginFormField{{Key: "appId", Label: "App ID", Component: "input", Required: true, Scope: "both"}},
		Enabled:    true,
	}); err != nil {
		t.Fatalf("template Insert: %v", err)
	}
	refsWithTemplate, err := repo.CountReferences(ctx, row.Plugin.ID)
	if err != nil {
		t.Fatalf("CountReferences with template: %v", err)
	}
	if refsWithTemplate.Templates != 1 {
		t.Fatalf("want Templates=1, got %+v", refsWithTemplate)
	}

	// plugin_id 唯一：重复插入应映射为 ErrConflict（SAVEPOINT 隔离，见上方注释）。
	withSavepoint(t, tx, func(sp pgx.Tx) {
		spRepo := &FeaturePluginAdminRepo{db: sp}
		err := spRepo.Insert(ctx, domainplugin.FeaturePlugin{PluginID: "db_test_plugin", PluginName: "重复", Region: "domestic", Enabled: true})
		if !errors.Is(err, adminapp.ErrConflict) {
			t.Fatalf("duplicate plugin_id: want ErrConflict, got %v", err)
		}
	})

	// Delete 前先清掉挂在这个插件上的模板（FK 约束），模拟服务层「引用清零后才允许删除」的前置条件。
	if _, err := tx.Exec(ctx, `DELETE FROM platform.feature_plugin_templates WHERE plugin_id_ref = $1`, row.Plugin.ID); err != nil {
		t.Fatalf("cleanup template before delete: %v", err)
	}
	// Delete 正常路径 + 不存在 id 映射 ErrNotFound。
	if err := repo.Delete(ctx, row.Plugin.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := repo.Delete(ctx, 999999999); !errors.Is(err, adminapp.ErrNotFound) {
		t.Fatalf("delete missing id: want ErrNotFound, got %v", err)
	}
}

func TestFeaturePluginTemplateAdminRepo_CRUD_DB(t *testing.T) {
	dsn := featurePluginDSN(t)
	pool := featurePluginPool(t, dsn)
	tx := featurePluginTx(t, pool)
	ctx := context.Background()
	pluginRepo := &FeaturePluginAdminRepo{db: tx}
	repo := &FeaturePluginTemplateAdminRepo{db: tx}

	if err := pluginRepo.Insert(ctx, domainplugin.FeaturePlugin{
		PluginID: "db_test_tpl_plugin", PluginName: "DB 模板宿主插件", Region: "overseas", Enabled: true,
	}); err != nil {
		t.Fatalf("plugin Insert: %v", err)
	}
	plugin, err := pluginRepo.GetByPluginID(ctx, "db_test_tpl_plugin")
	if err != nil {
		t.Fatalf("GetByPluginID: %v", err)
	}

	tpl := domainplugin.FeaturePluginTemplate{
		PluginIDRef:     plugin.Plugin.ID,
		TemplateVersion: "v1",
		FormSchema: []domainplugin.PluginFormField{
			{Key: "appId", Label: "App ID", Component: "input", Required: true, Order: 10, Scope: "both"},
			{Key: "appSecret", Label: "App Secret", Component: "password", Required: true, Order: 20, Scope: "server"},
		},
		SecretFields: []string{"appSecret"},
		ValidationRules: map[string]domainplugin.PluginValidationRule{
			"appId": {Required: true, MinLen: intPtr(1)},
		},
		Enabled: true,
	}
	created, err := repo.Insert(ctx, tpl)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if created.ID == 0 {
		t.Fatalf("Insert should assign id")
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.PluginID != "db_test_tpl_plugin" {
		t.Fatalf("GetByID should join plugin_id, got %q", got.PluginID)
	}
	if len(got.FormSchema) != 2 || got.FormSchema[1].Component != "password" {
		t.Fatalf("form_schema_json round-trip mismatch: %+v", got.FormSchema)
	}
	if len(got.SecretFields) != 1 || got.SecretFields[0] != "appSecret" {
		t.Fatalf("secret_fields_json round-trip mismatch: %+v", got.SecretFields)
	}
	if got.ValidationRules["appId"].MinLen == nil || *got.ValidationRules["appId"].MinLen != 1 {
		t.Fatalf("validation_rules_json round-trip mismatch: %+v", got.ValidationRules)
	}

	list, err := repo.ListByPlugin(ctx, plugin.Plugin.ID)
	if err != nil {
		t.Fatalf("ListByPlugin: %v", err)
	}
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("ListByPlugin mismatch: %+v", list)
	}

	// Replace：整体覆盖四件套 + enabled，template_version/plugin_id_ref 不变。
	replacement := got
	replacement.FormSchema = []domainplugin.PluginFormField{{Key: "clientId", Label: "Client ID", Component: "input", Required: true, Scope: "both"}}
	replacement.SecretFields = []string{}
	replacement.Enabled = false
	if err := repo.Replace(ctx, replacement); err != nil {
		t.Fatalf("Replace: %v", err)
	}
	afterReplace, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID after replace: %v", err)
	}
	if len(afterReplace.FormSchema) != 1 || afterReplace.FormSchema[0].Key != "clientId" {
		t.Fatalf("Replace should overwrite form_schema_json, got %+v", afterReplace.FormSchema)
	}
	if len(afterReplace.SecretFields) != 0 {
		t.Fatalf("Replace should clear secret_fields_json, got %+v", afterReplace.SecretFields)
	}
	if afterReplace.Enabled {
		t.Fatalf("Replace should persist enabled=false")
	}
	if afterReplace.TemplateVersion != "v1" || afterReplace.PluginIDRef != plugin.Plugin.ID {
		t.Fatalf("Replace must not change version/plugin_id_ref, got %+v", afterReplace)
	}

	// (plugin_id_ref, template_version) 唯一：重复插入应映射为 ErrConflict（SAVEPOINT 隔离，见上方注释）。
	withSavepoint(t, tx, func(sp pgx.Tx) {
		spRepo := &FeaturePluginTemplateAdminRepo{db: sp}
		_, err := spRepo.Insert(ctx, domainplugin.FeaturePluginTemplate{PluginIDRef: plugin.Plugin.ID, TemplateVersion: "v1", Enabled: true})
		if !errors.Is(err, adminapp.ErrConflict) {
			t.Fatalf("duplicate (plugin_id_ref, template_version): want ErrConflict, got %v", err)
		}
	})

	// Replace 对不存在的 id 应映射为 ErrNotFound。
	ghost := replacement
	ghost.ID = 999999999
	if err := repo.Replace(ctx, ghost); !errors.Is(err, adminapp.ErrNotFound) {
		t.Fatalf("replace missing id: want ErrNotFound, got %v", err)
	}
}

// TestFeaturePluginAdminRepo_CountReferences_EnvSchemaNotReady_DB 验证已知边界问题的修复：
// 若当前 env schema 缺 game_channel_plugin_configs / channel_package_plugin_overrides
// （未跑 000012/000017），底层会报 42P01（relation does not exist）。修复前 mapErr 不认识
// 这个 code，会原样上抛并最终在 handler 兜底成裸 500；修复后应返回可读的
// *featurepluginapp.Error{Status:409, Code:"ENV_SCHEMA_NOT_READY"}。
// 用独立裸连接（非共享池）造一个「没跑过 000012」的 schema，验证后即删除，不污染共享库。
func TestFeaturePluginAdminRepo_CountReferences_EnvSchemaNotReady_DB(t *testing.T) {
	dsn := featurePluginDSN(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	// t.Cleanup 按注册的逆序执行：先注册 Close，再注册 DROP SCHEMA，
	// 使 DROP SCHEMA 先于 Close 在存活连接上执行，避免连接已关闭导致清理静默失效。
	t.Cleanup(func() { conn.Close(context.Background()) })

	const testSchema = "qa_env_schema_not_ready_test"
	if _, err := conn.Exec(ctx, "DROP SCHEMA IF EXISTS "+testSchema+" CASCADE"); err != nil {
		t.Fatalf("pre-clean schema: %v", err)
	}
	if _, err := conn.Exec(ctx, "CREATE SCHEMA "+testSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+testSchema+" CASCADE")
	})

	// 只切 search_path，不建 game_channel_plugin_configs / channel_package_plugin_overrides，
	// 模拟「该 env schema 尚未执行 000012/000017」的边界状态。platform.feature_plugins 里
	// 挑一个真实存在的 plugin_id_ref（realname，由 common/feature-plugin.sql 固定 seed），
	// 只是为了让前两个子查询（platform 前缀，不受 search_path 影响）也能正常求值。
	if _, err := conn.Exec(ctx, "SET search_path = "+testSchema+", platform"); err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	repo := &FeaturePluginAdminRepo{db: conn}
	_, err = repo.CountReferences(ctx, 1)
	if err == nil {
		t.Fatal("want error when game-side business tables are missing, got nil")
	}

	var appErr *featurepluginapp.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("want *featurepluginapp.Error (readable 409), got raw error: %v", err)
	}
	if appErr.Code != "ENV_SCHEMA_NOT_READY" {
		t.Fatalf("want code ENV_SCHEMA_NOT_READY, got %q (message=%q)", appErr.Code, appErr.Message)
	}
	if appErr.Status != 409 {
		t.Fatalf("want HTTP 409, got %d", appErr.Status)
	}
	if appErr.Message == "" {
		t.Fatal("message should be human-readable, got empty")
	}
}

func intPtr(v int) *int { return &v }
