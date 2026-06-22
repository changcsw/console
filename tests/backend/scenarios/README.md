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

> 说明：以上为 smoke 阶段的简化 schema。完整字段（`auth:` 鉴权身份、`fixture:` 数据集引用、`expect.bodyMatch` / `expect.db` / `expect.audit` 等）见 `docs/architecture/v2/03-testing.md` §4.1，待对应模块连库后再扩展 harness 支持。`dimension` 在 smoke 阶段可选，正式场景矩阵建议必填。
