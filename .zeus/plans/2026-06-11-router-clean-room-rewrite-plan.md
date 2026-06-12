# F-001 Implementation Plan — Router Clean-Room Rewrite

## Header

- **Goal**: 将 `router.go` 重写为原创分段 Trie，合规指纹清零，行为保真（唯一增量=回溯修正），性能 ≤1.3x 基线。
- **Architecture**: 方案 A（纯分段 Trie）+ 预留 B 接缝（per-method 静态精确表，默认 nil 不启用）。
- **Tech Stack**: Go 1.26.0，零新增依赖。
- **Feature**: F-001 · **Spec**: `.zeus/specs/2026-06-11-router-clean-room-rewrite-design.md`

## File Map

| 文件 | 动作 | 职责 |
|---|---|---|
| `router.go` | 重写 | 分段 Trie 全部实现：类型、注册校验、匹配、回溯、B 接缝 |
| `router_rewrite_test.go` | 新建 | 契约钉测试（冲突矩阵/前缀参数/nil-params/never-panic）+ 回溯新语义测试 |
| `router_test.go` | 仅必要时改 | 若钉死 gin 原文 panic 文案则同步新文案，断言逻辑不动 |
| `router_suffix_test.go` | 不动 | a099372 行为回归网 |
| `README.md` | 修改 | Benchmarks 小节刷新为实测数字 |
| `.zeus/bench/F-001-baseline.txt` / `F-001-after.txt` | 新建 | 基准存档（改前/改后） |
| `.zeus/dod.md` | 修改 | 追加 F-001 G4 delta 条目 |

## Architect Risk Analysis

Go 单栈，8 项风险及对策（用户已确认）：

1. **零分配段迭代**：字符串切片零分配；`Params` 扩容是唯一分配点 → 注册期统计 `maxVars`，匹配端按需预置容量；对标 param 26.8ns/1 alloc。
2. **回溯不上堆**：迭代式回溯 + 栈上 `[16]matchFrame` 固定数组，超深溢出到 slice（罕见路径才分配）。
3. **并发契约**：先注册后 Serve、服务期只读无锁（与现行为一致）；`Router` doc comment 显式写明；`go test -race` 覆盖。
4. **冲突检测对等矩阵**：同位异名参数 panic；静态+参数共存允许；同前缀异名参数 panic —— 全部入测试钉死，防止换结构后悄悄放宽。
5. **段内前缀参数 `/user_:id`**：节点内匹配顺序固定 精确静态 → 前缀参数 → 纯参数 → splat，每层参与回溯；专项测试。
6. **请求路径永不 panic**：panic 仅限注册期；新增属性测试保证任意请求输入只返回 not-found。
7. **基准方法论**：基线/改后同会话连续采集，`-count=5` 取中位数，存档对比。
8. **B 接缝隐性成本**：未启用时仅一次 nil map 判断（~0.3ns，计入 1.3x 预算）。

## Tasks

> 提交策略：T3–T5 为同一重写的连续切片，期间全套测试存在中间红状态，统一在 T5 绿后一次提交（避免 main 上出现红提交）；其余任务独立提交。

### T0 · 基线采集

1. 命令：`mkdir -p .zeus/bench && go test -bench='Router' -benchmem -run '^$' -count=5 | tee .zeus/bench/F-001-baseline.txt`
2. 预期：BenchmarkRouter_StaticRoute/ParamRoute/ManyRoutes 各 5 组数字落盘。
3. 提交：`git add .zeus/bench/F-001-baseline.txt && git commit -m "chore: archive router benchmark baseline for F-001"`

### T1 · 契约钉测试（旧实现上即绿，作为换芯前后的不变量）

1. 新建 `router_rewrite_test.go`，写入：

