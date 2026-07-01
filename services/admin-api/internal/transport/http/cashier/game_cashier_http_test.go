package cashier

import (
	"net/http"
	"testing"
)

// 进程内 L3 接口测试（httptest 全链路 transport->app->domain + 内存仓储 + 真实 JWT/审计 spy）。
// 覆盖 game-cashier 4 个接口（GET/PUT profile、GET/PUT price-overrides）的场景维度，
// 与 tests/backend/scenarios/game-cashier.yaml 对齐：
//   S1 成功 / S2 鉴权401 / S3 权限403 / S4 校验失败 / S5 冲突 / S7 审计 / S10 事务回滚。
//   S6 跨env：两表为业务表、每环境独立 schema、写落当前 env schema（search_path 决定），
//     进程内无法断言 schema，由连库 harness（SCENARIO_WITH_DB=1）承担；本文件不带 env 谓词即等价说明。
//   S8 脱敏 N/A：本模块无 secret/file 密文字段（价格均为明文业务数据）。
//   S9 分页 N/A：GET /profile 为单对象、GET /price-overrides 全量返回 items[]，无分页入参。
//
// 关键夹具说明：cashier-template 发布流程已在发布时计算并写入确定性 checksum
// （domaincashier.ComputeVersionChecksum）；BindProfile 不再硬闸空 checksum，按 compact 仅校验
// templateId 存在 + 目标版本为 published，并原样记录该版本 checksum 为 snapshot_checksum。
// 为对断言固定可读的快照值，preparePublishedTemplate 在发布后白盒覆盖为已知字符串；
// TestBindProfilePublishedComputesChecksum 则走真实发布路径校验 checksum 非空且绑定成功。

const testGameID = "100001" // memStore 默认 games 映射 100001 -> rowID 1

func gameProfilePath() string   { return "/api/admin/games/" + testGameID + "/cashier/profile" }
func gameOverridesPath() string { return "/api/admin/games/" + testGameID + "/cashier/price-overrides" }
func gameProfilePathFor(id string) string {
	return "/api/admin/games/" + id + "/cashier/profile"
}

// setVersionChecksum 白盒覆盖 checksum 为已知可读值，便于断言固定快照（发布流程已计算真实 checksum）。
func (h *harness) setVersionChecksum(t *testing.T, templateID string, version int, checksum string) {
	t.Helper()
	tpl := h.store.state.templates[templateID]
	if tpl == nil {
		t.Fatalf("template %q not found in memstore", templateID)
	}
	for _, v := range h.store.state.versions {
		if v.TemplateIDRef == tpl.ID && v.Version == version {
			v.Checksum = checksum
			return
		}
	}
	t.Fatalf("version %d of template %q not found", version, templateID)
}

// preparePublishedTemplate 创建模板 + 一个 published 版本（带 checksum），返回版本号。
func (h *harness) preparePublishedTemplate(t *testing.T, templateID, checksum string) int {
	t.Helper()
	h.createTemplate(t, templateID, templateID, "manual_confirm")
	v := h.createVersion(t, templateID)
	assertStatus(t, h.upsertUSDRow(t, templateID, v, "10.00", "0.1"), http.StatusOK)
	assertStatus(t, h.publish(t, templateID, v), http.StatusOK)
	h.setVersionChecksum(t, templateID, v, checksum)
	return v
}

func validOverrideItem() map[string]any {
	return map[string]any{
		"countryCode": "US", "regionCode": "*", "currency": "USD", "priceId": "p_basic",
		"preTaxAmountMinor": 1000, "taxRate": "0.1", "taxAmountMinor": 100, "afterTaxAmountMinor": 1100,
		"reason": "promo", "effectiveAt": "2026-01-01T00:00:00Z",
	}
}

// ───────────────────────── S2 鉴权（缺/伪造令牌 → 401） ─────────────────────────

