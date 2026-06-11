# Definition of Done — rue

Bootstrap DoD (G4 baseline). Every item must exit 0. Refine thresholds via `zeus:kickoff-definition-of-done`.

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `go test -race ./...`
- [ ] No performance regression on router benchmarks: `go test -bench='Engine|Router' -benchmem -run '^$' ./...` compared against pre-change baseline (static match stays ~8ns class, 0 allocs/op on hot path)

## F-001 delta (router clean-room rewrite)

- [ ] 合规指纹清零：`! grep -qE 'wildChild|nType|incrementChildPrio|insertChild|longestCommonPrefix|findWildcard|catchAllNode|paramNode|staticNode' router.go`
- [ ] gin 原文 panic 短语清零：`! grep -qE "must begin with|can not be empty|are already registered|only one wildcard|must be named with|only allowed at the end|no / before" router.go`
- [ ] router 基准不回退（对 `.zeus/bench/F-001-baseline.txt` 中位数 ≤1.3x）：static ≤11ns 且 0 allocs、param ≤35ns、ManyRoutes ≤64.1ns

## 存量豁免（详见 .zeus/features.md）

`go test ./...` 与 `go test -race ./...` 按「零新增失败」执行，已登记存量：F-002（TestContext_ClientIP）、F-003（TestCompression_Property_ContentNegotiation）、F-004（TestWebSocket_FrameEncoding，仅 -race）。验收命令：
`go test -skip 'TestContext_ClientIP|TestCompression_Property_ContentNegotiation' ./...`
`go test -race -skip 'TestContext_ClientIP|TestCompression_Property_ContentNegotiation|TestWebSocket_FrameEncoding' ./...`