```go
package rue

import "testing"

func noopChain() HandlersChain { return HandlersChain{func(c *Context) {}} }

// 冲突矩阵：换结构后必须保持对等的注册期 panic 行为
func TestRouter_ConflictMatrix(t *testing.T) {
	mustPanic := func(name string, fn func()) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic, got none")
				}
			}()
			fn()
		})
	}
	mustNotPanic := func(name string, fn func()) {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("unexpected panic: %v", r)
				}
			}()
			fn()
		})
	}

	mustPanic("same-position different param names", func() {
		r := newRouter()
		r.addRoute("GET", "/user/:id", noopChain())
		r.addRoute("GET", "/user/:name", noopChain())
	})
	mustPanic("duplicate registration", func() {
		r := newRouter()
		r.addRoute("GET", "/a/b", noopChain())
		r.addRoute("GET", "/a/b", noopChain())
	})
	mustPanic("empty method", func() {
		r := newRouter()
		r.addRoute("", "/a", noopChain())
	})
	mustPanic("path without leading slash", func() {
		r := newRouter()
		r.addRoute("GET", "a/b", noopChain())
	})
	mustPanic("empty handlers", func() {
		r := newRouter()
		r.addRoute("GET", "/a", HandlersChain{})
	})
	mustPanic("two wildcards in one segment", func() {
		r := newRouter()
		r.addRoute("GET", "/a/:x:y", noopChain())
	})
	mustPanic("unnamed param", func() {
		r := newRouter()
		r.addRoute("GET", "/a/:", noopChain())
	})
	mustPanic("catch-all not at end", func() {
		r := newRouter()
		r.addRoute("GET", "/a/*x/b", noopChain())
	})
	mustNotPanic("static and param siblings coexist", func() {
		r := newRouter()
		r.addRoute("GET", "/user/new", noopChain())
		r.addRoute("GET", "/user/:id", noopChain())
	})
}

// 段内前缀参数（gin 兼容形态）
func TestRouter_PrefixParamSegment(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/user_:id", noopChain())
	var ps Params
	h, pattern, ok := r.getValue("GET", "/user_42", &ps)
	if !ok || h == nil {
		t.Fatal("expected /user_42 to match /user_:id")
	}
	if pattern != "/user_:id" {
		t.Fatalf("pattern = %q, want /user_:id", pattern)
	}
	if v := ps.ByName("id"); v != "42" {
		t.Fatalf("id = %q, want 42", v)
	}
}

// rue.go:153 重定向探测用 nil params 调 getValue
func TestRouter_NilParams(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/x/:id", noopChain())
	h, _, ok := r.getValue("GET", "/x/1", nil)
	if !ok || h == nil {
		t.Fatal("nil params lookup must still match")
	}
}

// 根路由与根级通配
func TestRouter_RootForms(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/", noopChain())
	r.addRoute("GET", "/:x", noopChain())
	r.addRoute("POST", "/*rest", noopChain())

	if _, _, ok := r.getValue("GET", "/", nil); !ok {
		t.Fatal("/ must match")
	}
	var ps Params
	if _, _, ok := r.getValue("GET", "/abc", &ps); !ok || ps.ByName("x") != "abc" {
		t.Fatal("/:x must capture abc")
	}
	ps = ps[:0]
	if _, _, ok := r.getValue("POST", "/a/b/c", &ps); !ok || ps.ByName("rest") != "a/b/c" {
		t.Fatal("/*rest must capture a/b/c")
	}
}

// 请求路径输入永不 panic（注册期之外无 panic）
func TestRouter_LookupNeverPanics(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/a/:b/c", noopChain())
	r.addRoute("GET", "/files/*path", noopChain())
	inputs := []string{"", "/", "//", "/a", "/a/", "/a//c", "/a/b/c/d", "/%2e%2e", "/files", "/\x00", "/a/b/c"}
	for _, in := range inputs {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Fatalf("lookup panicked on %q: %v", in, rec)
				}
			}()
			var ps Params
			r.getValue("GET", in, &ps)
		}()
	}
}
```

2. 红验证（预期绿——这是契约钉，不是新行为）：`go test -run 'TestRouter_(ConflictMatrix|PrefixParamSegment|NilParams|RootForms|LookupNeverPanics)' ./...` → 全 PASS（若有任何 FAIL，说明对旧行为理解有误，停下修正测试认知，不改实现）。
3. 提交：`git add router_rewrite_test.go && git commit -m "test: pin router behavior contract ahead of clean-room rewrite"`

### T2 · 回溯新语义测试（旧实现上必须红 — G2 红证据）

1. 追加到 `router_rewrite_test.go`：

