module github.com/ch-hash-lab/rue

go 1.26.0

require (
	github.com/andybalholm/brotli v1.2.0
	github.com/bytedance/sonic v1.15.0
	github.com/quic-go/quic-go v0.58.0
	google.golang.org/grpc v1.78.0
)

require (
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic/loader v0.5.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/klauspost/cpuid/v2 v2.2.9 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	golang.org/x/arch v0.0.0-20210923205945-b76863e36670 // indirect
	golang.org/x/crypto v0.44.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

retract [v0.0.1, v0.0.6] // Contains unattributed derived code; use v0.0.7+