func TestGameCashierRequiresAuth(t *testing.T) {
	h := newHarness(t)
	for _, ep := range []struct{ m, p string }{
		{http.MethodGet, gameProfilePath()},
		{http.MethodPut, gameProfilePath()},
		{http.MethodGet, gameOverridesPath()},
		{http.MethodPut, gameOverridesPath()},
	} {
		res := h.do(t, ep.m, ep.p, "", nil)
		assertStatus(t, res, http.StatusUnauthorized)
		if res.errCode() != "UNAUTHENTICATED" {
			t.Fatalf("%s %s want UNAUTHENTICATED got %q", ep.m, ep.p, res.errCode())
		}
	}
	// 伪造 Bearer → 401。
	assertStatus(t, h.do(t, http.MethodGet, gameProfilePath(), "not.a.jwt", nil), http.StatusUnauthorized)
}

// ───────────────────────── S3 权限（403） ─────────────────────────

func TestGameCashierForbidden(t *testing.T) {
	h := newHarness(t)

	// 仅 cashier.read：写接口 → 403 FORBIDDEN。
	readOnly := h.readToken(t)
	for _, ep := range []struct {
		m, p string
		body any
	}{
		{http.MethodPut, gameProfilePath(), map[string]any{"templateId": "t1", "templateVersion": "1"}},
		{http.MethodPut, gameOverridesPath(), map[string]any{"items": []any{}}},
	} {
		res := h.do(t, ep.m, ep.p, readOnly, ep.body)
		assertStatus(t, res, http.StatusForbidden)
		if res.errCode() != "FORBIDDEN" {
			t.Fatalf("%s %s want FORBIDDEN got %q", ep.m, ep.p, res.errCode())
		}
	}

	// 缺 cashier.read：读接口 → 403。
	noRead := h.token(t, 12, []string{"system.read"})
	assertStatus(t, h.do(t, http.MethodGet, gameProfilePath(), noRead, nil), http.StatusForbidden)
	assertStatus(t, h.do(t, http.MethodGet, gameOverridesPath(), noRead, nil), http.StatusForbidden)
}

// ───────────────────────── S1 成功 + S7 审计（绑定 / 读取） ─────────────────────────

func TestBindProfileSuccessAndAudit(t *testing.T) {
	h := newHarness(t)
	h.preparePublishedTemplate(t, "tpl_a", "chk-tpl_a-v1")

	res := h.do(t, http.MethodPut, gameProfilePath(), h.writeToken(t), map[string]any{
		"templateId": "tpl_a", "templateVersion": "1",
	})
	assertStatus(t, res, http.StatusOK)
	d := res.data()
	if d["templateId"] != "tpl_a" || d["appliedTemplateVersion"] != "1" || d["snapshotChecksum"] != "chk-tpl_a-v1" {
		t.Fatalf("bind result mismatch: %v", d)
	}
	// S7：审计 cashier.profile.bind。
	if _, ok := h.audit.byAction("cashier.profile.bind"); !ok {
		t.Fatal("expected cashier.profile.bind audit")
	}

	// 绑定后 GET /profile → 200 + 同快照。
	got := h.do(t, http.MethodGet, gameProfilePath(), h.readToken(t), nil)
	assertStatus(t, got, http.StatusOK)
	if got.data()["snapshotChecksum"] != "chk-tpl_a-v1" {
		t.Fatalf("get profile mismatch: %v", got.data())
	}
}