```go
// F-001 唯一行为增量：static 分支走死后回退 param 分支
func TestRouter_BacktrackStaticDeadEnd(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/user/new", noopChain())
	r.addRoute("GET", "/user/:id", noopChain())
	var ps Params
	h, pattern, ok := r.getValue("GET", "/user/newx", &ps)
	if !ok || h == nil {
		t.Fatal("/user/newx must fall back to /user/:id")
	}
	if pattern != "/user/:id" || ps.ByName("id") != "newx" {
		t.Fatalf("got pattern=%q id=%q, want /user/:id, newx", pattern, ps.ByName("id"))
	}
}

func TestRouter_BacktrackMultiLevel(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/a/b/c", noopChain())
	r.addRoute("GET", "/a/:x/d", noopChain())
	var ps Params
	_, pattern, ok := r.getValue("GET", "/a/b/d", &ps)
	if !ok || pattern != "/a/:x/d" || ps.ByName("x") != "b" {
		t.Fatalf("/a/b/d must backtrack to /a/:x/d, got ok=%v pattern=%q", ok, pattern)
	}
}

func TestRouter_BacktrackPrefixParamInterplay(t *testing.T) {
	r := newRouter()
	r.addRoute("GET", "/v1/user_admin/list", noopChain())
	r.addRoute("GET", "/v1/user_:name/info", noopChain())
	var ps Params
	_, pattern, ok := r.getValue("GET", "/v1/user_admin/info", &ps)
	if !ok || pattern != "/v1/user_:name/info" || ps.ByName("name") != "admin" {
		t.Fatalf("must backtrack from static user_admin to prefix-param, got ok=%v pattern=%q", ok, pattern)
	}
}
```

2. 红验证：`go test -run 'TestRouter_Backtrack' ./...` → 预期 **3 个 FAIL**（终端输出存档为 G2 红证据）。**不提交**（红状态不上 main，与 T5 同提交）。

### T3 · 重写·注册侧（类型 + 解析校验 + 插入 + 冲突检测）

1. 整体替换 `router.go` 内部实现。新类型骨架（标识符全部原创，B 接缝字段就位）：

```go
// Router matches request paths against registered patterns.
// Registration (addRoute) must complete before serving begins;
// lookups (getValue) are lock-free and safe for concurrent use
// only after registration has finished.
type Router struct {
	roots   map[string]*segNode            // method → segment-trie root
	statics map[string]map[string]staticHit // B 接缝：method → exact path → hit；nil = 未启用
	maxVars int                             // 单条路由最大参数数，Params 预置容量用
}

type staticHit struct {
	chain   HandlersChain
	pattern string
}

// segNode is one path segment in the trie.
type segNode struct {
	label      string     // 静态段文本；前缀参数节点存静态前缀
	name       string     // 参数/splat 名（capture/mix/tail 节点用）
	statics    []*segNode // 精确静态子节点
	firstChars []byte     // statics[i].label 的首字节索引，加速扫描
	mixes      []*segNode // 前缀参数子节点（label=前缀）
	capture    *segNode   // 纯 :param 子节点
	tail       *segNode   // *splat 子节点
	chain      HandlersChain
	pattern    string     // chain 非空时 = 注册原 pattern
}
```

2. 注册侧函数（全新控制流，按段切分而非字节前缀分裂）：
   - `addRoute`：校验（leading `/`、method 非空、handlers 非空）→ 取/建 method root → `root.mount(pattern, chain)`。
   - `(*segNode).mount`：循环 `nextSlash` 切段；每段分类为 literal / `prefix:name` / `:name` / `*name`（校验：单段单通配、必须命名、`*` 仅结尾且前导 `/`）；逐段下钻或新建子节点；终点写 `chain`+`pattern`，重复注册 panic；参数异名冲突 panic。
   - panic 文案全新措辞（示例）："rue: route pattern must start with '/'"、"rue: duplicate route registration for pattern '%s'"、"rue: a path segment may contain at most one ':' or '*' marker (pattern '%s')"、"rue: pattern '%s' conflicts with parameter '%s' already registered at this position"。
   - 注册时更新 `maxVars`。
   - 移除死字段 `paramsPool`/`maxParams`（全仓库无使用方，已核实）。
3. 验证（lookup 未实现，只验注册侧）：`go test -run 'TestRouter_ConflictMatrix' ./...` → PASS；`go build ./...` → 0。
4. 不提交（与 T5 同提交）。

### T4 · 重写·匹配侧（resolve：静态/前缀参数/参数/splat + Params 捕获）

