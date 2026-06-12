# Handoff — F-001 Router Clean-Room Rewrite（完成）

日期: 2026-06-12 · 状态: **done**，全部 7 门通过 · 分支: main（直接提交，无待合并分支）

## 交付

`router.go` 整体重写为原创分段 Trie（gin/httprouter 派生代码清零）：
- 结构：按段建树 + 段对齐字面量链压缩（单链静态路由一次字符串比较）；四类子节点（statics/mixes/capture/tail）按特异性排序
- 查找：纯字面量层零状态快速路径 → 首个动态节点切入带回溯帧的完整解析器（[8]matchFrame 栈上，超深溢出堆）
- method 路由：常用 method string switch 直取，罕见 method 后备 map
- B 接缝（静态精确表）**保持休眠** —— 基准达标无需启用；`Router.statics` 字段在位，全仓库零赋值

## 关键数字（vs gin 系基线，M2 同机中位数）

static 8.99→**6.58ns（-27%）**、param 27.74→32.2ns（1.16x ≤1.3x门）、ManyRoutes 49.3→48.6ns、分配 0/1/1 不变（Many 96B→64B）。存档：`.zeus/bench/F-001-{baseline,after}.txt`

## 行为变更记录（G6 评审 400 万次差分模糊验证）

1. **回溯修正**（spec 授权的唯一增量）：static 走死回退 capture（`/user/newx` → `/user/:id`，旧 404）
2. **空捕获对等已恢复**（评审发现 3，用户裁定恢复旧语义）：非末尾位置捕获可空（`/a//c` → `b=""`），末尾不可；钉于 `TestRouter_EmptyCaptureParity`
3. **winner-flip**（少量）：旧实现 order-dependent 的胜出路由现在严格按特异性优先 —— 旧行为是 bug，新行为符合文档
4. **注册接受度变化**：`*` 捕获与同位其他路由的共存改为**对称拒绝**（旧：一个注册顺序 panic、反序则静默产生不可达路由甚至请求时 panic"invalid node type" —— 语料中旧实现 39.7 万次请求时 panic，新实现零）

## 顺手修复（独立 commit）

- `75ef299` SSEClient.Close 零值 nil channel panic（存量，卡 G4）

## 存量豁免（待办特性）

- **F-002** TestContext_ClientIP：疑似 ef832d1 trusted-proxy 安全语义 vs 测试期望冲突，需裁定
- **F-003** TestCompression_Property_ContentNegotiation：根因未查
- **F-004** TestWebSocket_FrameEncoding：数据竞争，仅 -race 暴露
验收命令带 -skip 豁免，见 `.zeus/dod.md`

## Commit 链

9700a9f 基线 → 80298e8 契约钉测试 → 75ef299 SSE 修复 → 774c9ad 重写 → f21e80d/fab1b58 存量登记 → a78c17e 性能（链压缩+method switch）→ c2368f7 README → e7d119f DoD delta → ac775f4 评审修复（空捕获对等）

## 下一步建议

1. F-002 优先（涉及安全语义裁定）走 zeus:systematic-debugging
2. /tmp/rue-e2e 样例服务目录残留（用户拒绝自动清理，可手动删）
3. 若未来启用 B 接缝：注册同步填充 `Router.statics`，`getValue` 短路位已就绪