// 升级绑定：切换到另一 published 版本，快照不实时跟随、显式升级后更新。
func TestBindProfileUpgradeVersion(t *testing.T) {
	h := newHarness(t)
	h.preparePublishedTemplate(t, "tpl_a", "chk-v1")
	// 绑定 v1。
	assertStatus(t, h.do(t, http.MethodPut, gameProfilePath(), h.writeToken(t), map[string]any{
		"templateId": "tpl_a", "templateVersion": "1",
	}), http.StatusOK)

	// 新建 v2（copy from published v1）并发布，注入新 checksum。
	v2res := h.do(t, http.MethodPost, "/api/admin/cashier/templates/tpl_a/versions", h.writeToken(t), map[string]any{
		"sourceType": "copy_published", "sourceVersion": "1",
	})
	assertStatus(t, v2res, http.StatusCreated)
	v2 := int(v2res.data()["version"].(float64))
	assertStatus(t, h.publish(t, "tpl_a", v2), http.StatusOK)
	h.setVersionChecksum(t, "tpl_a", v2, "chk-v2")

	// 升级绑定到 v2。
	up := h.do(t, http.MethodPut, gameProfilePath(), h.writeToken(t), map[string]any{
		"templateId": "tpl_a", "templateVersion": itoa(v2),
	})
	assertStatus(t, up, http.StatusOK)
	if up.data()["appliedTemplateVersion"] != itoa(v2) || up.data()["snapshotChecksum"] != "chk-v2" {
		t.Fatalf("upgrade bind mismatch: %v", up.data())
	}
}

// S4/空态：未绑定 profile → 404 NOT_FOUND（前端据此映射空态）。
func TestGetProfileUnboundNotFound(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, gameProfilePath(), h.readToken(t), nil)
	assertStatus(t, res, http.StatusNotFound)
	if res.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q", res.errCode())
	}
}

// ───────────────────────── S4 校验失败（profile） ─────────────────────────

func TestBindProfileValidation(t *testing.T) {
	h := newHarness(t)
	h.preparePublishedTemplate(t, "tpl_a", "chk-v1")
	tok := h.writeToken(t)
	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing_templateId", map[string]any{"templateVersion": "1"}},
		{"non_numeric_version", map[string]any{"templateId": "tpl_a", "templateVersion": "abc"}},
		{"zero_version", map[string]any{"templateId": "tpl_a", "templateVersion": "0"}},
		{"empty_version", map[string]any{"templateId": "tpl_a", "templateVersion": ""}},
	}
	for _, c := range cases {
		res := h.do(t, http.MethodPut, gameProfilePath(), tok, c.body)
		assertStatus(t, res, http.StatusBadRequest)
		if res.errCode() != "VALIDATION_FAILED" {
			t.Fatalf("%s want VALIDATION_FAILED got %q (%s)", c.name, res.errCode(), res.raw)
		}
	}
}

// ───────────────────────── S5 冲突（profile） ─────────────────────────

// 绑定 draft（非 published）版本 → 409 CONFLICT。
func TestBindProfileDraftRejected(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "tpl_a", "Tpl A", "manual_confirm")
	v1 := h.createVersion(t, "tpl_a") // draft
	res := h.do(t, http.MethodPut, gameProfilePath(), h.writeToken(t), map[string]any{
		"templateId": "tpl_a", "templateVersion": itoa(v1),
	})
	assertStatus(t, res, http.StatusConflict)
	if res.errCode() != "CONFLICT" {
		t.Fatalf("want CONFLICT got %q (%s)", res.errCode(), res.raw)
	}
}

// 绑定不存在的模板 → 404 NOT_FOUND。
func TestBindProfileTemplateNotFound(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPut, gameProfilePath(), h.writeToken(t), map[string]any{
		"templateId": "ghost", "templateVersion": "1",
	})
	assertStatus(t, res, http.StatusNotFound)
	if res.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q (%s)", res.errCode(), res.raw)
	}
}

// 绑定不存在的游戏 → 404 NOT_FOUND。
func TestBindProfileGameNotFound(t *testing.T) {
	h := newHarness(t)
	h.preparePublishedTemplate(t, "tpl_a", "chk-v1")
	res := h.do(t, http.MethodPut, gameProfilePathFor("999999"), h.writeToken(t), map[string]any{
		"templateId": "tpl_a", "templateVersion": "1",
	})
	assertStatus(t, res, http.StatusNotFound)
	if res.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q (%s)", res.errCode(), res.raw)
	}
}