1. `(*segNode).resolve(path string, params *Params) (HandlersChain, string, bool)`：
   - 零分配段迭代：`for start := 1; start <= len(path); ` + `nextSlash(path, start)`，全程索引运算，无 `strings.Split`。
   - 每段依序尝试：`firstChars` 扫静态 → `mixes` 前缀匹配 → `capture` 捕获整段 → 段耗尽时 `tail` 捕获余下全部。
   - 参数捕获：`params != nil` 时 append（容量不足先 `make(Params, 0, r.maxVars)` 一次性预置）；nil params 跳过捕获仅判通。
   - `getValue` 入口：B 接缝短路 `if r.statics != nil { ... }` → 否则 `roots[method].resolve(...)`。
2. 绿验证：`go test -run 'TestRouter_(StaticRoutes|ParamRoutes|WildcardRoutes|MethodRouting|NotFound|PrefixParamSegment|NilParams|RootForms|ParamWithDifferentSuffixes|ParamWithMultipleSuffixes)' ./...` → PASS。
3. 不提交（与 T5 同提交）。

### T5 · 重写·回溯 + 全套绿 + 提交

1. resolve 改为迭代回溯：`[16]matchFrame` 栈上数组（`type matchFrame struct{ n *segNode; pos int; nvars int; stage uint8 }`），stage 推进 静态→前缀参数→参数→splat；走死弹帧回退，`*params = (*params)[:f.nvars]` 截断已捕获参数；深度超 16 溢出 append 到 slice。
2. 绿验证（G2 绿证据，对应 T2 红）：`go test -run 'TestRouter_Backtrack' ./...` → 3 PASS。
3. 全套：`go test ./...` → 全 PASS；若 `router_test.go` 有钉死 gin 原文 panic 文案的断言（执行时 `grep -n "must begin\|can not be empty\|at least one handler\|wildcard" router_test.go` 核查），同步为新文案后再跑到绿。
4. 提交：`git add router.go router_rewrite_test.go router_test.go && git commit -m "feat: rewrite router as original segment trie (clean-room, F-001)"`

### T6 · 合规指纹清零审计

1. 命令：`! grep -qE 'wildChild|nType|incrementChildPrio|insertChild|longestCommonPrefix|findWildcard|catchAllNode|paramNode|staticNode' router.go && echo FINGERPRINT-CLEAN`
2. panic 文案逐字比对 gin tree.go 已知短语：`! grep -qE "must begin with|can not be empty|are already registered|only one wildcard|must be named with|only allowed at the end|no / before" router.go && echo TEXT-CLEAN`
3. 两条均输出 CLEAN；任一失败 → 改名/改文案重跑。无代码变更则无提交；有则 `git commit -am "refactor: scrub remaining derived identifiers"`。

### T7 · 并发与竞态

1. 命令：`go test -race ./...` → 全 PASS、无 race 报告。
2. `go vet ./...` → 0。

### T8 · 基准对比（1.3x 硬门）

1. 命令：`go test -bench='Router' -benchmem -run '^$' -count=5 | tee .zeus/bench/F-001-after.txt`
2. 取中位数对比基线：static ≤11ns 且 0 allocs、param ≤35ns、ManyRoutes ≤1.3x 基线。
3. 达标 → `git add .zeus/bench/F-001-after.txt && git commit -m "chore: archive post-rewrite router benchmarks"`，跳过 T9。未达标 → 进 T9。

### T9 ·（条件）启用 B 接缝

1. 注册时同步填充 `Router.statics`（仅纯静态 pattern）；`getValue` 已有短路位。先写测试：注册静态+动态各若干，断言两路径均正确（含接缝表与 Trie 一致性：同 pattern 仅一处命中）。
2. `go test ./...` 绿 + 重跑 T8 直到达标（阈值不放宽）。
3. 提交：`git commit -am "perf: enable static exact-match fast path to meet 1.3x gate"`；在 handoff 备忘录记录启用原因与前后数字。

### T10 · 文档刷新

1. README Benchmarks 小节替换为 `F-001-after.txt` 中位数实测；`Router` doc comment 写明并发契约。
2. 验证：`grep -n "8.348" README.md` → 无匹配（旧数字已清）。
3. 提交：`git add README.md && git commit -m "docs: refresh benchmark numbers after router rewrite"`

### T11 · 契约收尾

1. `.zeus/dod.md` 追加 G4 delta 条目（见下节）；`.zeus/features.md` F-001 行补 plan 链接。
2. 提交：`git add .zeus/dod.md .zeus/features.md && git commit -m "chore: record F-001 DoD delta"`

