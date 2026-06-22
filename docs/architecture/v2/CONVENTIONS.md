---
id: conventions
code: "conv"
title: 文档规范（命名 / front-matter / 拆分 / 关联）
status: target
code_paths: []
depends_on: []
impacts: []
children: []
---

# 文档规范（CONVENTIONS）

> 本文件定义 `docs/architecture/v2` 文档集的**结构规范**：目录命名、front-matter、拆分规则、模块关联维护、文档↔代码对齐。所有新增/修改文档须遵循。

---

## 1. 目录与命名

- 顶层基座（单文件）：`00-common.md`（公共契约）、`01-structure.md`（项目结构）、`02-operation-flow.md`（操作主线）。
- 模块：`modules/NN-英文短名/`，模块总纲固定为 `README.md`。
- 子文档：`modules/NN-英文短名/英文短名.md`（命名优先对齐代码文件名，如 `visibility.md` ↔ `domain/channel/visibility.go`）。
- 命名用 **数字前缀 + 英文 kebab 短名**，英文名尽量对齐代码 domain（`auth/game/channel/product/cashier/payment/snapshot/sync` 等）。
- 参考文件：`schema-reference.md`（DB 速查）、本文件 `CONVENTIONS.md`。

---

## 2. front-matter（每个文档头部必带）

### 2.1 模块总纲 / 顶层文档
```yaml
---
id: channel                  # 稳定唯一标识（kebab，对齐代码 domain），重命名/移动不变
code: "12"                   # 排序编号（与文件夹前缀一致）
title: 渠道与渠道实例
status: target               # target | draft | deprecated
code_paths:                  # 文档 ↔ 代码对齐（目录或文件）
  - services/admin-api/internal/domain/channel
  - apps/admin-web/src/views/channels
depends_on: [game, common]   # 我依赖的上游（改它们可能影响我）
impacts: [account-auth, channel-login, feature-plugin, product, payment, snapshot, sync]  # 改我时需联动核对的下游
children: []                 # 已拆出的子文档 id
---
```

### 2.2 子文档
```yaml
---
id: channel/visibility       # 路径式 <module>/<sub>
parent: channel
title: 可见性与兼容性规则
code_paths: [services/admin-api/internal/domain/channel/visibility.go]
depends_on: [common]
impacts: [snapshot/merge, sync/diff]   # 可精确到其它模块的子文档
---
```

### 2.3 字段语义
| 字段 | 含义 |
| --- | --- |
| `id` | 稳定标识，是所有关联引用的目标。模块用 `name`，子文档用 `name/sub`。 |
| `code` | 排序编号，与文件夹前缀一致；仅用于展示/排序，**不用于引用**。 |
| `depends_on` | 本文档依赖的上游对象。 |
| `impacts` | 修改本文档时需连带核对/修改的下游对象（可为模块或子文档 id）。 |
| `code_paths` | 对应代码目录/文件，体现文档与代码结构对齐。 |
| `parent` / `children` | 子文档↔父模块的树关系。 |

---

## 3. 模块关联维护（核心）

1. **改动前先看 `impacts`**：打开本文档 front-matter 的 `impacts`（含子文档粒度），逐一核对是否需要同步修改对应模块/子文档。
2. **引用用 `id`，不用编号**：正文内引用其它模块一律用稳定 `id`（如「见 `channel`」「见 `snapshot/merge`」）或相对链接，**不要再用「模块 NN」数字**（数字会因重编号漂移）。
3. **反向一致性**：`A.impacts` 含 `B` ⇔ `B.depends_on` 含 `A`。新增/删除关联时两端同步更新。
4. **增删模块/子文档**：同步更新相关文档的 `depends_on`/`impacts` 与父文档 `children`。
5. 后续可加脚本：解析所有 front-matter，校验 `depends_on`/`impacts` 反向一致、`id` 引用不断链、生成关系图。

---

## 4. 文档拆分规则

- **何时拆**：当模块 `README.md` 过长（一个文件难以一眼掌握）或某子主题需独立演进时，把对应章节下沉为子文档。
- **怎么拆**：
  1. 新建 `modules/NN-xxx/子名.md`，迁移正文，补子文档 front-matter（`id`/`parent`/`depends_on`/`impacts`/`code_paths`）。
  2. 在父 `README.md` 的 `children` 登记子文档 id，正文保留**精简索引 + 链接**。
  3. 更新引用本章节的其它文档关联到更细的子文档 id。
- **`00`/`01`/`02` 将来文件夹化**：`id` 不变（仍为 `common`/`structure`/`operation-flow`），原大文件内容下沉为子文档；因 `id` 稳定，所有 `depends_on: [common]` 等引用不断链。拆分 PR 必须同时更新 `children` 与受影响文档关联（这是「与之关联的内容同步联动」的落地方式）。

---

## 5. 文档 ↔ 代码对齐

- 每个模块/子文档用 `code_paths` 指向其对应后端 domain/transport 与前端 views/api 路径。
- 模块英文短名尽量与代码 domain 同名；新建代码 domain 时回填实际路径（如功能插件 `internal/domain/plugin` 落地后核对）。

---

## 6. 编号映射（本轮重编号：旧 → 新）

> 本轮新增 `15-feature-plugin` 并将其后模块顺延。各模块文档正文的交叉引用**已统一改写为稳定 `id`**（见 §3.2）；本表仅作旧编号 ↔ 新编号/`id` 的**历史对照留档**，不再用于正文引用。10–14 未变。

| 旧编号 | 新编号 / id |
| --- | --- |
| 10 | 10 / `auth` |
| 11 | 11 / `game` |
| 12 | 12 / `channel` |
| 13 | 13 / `account-auth` |
| 14 | 14 / `channel-login` |
| （新增） | 15 / `feature-plugin` |
| 15 | 16 / `product` |
| 16 | 17 / `cashier-template` |
| 17 | 18 / `game-cashier` |
| 18 | 19 / `payment` |
| 19 | 20 / `snapshot` |
| 20 | 21 / `sync` |
| 21 | 22 / `audit` |
| 22 | 23 / `dashboard` |

> 迁移状态：各模块 README 正文的「模块 NN」交叉引用已全部改写为稳定 `id`（如 `channel-login`/`snapshot`/`sync`）。后续如再新增/重编号模块，仍遵循 §3.2「引用用 id 不用编号」，本表追加对照即可。