// 回归（阻断2）：API 发布流程在发布时计算并写入确定性 checksum，BindProfile 走真实路径（不白盒注入）
// 应成功绑定，且 snapshotChecksum 非空。固化「发布计算 checksum → 绑定主流程真实 API 可达」。
func TestBindProfilePublishedComputesChecksum(t *testing.T) {
	h := newHarness(t)
	h.createTemplate(t, "tpl_a", "Tpl A", "manual_confirm")
	v1 := h.createVersion(t, "tpl_a")
	assertStatus(t, h.upsertUSDRow(t, "tpl_a", v1, "10.00", "0.1"), http.StatusOK)
	assertStatus(t, h.publish(t, "tpl_a", v1), http.StatusOK)
	// 注意：此处不注入 checksum，走真实 API 发布路径（发布已计算 checksum）。
	res := h.do(t, http.MethodPut, gameProfilePath(), h.writeToken(t), map[string]any{
		"templateId": "tpl_a", "templateVersion": itoa(v1),
	})
	assertStatus(t, res, http.StatusOK)
	chk, _ := res.data()["snapshotChecksum"].(string)
	if chk == "" {
		t.Fatalf("want non-empty snapshotChecksum from publish-computed checksum, got %q (%s)", chk, res.raw)
	}
}

// ───────────────────────── S1 成功 + S7 审计（price-overrides 全量替换 + 归一化） ─────────────────────────

func TestSavePriceOverridesSuccessAndAudit(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPut, gameOverridesPath(), h.writeToken(t), map[string]any{
		"items": []any{validOverrideItem()},
	})
	assertStatus(t, res, http.StatusOK)
	items, _ := res.data()["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected 1 saved override, got %v", items)
	}
	row := items[0].(map[string]any)
	if row["currency"] != "USD" || row["preTaxAmountMinor"] != float64(1000) || row["regionCode"] != "*" {
		t.Fatalf("override row mismatch: %v", row)
	}
	// S7：审计 cashier.override.update。
	if _, ok := h.audit.byAction("cashier.override.update"); !ok {
		t.Fatal("expected cashier.override.update audit")
	}

	// 全量替换语义：用空 items 替换 → 清空。
	clr := h.do(t, http.MethodPut, gameOverridesPath(), h.writeToken(t), map[string]any{"items": []any{}})
	assertStatus(t, clr, http.StatusOK)
	got := h.do(t, http.MethodGet, gameOverridesPath(), h.readToken(t), nil)
	gotItems, _ := got.data()["items"].([]any)
	if len(gotItems) != 0 {
		t.Fatalf("replace-with-empty must clear overrides, got %v", gotItems)
	}
}

func TestListPriceOverridesEmpty(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodGet, gameOverridesPath(), h.readToken(t), nil)
	assertStatus(t, res, http.StatusOK)
	items, _ := res.data()["items"].([]any)
	if len(items) != 0 {
		t.Fatalf("expected empty overrides, got %v", items)
	}
}

// ───────────────────────── S4 校验失败（price-overrides） ─────────────────────────

