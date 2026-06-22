# v2 测试体系（测试 harness + 统一回归）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: 使用 superpowers:subagent-driven-development（推荐）或 superpowers:executing-plans 逐任务实施。步骤用 `- [ ]` 复选框跟踪。

**Goal:** 把 `docs/architecture/v2/03-testing.md` 描述的测试体系落地为**可运行的基础设施**：`tests/` 目录树、Postgres 依赖（compose）、迁移/seed 脚本、后端 scenario harness（Go，按 manifest 驱动 httptest）、前端 Playwright e2e（真实截图/trace/HTML report）、统一回归入口脚本与 summary 产物，并用一条贯穿现有 scaffold 的 smoke 切片证明全链路可跑。

**Architecture:** 单元/集成/组件测试就近放代码侧；跨栈 e2e、scenario manifest（数据）、fixtures、reports 放仓库顶层 `tests/`。后端 harness **代码**放在 Go module 内（`services/admin-api/internal/testkit/scenario`，可 import `httpserver` 用 httptest 在进程内起服务），**manifest 数据**放顶层 `tests/backend/scenarios/*.yaml`，harness 运行期向上查找仓库根定位 manifest 目录。回归脚本 `scripts/regression/*.sh` 编排：起 PG → migrate → seed → 后端（`go test ./...` + harness）→ 前端（vitest + Playwright）→ 汇总 `tests/reports/summary.{md,json}`。

**Tech Stack:** Go 1.22 + `net/http/httptest` + `gopkg.in/yaml.v3`；前端 Vite 6 + Vitest 4 + `@playwright/test`；Postgres 16（docker compose）+ `golang-migrate`；编排用 POSIX `sh`。

---

## 范围与约束（务必先读）

- 本计划交付**测试基础设施 + smoke 切片**，不实现业务模块。各模块完整的 S1–S10 场景矩阵 YAML 与 DB 状态断言**依赖对应模块先落地**（`services/admin-api` 目前为 scaffold：`main.go` 未连库、`httpserver` 用 `http.ServeMux` + scaffold service）。这些**逐模块场景**在对应模块实现后按本 harness 的 manifest schema 增量补充，不在本计划内。
- 现有可被 smoke 覆盖的真实端点：`GET /healthz`、`GET /api/admin/me`、`GET /api/admin/games`、`POST /api/admin/games/{id}/sync/preview`（未知 section → 400，请求体字段 `selected_sections`）。
- harness 默认以**进程内 httptest** 跑后端 manifest（无需连库），因此 smoke 不依赖 PG；PG/compose/迁移/seed 作为**集成测试地基**先就绪，待模块连库后启用 `expect.db` 断言。
- 不引入 chi/pgx（属模块实现）；不新增 CI（spec 未要求，留待后续）。
- 提交粒度：每个 Task 结束提交一次。提交信息用 `test:` / `chore(test):` 前缀。

## 文件结构（本计划新建/修改）

**新建（仓库顶层数据/产物树）**
- `tests/README.md` — 测试树说明（引用 `03-testing.md`）
- `tests/backend/scenarios/smoke.yaml` — 后端 smoke manifest（worked example）
- `tests/backend/scenarios/README.md` — manifest schema 速查
- `tests/frontend/e2e/smoke.spec.ts` — Playwright smoke
- `tests/frontend/visual-baseline/.gitkeep`、`tests/frontend/screenshots/.gitkeep`
- `tests/fixtures/README.md`、`tests/fixtures/common/.gitkeep`、`tests/fixtures/sandbox/.gitkeep`、`tests/fixtures/production/.gitkeep`
- `tests/reports/.gitkeep`
- `tests/.gitignore` — 忽略运行期产物（screenshots/reports）

**新建（后端 harness 代码，Go module 内）**
- `services/admin-api/internal/testkit/scenario/manifest.go` — manifest 结构 + 加载
- `services/admin-api/internal/testkit/scenario/jsonpath.go` — 极简点路径取值
- `services/admin-api/internal/testkit/scenario/runner.go` — 进程内执行一个 case
- `services/admin-api/internal/testkit/scenario/reporter.go` — 产出 junit-ish + json summary 片段
- `services/admin-api/internal/testkit/scenario/repo_root.go` — 向上查找仓库根
- `services/admin-api/internal/testkit/scenario/*_test.go` — 单元 + 入口测试

**新建（依赖与编排）**
- `docker-compose.yml` — Postgres 16 服务（仅测试/本地）
- `scripts/regression/lib.sh` — 公共：日志、等待 PG、查找根
- `scripts/regression/db.sh` — migrate up + seed
- `scripts/regression/backend.sh` — `go test ./...` + harness
- `scripts/regression/frontend.sh` — vitest + Playwright
- `scripts/regression/run.sh` — 全链路 + summary
- `scripts/regression/summarize.sh` — 聚合 `tests/reports/summary.{md,json}`

**修改**
- `services/admin-api/go.mod` / `go.sum` — 加 `gopkg.in/yaml.v3`
- `apps/admin-web/package.json` — 加 `@playwright/test` 与 `e2e` 脚本
- `apps/admin-web/playwright.config.ts` — 新建（webServer + reporters）
- `docs/architecture/v2/03-testing.md` — 末尾补「实现状态」小节
- `README.md`（仓库根）— 补测试/回归运行说明
- `.gitignore`（仓库根）— 忽略 `tests/reports/*`、`tests/frontend/screenshots/*`、`playwright-report`

---

## Phase 0 — 仓库测试树与依赖地基

### Task 1: 建立 `tests/` 目录树与忽略规则

