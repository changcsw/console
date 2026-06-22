# fixtures（按 env 维度）

- `common/` — 平台级、全 env 共享（字典/枚举/currency_specs/channels(region)/模板四件套）。
- `sandbox/` — 同步源样本。
- `production/` — 同步目标基线（用于 sync diff/baseline/nonce 测试）。

约定：seed 用 `ON CONFLICT DO NOTHING` 幂等可重复；命名与 manifest 中 `fixture:` 引用一致（待模块连库后填充实体 SQL）。