// currency 不在 currency_specs → 400 CURRENCY_NOT_SUPPORTED。
func TestSavePriceOverridesCurrencyNotSupported(t *testing.T) {
	h := newHarness(t)
	item := validOverrideItem()
	item["currency"] = "EUR"
	res := h.do(t, http.MethodPut, gameOverridesPath(), h.writeToken(t), map[string]any{
		"items": []any{item},
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "CURRENCY_NOT_SUPPORTED" {
		t.Fatalf("want CURRENCY_NOT_SUPPORTED got %q (%s)", res.errCode(), res.raw)
	}
}

// 归一化后低于 currency_specs 下限（USD min=50，10 minor）→ 400 VALIDATION_FAILED。
func TestSavePriceOverridesBelowMinimum(t *testing.T) {
	h := newHarness(t)
	item := validOverrideItem()
	item["preTaxAmountMinor"] = 10
	res := h.do(t, http.MethodPut, gameOverridesPath(), h.writeToken(t), map[string]any{
		"items": []any{item},
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q (%s)", res.errCode(), res.raw)
	}
}

// effectiveAt 非法格式 → 400 VALIDATION_FAILED。
func TestSavePriceOverridesInvalidEffectiveAt(t *testing.T) {
	h := newHarness(t)
	item := validOverrideItem()
	item["effectiveAt"] = "not-a-time"
	res := h.do(t, http.MethodPut, gameOverridesPath(), h.writeToken(t), map[string]any{
		"items": []any{item},
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q (%s)", res.errCode(), res.raw)
	}
}

// 缺必填键字段（priceId 空）→ 400 VALIDATION_FAILED。
func TestSavePriceOverridesMissingPriceID(t *testing.T) {
	h := newHarness(t)
	item := validOverrideItem()
	item["priceId"] = "  "
	res := h.do(t, http.MethodPut, gameOverridesPath(), h.writeToken(t), map[string]any{
		"items": []any{item},
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED got %q (%s)", res.errCode(), res.raw)
	}
}

// 不存在的游戏 → 404 NOT_FOUND。
func TestSavePriceOverridesGameNotFound(t *testing.T) {
	h := newHarness(t)
	res := h.do(t, http.MethodPut, "/api/admin/games/999999/cashier/price-overrides", h.writeToken(t), map[string]any{
		"items": []any{validOverrideItem()},
	})
	assertStatus(t, res, http.StatusNotFound)
	if res.errCode() != "NOT_FOUND" {
		t.Fatalf("want NOT_FOUND got %q (%s)", res.errCode(), res.raw)
	}
}

// ───────────────────────── S10 事务回滚（price-overrides 全量替换原子性） ─────────────────────────

// 先存入 1 条有效覆盖，再用 [有效, 非法currency] 批量替换 → 整体失败回滚，原有覆盖不被清空/部分写入。
func TestSavePriceOverridesTransactionRollback(t *testing.T) {
	h := newHarness(t)
	// 初始：1 条有效覆盖。
	first := validOverrideItem()
	first["priceId"] = "p_keep"
	assertStatus(t, h.do(t, http.MethodPut, gameOverridesPath(), h.writeToken(t), map[string]any{
		"items": []any{first},
	}), http.StatusOK)

	// 批量替换：第二条 currency 非法 → 整体 400。
	good := validOverrideItem()
	good["priceId"] = "p_new"
	bad := validOverrideItem()
	bad["priceId"] = "p_bad"
	bad["currency"] = "EUR"
	res := h.do(t, http.MethodPut, gameOverridesPath(), h.writeToken(t), map[string]any{
		"items": []any{good, bad},
	})
	if res.status < 400 {
		t.Fatalf("expected failure, got %d (%s)", res.status, res.raw)
	}

	// 回滚断言：仍为初始的 1 条 p_keep，无部分写入。
	got := h.do(t, http.MethodGet, gameOverridesPath(), h.readToken(t), nil)
	assertStatus(t, got, http.StatusOK)
	items, _ := got.data()["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("rollback failed: expected 1 original override, got %d (%v)", len(items), items)
	}
	if items[0].(map[string]any)["priceId"] != "p_keep" {
		t.Fatalf("rollback failed: original override changed: %v", items[0])
	}
}

// 回归（非阻断4）：SavePriceOverrides 在应用层预检 items 内重复 (country,region,currency,priceId)，
// 返回 400 VALIDATION_FAILED，而非连库命中 gcpo_key UNIQUE 的 409 CONFLICT。
func TestSavePriceOverridesDuplicateKeyPrevalidated(t *testing.T) {
	h := newHarness(t)
	dupA := validOverrideItem()
	dupB := validOverrideItem() // 与 dupA 同键 (US,*,USD,p_basic)
	res := h.do(t, http.MethodPut, gameOverridesPath(), h.writeToken(t), map[string]any{
		"items": []any{dupA, dupB},
	})
	assertStatus(t, res, http.StatusBadRequest)
	if res.errCode() != "VALIDATION_FAILED" {
		t.Fatalf("want VALIDATION_FAILED (duplicate key) got %q (%s)", res.errCode(), res.raw)
	}
}
