# F-001: Router clean-room rewrite — Design Spec

日期: 2026-06-11 · 状态: 待批准 · 模式: brainstorming [2]（双方案对比后混合选型）

## Goal / Scope

**目标**：将 `router.go` 重写为纯原创实现，消除 gin/httprouter 派生代码带来的开源合规风险（CLAUDE.md Invariant 1），同时保全全部现有路由行为与性能。

**选型决策（已与用户确认）**：
1. 架构 = 方案 A（纯分段 Trie，单一结构）+ 预留方案 B 接缝（静态精确表快路径的明确插入点，benchmark 超阈值时启用）
2. 性能阈值 = 严格同量级 **≤1.3x 基线**：static ≤11ns、param ≤35ns、ManyRoutes ≤1.3x 基线、热路径 0 堆分配。**未达标则必须叠加优化（首选启用 B 接缝）直到达标，不得放宽阈值。**
3. 行为增量 = 修正回溯缺陷：static 分支走死后正确回退到 param 分支（如注册 `/user/new` + `/user/:id`，请求 `/user/newx` 由 404 变为命中 `:id`）。严格增量匹配——只多匹配、不少匹配。

**范围内**：
- 重写 `router.go` 全部内部实现（数据结构、注册、匹配、校验）
- 重写所有 panic 文案（现为 gin 原文）；若有测试钉死原文案，测试同步更新
- 新增测试：回溯语义、段内前缀参数（`/user_:id`）、多级回溯、B 接缝开关
- 重写后刷新 README 的 Benchmarks 小节为实测新数字
- 基线 benchmark 在改动前采集并存档 `.zeus/bench/F-001-baseline.txt`

**范围外**：
- `Engine`/`Context`/`RouterGroup` 的任何改动（调用点零变更）
- trailing-slash 重定向逻辑（留在 Engine 层，`rue.go:153`）
- 新路由语法（regex、可选段等一概不加）
- LICENSE/NOTICE 变更（重写完成后无需归属声明）

**用户可见性**：框架使用者零感知（公开 API 与匹配语义不变，仅回溯场景多匹配）。

**边界用例清单**（实现与测试必须覆盖）：
- 根路由 `/`；根级参数 `/:x`；根级 splat `/*all`
- 段内静态前缀参数 `/user_:id`（gin 兼容形态，现实现支持）
- param 节点多静态后缀共存 `/:id/sync` + `/:id/toggle`（a099372 行为）
- 多级回溯：`/a/b/c`(static) 走死 → 回退 `/a/:x/c`
- `getValue` 的 params 入参为 nil（`rue.go:153` 重定向探测路径在用）
- 注册校验 panic：非 `/` 开头、空 method、空 handlers、重复注册、通配符冲突、段内多通配符、未命名通配符、`*` 非结尾、`*` 前无 `/`
- method 隔离（GET 注册不影响 POST 查找）

## Architecture / Context dependencies

**新数据结构（分段 Trie）**：按 `/` 分段建树，节点代表完整路径段。每节点持有：段文本、静态子节点表 + 首字节索引、参数子节点（支持静态前缀形态）、尾捕获子节点、HandlersChain、注册时完整 pattern。命名全部原创，禁用指纹标识符（见 DoD）。

**匹配算法**：零分配段迭代器（索引扫 `/`，无 strings.Split）；每段优先级 静态 → 前缀参数 → 纯参数 → splat；栈上固定深度回溯帧处理分叉回退；splat 作为各层兜底。

**B 接缝（预留，默认不启用）**：Router 内预留静态精确表字段与查找短路点（注册时分流判断 + getValue 入口短路），结构上隔离为独立小函数，启用与否不影响 Trie 正确性。仅当 benchmark 不达标时启用。

**不变的集成契约**：
- `newRouter() *Router`
- `(*Router).addRoute(method, path string, handlers HandlersChain)` — 唯一调用点 `routergroup.go:36`
- `(*Router).getValue(method, path string, params *Params) (HandlersChain, string, bool)` — 调用点 `rue.go:136`（取 `&c.Params`）、`rue.go:153`（nil params）
- 导出类型 `Params`/`Param` 及 `Get`/`ByName` 方法签名不变（`context.go:27,45,91` 依赖）
- `Router` 结构体名保留；现有死字段 `paramsPool`/`maxParams`（全仓库无使用方）随重写移除

**行为契约来源**：`router_test.go`（13 test + 3 benchmark）、`router_suffix_test.go`（2 test）、全套 Engine/middleware/context 集成测试。测试断言不改（除非钉死 gin 原文 panic 文案）。

## Environment requirements

- Go 1.26.0（现 toolchain，不变）
- **零新增依赖**（Invariant 2）；不引入 benchstat 等工具，基准对比用 `go test -bench -count` 原始输出人工/脚本比对
- 无 CI 变更（项目无 CI）；无运行时/部署影响（纯库代码）
- 基线采集：改动前在当前 HEAD 上运行 benchmark 存档（同机同负载条件下对比）

## Definition of Done delta

在 `.zeus/dod.md` 基线之上，F-001 追加：

- [ ] 合规指纹清零：`! grep -qE 'wildChild|nType|incrementChildPrio|insertChild|longestCommonPrefix|findWildcard|catchAllNode|paramNode|staticNode' router.go`
- [ ] panic 文案与 gin tree.go 无逐字重合（人工核对一次）
- [ ] `go test -bench='Router' -benchmem -run '^$' -count=5`：static ≤11ns、param ≤35ns、ManyRoutes ≤1.3x 基线、static 0 allocs/op
- [ ] 新增回溯测试 + 段内前缀参数测试存在且通过（G2 红绿证据）
- [ ] README Benchmarks 小节已更新为实测数字

## Handoff state requirements

- `.zeus/bench/F-001-baseline.txt`（改动前）与 `.zeus/bench/F-001-after.txt`(改动后) 存档
- `.zeus/features.md` F-001 状态随阶段推进（in-progress → done）
- 会话结束走 `zeus:session-handoff` + `zeus:observability` 出运行日志
- 若 B 接缝被启用，在 handoff 备忘录中记录启用原因与启用前后数字

## 7-gate impact map

- **G1 代码**：router.go 整体替换；新测试文件
- **G2 TDD**：回溯/前缀参数等新行为先红后绿；现有 16 个 router 测试作为回归网
- **G3 验证命令**：build/vet/test/race 全绿
- **G4 DoD**：上述 delta 全部勾选（指纹 grep + 性能阈值是新增硬门）
- **G5 E2E**：完整测试套 + 起一个样例服务手测静态/param/splat/回溯四类路径
- **G6 两阶段评审**：spec 合规性评审 + 代码质量评审（zeus:requesting-code-review）；合并前加 /codex challenge 对抗评审
- **G7 交接**：handoff 备忘录 + 运行日志 + bench 存档

---
*Spec 自审：无占位符；范围/阈值/行为增量与用户三项决策一致；六节齐全；边界用例与测试契约对齐。*
*Footer: 等待用户批准 — 批准后写入 .zeus/state/spec-approved 并进入 zeus:writing-plans。*
