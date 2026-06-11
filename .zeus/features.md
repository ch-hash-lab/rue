# Features — rue

| ID | Feature | Status | Spec | Plan |
|---|---|---|---|---|
| F-001 | Router clean-room rewrite（消除 gin/httprouter 派生合规风险） | in-progress | [spec](specs/2026-06-11-router-clean-room-rewrite-design.md) | [plan](plans/2026-06-11-router-clean-room-rewrite-plan.md) |
| F-002 | 修复 TestContext_ClientIP 存量失败（裁定 ef832d1 trusted-proxy 安全语义 vs 测试期望） | planned | — | — |
| F-003 | 修复 TestCompression_Property_ContentNegotiation 存量失败（根因未查） | planned | — | — |
| F-004 | 修复 TestWebSocket_FrameEncoding 数据竞争（仅 -race 暴露，websocket_test.go:159 牵涉） | planned | — | — |

> 存量基线（2026-06-11，F-001 期间盘点）：F-002/F-003 两项失败在 F-001 改动前即存在（stash 对比验证），与 router 无关。F-001 的 G4 按「除已登记存量外全绿 + 零新增失败」执行。

## F-001 DoD subset

- 合规指纹 grep 清零（wildChild/nType/incrementChildPrio/... 见 spec）
- 性能 ≤1.3x 基线（static ≤11ns、param ≤35ns、0 alloc 热路径）
- 全部现有测试不改断言通过；回溯/前缀参数新测试红绿
- README benchmark 数字刷新