**Files:**
- Create: `tests/README.md`, `tests/.gitignore`, `tests/backend/scenarios/README.md`, `tests/fixtures/README.md`
- Create（占位）: `tests/fixtures/common/.gitkeep`, `tests/fixtures/sandbox/.gitkeep`, `tests/fixtures/production/.gitkeep`, `tests/frontend/screenshots/.gitkeep`, `tests/frontend/visual-baseline/.gitkeep`, `tests/reports/.gitkeep`
- Modify: `.gitignore`（仓库根）

- [ ] **Step 1: 建目录与占位文件**

Run:
```bash
cd /Users/csw/gitproject/console
mkdir -p tests/backend/scenarios tests/frontend/e2e tests/frontend/screenshots \
  tests/frontend/visual-baseline tests/fixtures/common tests/fixtures/sandbox \
  tests/fixtures/production tests/reports
touch tests/fixtures/common/.gitkeep tests/fixtures/sandbox/.gitkeep \
  tests/fixtures/production/.gitkeep tests/frontend/screenshots/.gitkeep \
  tests/frontend/visual-baseline/.gitkeep tests/reports/.gitkeep
```
Expected: 目录创建成功，无报错。

- [ ] **Step 2: 写 `tests/README.md`**

```markdown
# tests — 跨栈测试与统一回归

本目录承载**跨栈 / 顶层**测试资产；单元/集成/组件测试就近放在各自代码侧（见 `docs/architecture/v2/03-testing.md`）。

- `backend/scenarios/*.yaml` — 后端接口场景矩阵 manifest（数据）。harness 代码在 `services/admin-api/internal/testkit/scenario`。
- `frontend/e2e/*.spec.ts` — Playwright 用例（真实页面截图 / trace）。
- `frontend/screenshots/` — 运行期采集截图（git 忽略）。
- `frontend/visual-baseline/` — 截图基线（git 跟踪）。
- `fixtures/{common,sandbox,production}/` — 统一 seed/fixtures（按 env 维度）。
- `reports/` — 测试产物：junit / html / summary（git 忽略）。

运行：`scripts/regression/run.sh`（全量）/ `backend.sh` / `frontend.sh`。
```

- [ ] **Step 3: 写 `tests/.gitignore`**

```gitignore
reports/*
!reports/.gitkeep
frontend/screenshots/*
!frontend/screenshots/.gitkeep
```

- [ ] **Step 4: 写 `tests/backend/scenarios/README.md`（manifest schema）**

```markdown
# 后端 scenario manifest schema

每个 `<module>.yaml`：
```yaml
module: <id>            # 对齐 docs 模块 id
cases:
  - name: <唯一名>
    dimension: S1       # 可选，见 03-testing §4
    request:
      method: GET|POST|PUT|PATCH|DELETE
      path: /api/admin/...
      headers: { Authorization: "Bearer x" }   # 可选
      body: { ... }                            # 可选，对象→JSON
    expect:
      status: 200
      jsonContains: { "data.status": "ok" }    # 点路径 → 期望值（相等）
```
harness（`services/admin-api/internal/testkit/scenario`）以进程内 httptest 执行，不需要连库。
`expect.db` 等 DB 断言待模块连库后扩展。
```

- [ ] **Step 5: 写 `tests/fixtures/README.md`**

```markdown
# fixtures（按 env 维度）

- `common/` — 平台级、全 env 共享（字典/枚举/currency_specs/channels(region)/模板四件套）。
- `sandbox/` — 同步源样本。
- `production/` — 同步目标基线（用于 sync diff/baseline/nonce 测试）。

约定：seed 用 `ON CONFLICT DO NOTHING` 幂等可重复；命名与 manifest 中 `fixture:` 引用一致（待模块连库后填充实体 SQL）。
```

- [ ] **Step 6: 追加仓库根 `.gitignore`**

在 `/Users/csw/gitproject/console/.gitignore` 末尾追加：
```gitignore

# test artifacts
tests/reports/*
!tests/reports/.gitkeep
tests/frontend/screenshots/*
!tests/frontend/screenshots/.gitkeep
apps/admin-web/playwright-report/
apps/admin-web/test-results/
```

- [ ] **Step 7: 提交**

```bash
git add tests/ .gitignore
git commit -m "test: scaffold tests/ tree, manifest schema, fixtures layout"
```

---

### Task 2: Postgres 依赖（docker compose）+ 脚本公共库

**Files:**
- Create: `docker-compose.yml`, `scripts/regression/lib.sh`

- [ ] **Step 1: 写 `docker-compose.yml`**

```yaml
services:
  postgres:
    image: postgres:16-alpine
    container_name: console-test-pg
    environment:
      POSTGRES_USER: admin
      POSTGRES_PASSWORD: admin
      POSTGRES_DB: admin_console
    ports:
      - "55432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U admin -d admin_console"]
      interval: 2s
      timeout: 3s
      retries: 30
```

- [ ] **Step 2: 写 `scripts/regression/lib.sh`**

```bash
#!/usr/bin/env sh
# shellcheck shell=sh
set -eu

log()  { printf '\033[1;34m[regression]\033[0m %s\n' "$1"; }
warn() { printf '\033[1;33m[regression]\033[0m %s\n' "$1"; }
err()  { printf '\033[1;31m[regression]\033[0m %s\n' "$1" >&2; }

repo_root() {
  d=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
  while [ "$d" != "/" ]; do
    if [ -d "$d/tests/backend/scenarios" ]; then printf '%s\n' "$d"; return 0; fi
    d=$(dirname -- "$d")
  done
  err "repo root not found (missing tests/backend/scenarios)"; return 1
}

# DB 连接（与 docker-compose.yml 一致）
export PGHOST="${PGHOST:-127.0.0.1}"
export PGPORT="${PGPORT:-55432}"
export PGUSER="${PGUSER:-admin}"
export PGPASSWORD="${PGPASSWORD:-admin}"
export PGDATABASE="${PGDATABASE:-admin_console}"
export DATABASE_URL="${DATABASE_URL:-postgres://admin:admin@127.0.0.1:55432/admin_console?sslmode=disable}"

wait_for_pg() {
  log "waiting for postgres at ${PGHOST}:${PGPORT} ..."
  i=0
  while [ "$i" -lt 60 ]; do
    if docker compose exec -T postgres pg_isready -U "$PGUSER" -d "$PGDATABASE" >/dev/null 2>&1; then
      log "postgres ready"; return 0
    fi
    i=$((i+1)); sleep 1
  done
  err "postgres not ready after 60s"; return 1
}
```

- [ ] **Step 3: 让脚本可执行**

Run: `chmod +x scripts/regression/lib.sh`
Expected: 无输出。

- [ ] **Step 4: 验证 compose 起停（需本机 docker）**

Run:
```bash
cd /Users/csw/gitproject/console
docker compose up -d postgres
sh -c '. scripts/regression/lib.sh; wait_for_pg'
docker compose down
```
Expected: 打印 `postgres ready`；`down` 正常清理。若本机无 docker，记录跳过原因，后续 CI/有 docker 环境再验。

- [ ] **Step 5: 提交**

```bash
git add docker-compose.yml scripts/regression/lib.sh
git commit -m "chore(test): add postgres compose and regression shell lib"
```

---

### Task 3: 迁移 + seed 脚本（集成测试地基）

**Files:**
- Create: `scripts/regression/db.sh`

- [ ] **Step 1: 写 `scripts/regression/db.sh`**

```bash
#!/usr/bin/env sh
# shellcheck shell=sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$DIR/lib.sh"
ROOT=$(repo_root)
MIGRATIONS="$ROOT/services/admin-api/migrations"

# golang-migrate：本机需安装 `migrate`（brew install golang-migrate）
if ! command -v migrate >/dev/null 2>&1; then
  err "golang-migrate 'migrate' not installed; see https://github.com/golang-migrate/migrate"
  exit 1
fi

log "migrate up ($MIGRATIONS)"
migrate -path "$MIGRATIONS" -database "$DATABASE_URL" up

# seed：000002 已是 seed 迁移；额外 fixtures（如有）按 env 灌入
for f in "$ROOT"/tests/fixtures/common/*.sql; do
  [ -e "$f" ] || continue
  log "seed common: $(basename "$f")"
  docker compose exec -T postgres psql -U "$PGUSER" -d "$PGDATABASE" < "$f"
done
log "db ready (migrated + seeded)"
```

- [ ] **Step 2: 可执行**

Run: `chmod +x scripts/regression/db.sh`

- [ ] **Step 3: 验证 migrate up/down（需 docker + migrate）**

Run:
```bash
cd /Users/csw/gitproject/console
docker compose up -d postgres
sh -c '. scripts/regression/lib.sh; wait_for_pg'
sh scripts/regression/db.sh
migrate -path services/admin-api/migrations -database "postgres://admin:admin@127.0.0.1:55432/admin_console?sslmode=disable" down -all
docker compose down
```
Expected: `db ready (migrated + seeded)`；`down -all` 回滚成功。无 docker/migrate 时记录跳过。

- [ ] **Step 4: 提交**

```bash
git add scripts/regression/db.sh
git commit -m "chore(test): add migrate+seed db bootstrap script"
```

---

## Phase 1 — 后端 scenario harness（Go，TDD）

> harness 代码在 module 内（可 import `httpserver`），manifest 数据在顶层 `tests/`。包名 `scenario`，路径 `services/admin-api/internal/testkit/scenario/`。下列 `go test` 命令均在 `services/admin-api/` 目录执行。

### Task 4: manifest 结构 + 加载器（含 yaml 依赖）

**Files:**
- Modify: `services/admin-api/go.mod`, `services/admin-api/go.sum`
- Create: `services/admin-api/internal/testkit/scenario/manifest.go`
- Test: `services/admin-api/internal/testkit/scenario/manifest_test.go`

- [ ] **Step 1: 加 yaml 依赖**

Run（在 `services/admin-api/`）:
```bash
cd /Users/csw/gitproject/console/services/admin-api
go get gopkg.in/yaml.v3@v3.0.1
```
Expected: `go.mod` 出现 `require gopkg.in/yaml.v3 v3.0.1`，生成/更新 `go.sum`。

- [ ] **Step 2: 写失败测试 `manifest_test.go`**

```go
package scenario

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadManifestParsesCases(t *testing.T) {
	dir := t.TempDir()
	yaml := `module: smoke
cases:
  - name: healthz_ok
    dimension: S1
    request:
      method: GET
      path: /healthz
    expect:
      status: 200
      jsonContains:
        status: ok
`
	path := filepath.Join(dir, "smoke.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if m.Module != "smoke" || len(m.Cases) != 1 {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	c := m.Cases[0]
	if c.Name != "healthz_ok" || c.Request.Method != "GET" || c.Request.Path != "/healthz" {
		t.Fatalf("unexpected case: %+v", c)
	}
	if c.Expect.Status != 200 || c.Expect.JSONContains["status"] != "ok" {
		t.Fatalf("unexpected expect: %+v", c.Expect)
	}
}

func TestLoadManifestRejectsEmptyCaseName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("module: x\ncases:\n  - request:\n      method: GET\n      path: /\n    expect:\n      status: 200\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected validation error for empty case name")
	}
}
```

- [ ] **Step 3: 运行确认失败**

Run: `go test ./internal/testkit/scenario/ -run TestLoadManifest -v`
Expected: FAIL（`LoadManifest` / 类型未定义，编译错误）。

- [ ] **Step 4: 写 `manifest.go`**

```go
// Package scenario runs backend API scenario manifests in-process via httptest.
package scenario

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Module string `yaml:"module"`
	Cases  []Case `yaml:"cases"`
}

type Case struct {
	Name      string  `yaml:"name"`
	Dimension string  `yaml:"dimension"`
	Request   Request `yaml:"request"`
	Expect    Expect  `yaml:"expect"`
}

type Request struct {
	Method  string            `yaml:"method"`
	Path    string            `yaml:"path"`
	Headers map[string]string `yaml:"headers"`
	Body    map[string]any    `yaml:"body"`
}

type Expect struct {
	Status       int            `yaml:"status"`
	JSONContains map[string]any `yaml:"jsonContains"`
}

func LoadManifest(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if m.Module == "" {
		return Manifest{}, fmt.Errorf("%s: missing module", path)
	}
	for i, c := range m.Cases {
		if c.Name == "" {
			return Manifest{}, fmt.Errorf("%s: case[%d] missing name", path, i)
		}
		if c.Request.Method == "" || c.Request.Path == "" {
			return Manifest{}, fmt.Errorf("%s: case %q missing request method/path", path, c.Name)
		}
		if c.Expect.Status == 0 {
			return Manifest{}, fmt.Errorf("%s: case %q missing expect.status", path, c.Name)
		}
	}
	return m, nil
}
```

- [ ] **Step 5: 运行确认通过**

Run: `go test ./internal/testkit/scenario/ -run TestLoadManifest -v`
Expected: PASS（2 个用例）。

- [ ] **Step 6: 提交**

```bash
git add go.mod go.sum internal/testkit/scenario/manifest.go internal/testkit/scenario/manifest_test.go
git commit -m "test: add scenario manifest schema and loader"
```

---

### Task 5: 点路径取值 + 进程内执行器（runner）

**Files:**
- Create: `services/admin-api/internal/testkit/scenario/jsonpath.go`, `services/admin-api/internal/testkit/scenario/runner.go`
- Test: `services/admin-api/internal/testkit/scenario/runner_test.go`

- [ ] **Step 1: 写失败测试 `runner_test.go`**

```go
package scenario

import (
	"testing"

	"github.com/csw/console/services/admin-api/internal/infra/config"
	"github.com/csw/console/services/admin-api/internal/transport/httpserver"
)

func testServer() *httpServerHandler {
	srv := httpserver.New(config.Config{AppName: "admin-api", Environment: "test", HTTPAddress: ":0"})
	return &httpServerHandler{handler: srv.Handler}
}

func TestRunCaseHealthzPasses(t *testing.T) {
	res := RunCase(testServer().handler, Case{
		Name:    "healthz_ok",
		Request: Request{Method: "GET", Path: "/healthz"},
		Expect:  Expect{Status: 200, JSONContains: map[string]any{"status": "ok"}},
	})
	if !res.Passed {
		t.Fatalf("expected pass, got: %s", res.Message)
	}
}

func TestRunCaseDetectsStatusMismatch(t *testing.T) {
	res := RunCase(testServer().handler, Case{
		Name:    "healthz_wrong",
		Request: Request{Method: "GET", Path: "/healthz"},
		Expect:  Expect{Status: 500},
	})
	if res.Passed {
		t.Fatal("expected failure on status mismatch")
	}
}

func TestRunCaseSyncPreviewRejectsUnknownSection(t *testing.T) {
	res := RunCase(testServer().handler, Case{
		Name: "sync_preview_unknown_section",
		Request: Request{
			Method: "POST",
			Path:   "/api/admin/games/100001/sync/preview",
			Body:   map[string]any{"selected_sections": []any{"marketing"}},
		},
		Expect: Expect{Status: 400},
	})
	if !res.Passed {
		t.Fatalf("expected 400 for unknown section, got: %s", res.Message)
	}
}

type httpServerHandler struct{ handler interface{ ServeHTTP(w any, r any) } }
```

> 注：`http.Handler` 不能用 `any` 占位，下方实现以真实 `http.Handler` 为准；上面 `httpServerHandler` 仅为说明，实际测试请用 `srv.Handler`（`http.Handler`）直接传给 `RunCase`。Step 修正见下。

- [ ] **Step 2: 修正测试为真实 `http.Handler`（替换上文占位）**

`runner_test.go` 最终版：
```go
package scenario

import (
	"net/http"
	"testing"

	"github.com/csw/console/services/admin-api/internal/infra/config"
	"github.com/csw/console/services/admin-api/internal/transport/httpserver"
)

func testHandler() http.Handler {
	return httpserver.New(config.Config{AppName: "admin-api", Environment: "test", HTTPAddress: ":0"}).Handler
}

func TestRunCaseHealthzPasses(t *testing.T) {
	res := RunCase(testHandler(), Case{
		Name:    "healthz_ok",
		Request: Request{Method: "GET", Path: "/healthz"},
		Expect:  Expect{Status: 200, JSONContains: map[string]any{"status": "ok"}},
	})
	if !res.Passed {
		t.Fatalf("expected pass, got: %s", res.Message)
	}
}

func TestRunCaseDetectsStatusMismatch(t *testing.T) {
	res := RunCase(testHandler(), Case{
		Name:    "healthz_wrong",
		Request: Request{Method: "GET", Path: "/healthz"},
		Expect:  Expect{Status: 500},
	})
	if res.Passed {
		t.Fatal("expected failure on status mismatch")
	}
}

func TestRunCaseSyncPreviewRejectsUnknownSection(t *testing.T) {
	res := RunCase(testHandler(), Case{
		Name: "sync_preview_unknown_section",
		Request: Request{
			Method: "POST",
			Path:   "/api/admin/games/100001/sync/preview",
			Body:   map[string]any{"selected_sections": []any{"marketing"}},
		},
		Expect: Expect{Status: 400},
	})
	if !res.Passed {
		t.Fatalf("expected 400 for unknown section, got: %s", res.Message)
	}
}
```

- [ ] **Step 3: 运行确认失败**

Run: `go test ./internal/testkit/scenario/ -run TestRunCase -v`
Expected: FAIL（`RunCase` / `CaseResult` 未定义）。

- [ ] **Step 4: 写 `jsonpath.go`**

```go
package scenario

import (
	"fmt"
	"strconv"
	"strings"
)

// lookup 按点路径在已解码 JSON（map/slice）中取值；支持数字下标，如 items.0.id。
func lookup(root any, path string) (any, bool) {
	cur := root
	for _, seg := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, false
			}
			cur = node[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// equalScalar 以字符串化方式比较，吸收 JSON number(float64) 与 yaml int 的差异。
func equalScalar(got, want any) bool {
	return fmt.Sprintf("%v", got) == fmt.Sprintf("%v", want)
}
```

- [ ] **Step 5: 写 `runner.go`**

```go
package scenario

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"
)

type CaseResult struct {
	Name      string
	Dimension string
	Passed    bool
	Message   string
	Status    int
	Duration  time.Duration
}

// RunCase 在进程内对 http.Handler 执行一个 case 并校验断言。
func RunCase(handler http.Handler, c Case) CaseResult {
	start := time.Now()
	res := CaseResult{Name: c.Name, Dimension: c.Dimension}

	var body *bytes.Reader
	if c.Request.Body != nil {
		raw, err := json.Marshal(c.Request.Body)
		if err != nil {
			res.Message = fmt.Sprintf("marshal body: %v", err)
			res.Duration = time.Since(start)
			return res
		}
		body = bytes.NewReader(raw)
	} else {
		body = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(c.Request.Method, c.Request.Path, body)
	if c.Request.Body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range c.Request.Headers {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	res.Status = rec.Code
	res.Duration = time.Since(start)

	if rec.Code != c.Expect.Status {
		res.Message = fmt.Sprintf("status: want %d, got %d (body: %s)", c.Expect.Status, rec.Code, truncate(rec.Body.String()))
		return res
	}

	if len(c.Expect.JSONContains) > 0 {
		var decoded any
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			res.Message = fmt.Sprintf("body not JSON: %v", err)
			return res
		}
		for path, want := range c.Expect.JSONContains {
			got, ok := lookup(decoded, path)
			if !ok {
				res.Message = fmt.Sprintf("json path %q not found", path)
				return res
			}
			if !equalScalar(got, want) {
				res.Message = fmt.Sprintf("json path %q: want %v, got %v", path, want, got)
				return res
			}
		}
	}

	res.Passed = true
	return res
}

func truncate(s string) string {
	const max = 240
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
```

- [ ] **Step 6: 运行确认通过**

Run: `go test ./internal/testkit/scenario/ -run TestRunCase -v`
Expected: PASS（3 个用例）。

- [ ] **Step 7: 提交**

```bash
git add internal/testkit/scenario/jsonpath.go internal/testkit/scenario/runner.go internal/testkit/scenario/runner_test.go
git commit -m "test: add in-process scenario runner with jsonpath assertions"
```

---

### Task 6: 仓库根定位 + manifest 入口测试 + smoke.yaml

**Files:**
- Create: `services/admin-api/internal/testkit/scenario/repo_root.go`, `services/admin-api/internal/testkit/scenario/scenarios_test.go`
- Create: `tests/backend/scenarios/smoke.yaml`
- Test: `services/admin-api/internal/testkit/scenario/repo_root_test.go`

- [ ] **Step 1: 写 `repo_root.go`**

```go
package scenario

import (
	"os"
	"path/filepath"
)

// findRepoRoot 从给定目录向上查找含 tests/backend/scenarios 的目录。
func findRepoRoot(start string) (string, bool) {
	dir := start
	for {
		if fi, err := os.Stat(filepath.Join(dir, "tests", "backend", "scenarios")); err == nil && fi.IsDir() {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// ScenariosDir 返回 tests/backend/scenarios 绝对路径。
func ScenariosDir() (string, bool) {
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	root, ok := findRepoRoot(wd)
	if !ok {
		return "", false
	}
	return filepath.Join(root, "tests", "backend", "scenarios"), true
}
```

- [ ] **Step 2: 写 `repo_root_test.go`**

```go
package scenario

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRepoRootWalksUp(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "services", "admin-api", "internal", "testkit", "scenario")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "tests", "backend", "scenarios"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, ok := findRepoRoot(deep)
	if !ok || got != root {
		t.Fatalf("want %s, got %s ok=%v", root, got, ok)
	}
}
```

- [ ] **Step 3: 运行确认通过**

Run: `go test ./internal/testkit/scenario/ -run TestFindRepoRoot -v`
Expected: PASS。

- [ ] **Step 4: 写 `tests/backend/scenarios/smoke.yaml`（worked example，覆盖现有真实端点）**

```yaml
module: smoke
cases:
  - name: healthz_ok
    dimension: S1
    request:
      method: GET
      path: /healthz
    expect:
      status: 200
      jsonContains:
        status: ok

  - name: me_returns_identity
    dimension: S1
    request:
      method: GET
      path: /api/admin/me
    expect:
      status: 200
      jsonContains:
        displayName: Admin

  - name: games_list_ok
    dimension: S1
    request:
      method: GET
      path: /api/admin/games
    expect:
      status: 200

  - name: sync_preview_valid_section_ok
    dimension: S1
    request:
      method: POST
      path: /api/admin/games/100001/sync/preview
      body:
        selected_sections: ["channels"]
    expect:
      status: 200
      jsonContains:
        targetEnv: production

  - name: sync_preview_unknown_section_rejected
    dimension: S4
    request:
      method: POST
      path: /api/admin/games/100001/sync/preview
      body:
        selected_sections: ["marketing"]
    expect:
      status: 400
```

> 校验前确认 `sync.Preview` 输出含 `targetEnv: production`（见 `httpserver.go` scaffold Preview 与 `domain/sync` 的 JSON tag）。若实际 JSON key 为 `target_env`，按真实输出改 `jsonContains` 的点路径。

- [ ] **Step 5: 写入口测试 `scenarios_test.go`（加载全部 manifest 并运行）**

```go
package scenario

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/csw/console/services/admin-api/internal/infra/config"
	"github.com/csw/console/services/admin-api/internal/transport/httpserver"
)

func TestScenarioManifests(t *testing.T) {
	dir, ok := ScenariosDir()
	if !ok {
		t.Skip("tests/backend/scenarios not found from cwd")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Skip("no scenario manifests")
	}

	handler := httpserver.New(config.Config{AppName: "admin-api", Environment: "test", HTTPAddress: ":0"}).Handler

	for _, f := range files {
		f := f
		m, err := LoadManifest(f)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
		t.Run(m.Module, func(t *testing.T) {
			for _, c := range m.Cases {
				c := c
				t.Run(c.Name, func(t *testing.T) {
					res := RunCase(handler, c)
					if !res.Passed {
						t.Errorf("[%s] %s", res.Dimension, res.Message)
					}
				})
			}
		})
	}
	_ = os.Getenv // keep imports stable if trimmed
}
```

- [ ] **Step 6: 运行确认全部通过**

Run（在 `services/admin-api/`）: `go test ./internal/testkit/scenario/ -run TestScenarioManifests -v`
Expected: PASS，子测试 `smoke/healthz_ok`、`smoke/me_returns_identity`、`smoke/games_list_ok`、`smoke/sync_preview_valid_section_ok`、`smoke/sync_preview_unknown_section_rejected` 均 PASS。

- [ ] **Step 7: 全量回归确认未破坏既有测试**

Run: `go test ./...`
Expected: 全 PASS（含既有 domain/app/transport 测试）。

- [ ] **Step 8: 提交**

```bash
git add internal/testkit/scenario/repo_root.go internal/testkit/scenario/repo_root_test.go internal/testkit/scenario/scenarios_test.go ../../tests/backend/scenarios/smoke.yaml
  git commit -m "test: wire scenario manifest entry test and smoke manifest"
```

---

## Phase 2 — 前端 Playwright e2e（真实截图 / trace / HTML report）

> 命令在 `apps/admin-web/` 下执行。包管理用 `pnpm`（仓库有 `.pnpm-store`）。

### Task 7: 安装 Playwright + 配置

**Files:**
- Modify: `apps/admin-web/package.json`
- Create: `apps/admin-web/playwright.config.ts`

- [ ] **Step 1: 安装依赖与浏览器**

Run（在 `apps/admin-web/`）:
```bash
cd /Users/csw/gitproject/console/apps/admin-web
pnpm add -D @playwright/test
pnpm exec playwright install chromium
```
Expected: `@playwright/test` 进入 devDependencies；chromium 下载完成。

- [ ] **Step 2: 加 package.json 脚本**

把 `scripts` 改为：
```json
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "test": "vitest run",
    "preview": "vite preview",
    "e2e": "playwright test",
    "e2e:update": "playwright test --update-snapshots"
  },
```

- [ ] **Step 3: 写 `playwright.config.ts`**

```ts
import { defineConfig, devices } from "@playwright/test";

const PORT = 5173;

export default defineConfig({
  testDir: "../../tests/frontend/e2e",
  outputDir: "../../tests/reports/playwright-artifacts",
  snapshotDir: "../../tests/frontend/visual-baseline",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: [
    ["list"],
    ["html", { outputFolder: "../../tests/reports/playwright-html", open: "never" }],
    ["json", { outputFile: "../../tests/reports/playwright-results.json" }],
  ],
  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
  ],
  webServer: {
    command: "pnpm dev",
    url: `http://127.0.0.1:${PORT}`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
```

- [ ] **Step 4: 提交**

```bash
git add package.json playwright.config.ts pnpm-lock.yaml
git commit -m "test: add playwright config and e2e scripts"
```

---

### Task 8: 前端 smoke e2e（加载首页 + 截图）

**Files:**
- Create: `tests/frontend/e2e/smoke.spec.ts`

- [ ] **Step 1: 确认首页可加载（手动起服务核对）**

Run（在 `apps/admin-web/`）:
```bash
pnpm dev &
sleep 4
curl -sS -o /dev/null -w "%{http_code}\n" http://127.0.0.1:5173/
kill %1
```
Expected: 打印 `200`。据此确认下一步断言用的根路由可达；若应用首屏有可见标题/导航，记录其文本用于断言。

- [ ] **Step 2: 写 `tests/frontend/e2e/smoke.spec.ts`**

```ts
import { test, expect } from "@playwright/test";

test("app shell loads and renders", async ({ page }) => {
  const response = await page.goto("/");
  expect(response?.status()).toBeLessThan(400);

  // 应用挂载点应出现内容（Vue 挂在 #app）
  await expect(page.locator("#app")).toBeVisible();

  // 采集真实页面截图到统一产物目录
  await page.screenshot({
    path: "../../tests/frontend/screenshots/app-shell.png",
    fullPage: true,
  });
});

test("app shell visual baseline", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator("#app")).toBeVisible();
  // 首次运行需 e2e:update 生成基线
  await expect(page).toHaveScreenshot("app-shell.png", { maxDiffPixelRatio: 0.02 });
});
```

> 若 `index.html` 的挂载节点不是 `#app`，按真实 id 改选择器（核对 `apps/admin-web/index.html` 与 `src/main.ts`）。

- [ ] **Step 3: 生成视觉基线并运行**

Run（在 `apps/admin-web/`）:
```bash
pnpm e2e:update   # 首次生成 visual-baseline 快照
pnpm e2e
```
Expected: 两个 e2e PASS；`tests/frontend/screenshots/app-shell.png` 生成；`tests/reports/playwright-html/` 生成 HTML 报告；`tests/frontend/visual-baseline/` 出现基线快照。

- [ ] **Step 4: 确认既有 vitest 未受影响**

Run: `pnpm test`
Expected: 既有 3 个 `*.spec.ts`（jsdom）全 PASS。

- [ ] **Step 5: 提交（含视觉基线，忽略截图/报告产物）**

```bash
cd /Users/csw/gitproject/console
git add tests/frontend/e2e/smoke.spec.ts tests/frontend/visual-baseline/
git commit -m "test: add frontend smoke e2e with screenshot and visual baseline"
```

---

## Phase 3 — 统一回归编排

### Task 9: 后端 / 前端 回归子脚本

**Files:**
- Create: `scripts/regression/backend.sh`, `scripts/regression/frontend.sh`

- [ ] **Step 1: 写 `scripts/regression/backend.sh`**

```bash
#!/usr/bin/env sh
# shellcheck shell=sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$DIR/lib.sh"
ROOT=$(repo_root)
REPORTS="$ROOT/tests/reports"
mkdir -p "$REPORTS"

log "backend: go test ./... (+ scenario harness)"
cd "$ROOT/services/admin-api"

# 统一跑单元/集成/transport + scenario 入口测试，输出 json 行供汇总
if go test ./... -count=1 -json > "$REPORTS/backend-go-test.json" 2> "$REPORTS/backend-go-test.err"; then
  log "backend tests PASS"
else
  err "backend tests FAILED (see $REPORTS/backend-go-test.*)"
  exit 1
fi
```

- [ ] **Step 2: 写 `scripts/regression/frontend.sh`**

```bash
#!/usr/bin/env sh
# shellcheck shell=sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$DIR/lib.sh"
ROOT=$(repo_root)
cd "$ROOT/apps/admin-web"

log "frontend: vitest"
pnpm test

log "frontend: playwright e2e"
# 更新基线请单独跑 pnpm e2e:update
pnpm e2e
```

- [ ] **Step 3: 可执行 + 验证后端子脚本**

Run:
```bash
cd /Users/csw/gitproject/console
chmod +x scripts/regression/backend.sh scripts/regression/frontend.sh
sh scripts/regression/backend.sh
```
Expected: 打印 `backend tests PASS`，生成 `tests/reports/backend-go-test.json`。

- [ ] **Step 4: 提交**

```bash
git add scripts/regression/backend.sh scripts/regression/frontend.sh
git commit -m "chore(test): add backend and frontend regression sub-scripts"
```

---

### Task 10: 汇总脚本 + 全链路入口 `run.sh`

**Files:**
- Create: `scripts/regression/summarize.sh`, `scripts/regression/run.sh`

- [ ] **Step 1: 写 `scripts/regression/summarize.sh`**

```bash
#!/usr/bin/env sh
# shellcheck shell=sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$DIR/lib.sh"
ROOT=$(repo_root)
REPORTS="$ROOT/tests/reports"
GO_JSON="$REPORTS/backend-go-test.json"

pass=0; fail=0
if [ -f "$GO_JSON" ]; then
  pass=$(grep -c '"Action":"pass"' "$GO_JSON" 2>/dev/null || echo 0)
  fail=$(grep -c '"Action":"fail"' "$GO_JSON" 2>/dev/null || echo 0)
fi

ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
printf '{"generatedAt":"%s","backend":{"pass":%s,"fail":%s}}\n' "$ts" "$pass" "$fail" > "$REPORTS/summary.json"
{
  echo "# 回归汇总 ($ts)"
  echo
  echo "## 后端 (go test)"
  echo "- pass: $pass"
  echo "- fail: $fail"
  echo
  echo "## 前端 (Playwright)"
  echo "- HTML 报告: tests/reports/playwright-html/index.html"
  echo "- 结果 JSON: tests/reports/playwright-results.json"
  echo "- 截图: tests/frontend/screenshots/"
} > "$REPORTS/summary.md"

log "summary written: $REPORTS/summary.md"
[ "$fail" -eq 0 ]
```

- [ ] **Step 2: 写 `scripts/regression/run.sh`（全链路）**

```bash
#!/usr/bin/env sh
# shellcheck shell=sh
set -eu
DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
. "$DIR/lib.sh"
ROOT=$(repo_root)

WITH_DB="${WITH_DB:-1}"   # WITH_DB=0 跳过 PG/迁移（smoke 走进程内 httptest 不需库）
MODULE="${1:-all}"        # 预留：按模块过滤（模块场景落地后用）

log "=== regression start (module=$MODULE, with_db=$WITH_DB) ==="

if [ "$WITH_DB" = "1" ]; then
  log "starting postgres"
  (cd "$ROOT" && docker compose up -d postgres)
  wait_for_pg
  sh "$DIR/db.sh"
else
  warn "WITH_DB=0: skipping postgres/migrate/seed (in-process scenarios only)"
fi

backend_status=0; frontend_status=0
sh "$DIR/backend.sh"  || backend_status=$?
sh "$DIR/frontend.sh" || frontend_status=$?

if [ "$WITH_DB" = "1" ]; then
  log "stopping postgres"
  (cd "$ROOT" && docker compose down)
fi

sh "$DIR/summarize.sh" || true

log "=== regression done (backend=$backend_status frontend=$frontend_status) ==="
[ "$backend_status" -eq 0 ] && [ "$frontend_status" -eq 0 ]
```

- [ ] **Step 3: 可执行 + 验证（不依赖 docker 的快路径）**

Run:
```bash
cd /Users/csw/gitproject/console
chmod +x scripts/regression/summarize.sh scripts/regression/run.sh
WITH_DB=0 sh scripts/regression/run.sh
```
Expected: 后端通过、前端 vitest+e2e 通过、生成 `tests/reports/summary.md` 与 `summary.json`，末行 `regression done (backend=0 frontend=0)`。

- [ ] **Step 4: 提交**

```bash
git add scripts/regression/summarize.sh scripts/regression/run.sh
git commit -m "chore(test): add unified regression entrypoint and summary"
```

---

### Task 11: 文档接入与运行说明

**Files:**
- Modify: `docs/architecture/v2/03-testing.md`（补「实现状态」）
- Modify: `README.md`（仓库根，补运行说明）

- [ ] **Step 1: 在 `03-testing.md` 末尾追加「实现状态」小节**

```markdown
## 实现状态（harness 已落地，模块场景增量补充）

- 已落地：`tests/` 目录树、`docker-compose.yml`(Postgres)、`scripts/regression/*`（migrate/seed/backend/frontend/run/summarize）、后端 scenario harness（`services/admin-api/internal/testkit/scenario`，进程内 httptest + manifest）、前端 Playwright（截图/trace/HTML report/视觉基线）、smoke 切片。
- 增量补充：各模块 S1–S10 完整场景 YAML 与 `expect.db` 断言，随对应模块连库实现后按 `tests/backend/scenarios/README.md` 的 schema 追加；`tests/fixtures/{common,sandbox,production}` 按 env 灌入实体样本。
- 运行：`sh scripts/regression/run.sh`（全量，需 docker+migrate）；快路径 `WITH_DB=0 sh scripts/regression/run.sh`（仅进程内场景 + 前端）。
```

- [ ] **Step 2: 在仓库根 `README.md` 增补「测试与回归」段落**

```markdown
## 测试与回归

- 全量回归：`sh scripts/regression/run.sh`（启动 Postgres → 迁移/seed → 后端 `go test ./...` + scenario harness → 前端 vitest + Playwright → 汇总 `tests/reports/summary.md`）。
- 快路径（无需 docker）：`WITH_DB=0 sh scripts/regression/run.sh`。
- 仅后端：`sh scripts/regression/backend.sh`；仅前端：`sh scripts/regression/frontend.sh`。
- 更新前端视觉基线：`cd apps/admin-web && pnpm e2e:update`。
- 测试体系契约见 `docs/architecture/v2/03-testing.md`。
```

- [ ] **Step 3: 全链路自检**

Run:
```bash
cd /Users/csw/gitproject/console
WITH_DB=0 sh scripts/regression/run.sh
```
Expected: 通过，summary 生成，无回归。

- [ ] **Step 4: 提交**

```bash
git add docs/architecture/v2/03-testing.md README.md
git commit -m "docs: document testing harness implementation status and regression usage"
```

---

## Self-Review（写完计划后的自检结果）

**1. Spec 覆盖：**
- 03-testing「五层测试」→ L1/L2/L3 走 `go test ./...`（Task 9）；L3 接口场景走 harness（Task 4–6）；L4 vitest（既有，Task 8 校验不破坏）；L5 Playwright（Task 7–8）。✅
- 「目录树」→ Task 1。✅
- 「接口场景矩阵 / manifest」→ Task 4–6（schema + 入口 + worked example）。✅
- 「前端 Playwright：截图/trace/HTML report/视觉回归」→ Task 7–8。✅
- 「统一回归入口（起依赖→迁移→seed→后端全场景→前端 e2e→summary）」→ Task 2/3/9/10。✅
- 「fixtures 约定（按 env）」→ Task 1 + db.sh seed 钩子（Task 3）。✅
- **Gap（有意）**：逐模块 S1–S10 完整 YAML 与 DB 断言依赖模块实现，已在「范围」与「实现状态」标注为增量项，非本计划交付。

**2. Placeholder 扫描：** 无 TBD/TODO；脚本/配置/Go 均为完整可运行代码。Task 5 中先给出占位测试再「Step 2 修正为真实 http.Handler」是刻意演示——最终落库以 Step 2 版本为准（执行者应直接采用 Step 2 的 `runner_test.go`）。

**3. 类型一致性：** `LoadManifest`/`Manifest`/`Case`/`Request`/`Expect`/`RunCase`/`CaseResult`/`lookup`/`equalScalar`/`findRepoRoot`/`ScenariosDir` 在各 Task 间签名一致；YAML 字段（`module/cases/name/dimension/request{method,path,headers,body}/expect{status,jsonContains}`）与 `manifest.go` tag 一致；请求体字段 `selected_sections` 与后端 dto 一致；端口（PG `55432`、前端 `5173`）跨 compose/lib/playwright 一致。✅

**待执行者注意：** harness 断言里 `sync/preview` 的 `targetEnv` 与前端挂载点 `#app` 两处，执行前用真实输出/`index.html` 核对，按需微调点路径与选择器（计划内已标注）。