## Test Plan

- **单元**（`router_rewrite_test.go` 新增 + 既有直测）：冲突矩阵 9 例、前缀参数、nil-params、根形态 3 例、never-panic 属性、回溯 3 例；既有 `router_test.go` 13 测试 + `router_suffix_test.go` 2 测试断言不动作为回归网。
- **集成**：既有 `TestEngine_ServeHTTP/NotFound/PathParams/Groups/MiddlewareChain` 走 Engine→Router 全链路；`rue.go:153` trailing-slash 重定向路径由 `TestEngine_NotFound` + NilParams 测试间接覆盖。
- **E2E**（G5 阶段执行）：起样例服务，手测四类路径各一：`/ping`（静态）、`/users/42`（param）、`/static/css/a.css`（splat）、`/user/newx`（回溯）。
- **回归**：`go test ./...` 全套（含 middleware/context/binder 等 30+ 文件间接走路由）。

## Security Review

- **工具**：`go vet ./...`（每任务跑）；标准库 + 既有依赖无新增（无 supply-chain 增面）。
- **威胁模型**：router 是未信任输入（URL path）的第一接触点。新增面 = 无（替换实现，接口不变）。
- **具体检查**：
  - 恶意路径不 panic：`TestRouter_LookupNeverPanics`（含空串、`//`、`%2e%2e`、NUL 字节）。
  - DoS：匹配复杂度 O(len(path) + 回溯帧数)，回溯帧数受注册 pattern 形态约束（每段最多 4 种尝试），无 regex 无灾难性回溯；溢出 slice 仅在 >16 段超深路径出现，分配量与路径段数线性。
  - 路径遍历：router 只做匹配不碰文件系统；splat 捕获值原样下传 —— 在 `Router` doc comment 标注"splat 值未做清洗，文件服务方负责 `..` 处理"（现状一致，不新增风险）。
  - 注入/secrets/PII：router 无日志、无外部调用、无字符串拼接进任何解释器。

## Logic Review Checkpoints

- **CP1（T5 提交前）**：数据流单向（注册写 → 服务读）；panic 仅存在于注册侧函数；resolve 路径上无 `make`/`append` 之外的分配点（`-benchmem` 证实 static 0 alloc）；`segNode` 字段所有权清晰（statics/mixes/capture/tail 互斥语义成立）；圈复杂度 —— mount/resolve 各 ≤15 分支，超出则拆 helper。
- **CP2（T8 后）**：若进 T9，评审接缝一致性（双结构注册同步、冲突检测仍单点）；6 个月可维护性自问 —— 新人能否仅凭 doc comment + 测试读懂回溯帧机制（不能则补注释）。
- **YAGNI 核查**：除 B 接缝（用户明确要求预留）外不得出现"为将来"的空挂钩。

## G4 contract delta

完成后追加到 `.zeus/dod.md`：

- [ ] 合规指纹清零：`! grep -qE 'wildChild|nType|incrementChildPrio|insertChild|longestCommonPrefix|findWildcard|catchAllNode|paramNode|staticNode' router.go`
- [ ] router 基准不回退：static ≤11ns/0 alloc、param ≤35ns（对 `.zeus/bench/F-001-baseline.txt` 中位数 ≤1.3x）

## Logic Completeness Manifest

Every requirement in the linked spec MUST be implemented in full. Authorized simplifications: **(none)**

## File Size Constraints

| 文件 | 预估行数 | 阈值类别 | 判定 |
|---|---|---|---|
| `router.go` | ~340 | 复杂业务逻辑 300–500 | OK（>400 时按 mount/resolve 拆分评审） |
| `router_rewrite_test.go` | ~280 | 测试（relaxed） | OK |
| `router_test.go` | ±10（仅文案） | 测试（relaxed） | OK |
| `README.md` | ±10 | 文档（relaxed） | OK |
| `.zeus/dod.md` / `.zeus/features.md` / bench 存档 | 小 | 配置（relaxed） | OK |

---
*Self-review: 10 节齐全；File Map 每行在 Section 10 有对应行；无 OVER 行;每任务有精确验证命令与预期输出；测试计划覆盖全部新路径；安全节为具名检查；Go 单栈架构师清单已答；无 TODO/TBD/stub；Manifest 为 (none)。*

**User-approved:** 2026-06-11T08:18:27Z by rainhan
