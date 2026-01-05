package rue

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Mock service for testing
type mockService struct {
	UnimplementedMockServer
}

func (s *mockService) Echo(ctx context.Context, req *MockRequest) (*MockResponse, error) {
	gc := GetGRPCContext(ctx)
	if gc != nil {
		// Check if middleware set any values
		if val, ok := gc.Get("test_key"); ok {
			return &MockResponse{Message: val.(string)}, nil
		}
	}
	return &MockResponse{Message: req.Message}, nil
}

func (s *mockService) EchoStream(stream Mock_EchoStreamServer) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		if err := stream.Send(&MockResponse{Message: req.Message}); err != nil {
			return err
		}
	}
}

// Mock protobuf types (simplified for testing without proto generation)
type MockRequest struct {
	Message string
}

type MockResponse struct {
	Message string
}

type UnimplementedMockServer struct{}

type Mock_EchoStreamServer interface {
	grpc.ServerStream
	Send(*MockResponse) error
	Recv() (*MockRequest, error)
}

func TestGRPCServer_New(t *testing.T) {
	engine := New()
	gs := NewGRPCServer(engine)

	if gs == nil {
		t.Fatal("NewGRPCServer returned nil")
	}
	if gs.engine != engine {
		t.Error("Engine not set correctly")
	}
}

func TestGRPCServer_NewWithConfig(t *testing.T) {
	engine := New()
	config := &GRPCConfig{
		MaxRecvMsgSize:       8 * 1024 * 1024,
		MaxSendMsgSize:       8 * 1024 * 1024,
		MaxConcurrentStreams: 200,
		ConnectionTimeout:    60 * time.Second,
	}

	gs := NewGRPCServerWithConfig(engine, config)

	if gs == nil {
		t.Fatal("NewGRPCServerWithConfig returned nil")
	}
}

func TestGRPCServer_DefaultConfig(t *testing.T) {
	config := DefaultGRPCConfig()

	if config.MaxRecvMsgSize != 4*1024*1024 {
		t.Errorf("MaxRecvMsgSize = %d, want %d", config.MaxRecvMsgSize, 4*1024*1024)
	}
	if config.MaxSendMsgSize != 4*1024*1024 {
		t.Errorf("MaxSendMsgSize = %d, want %d", config.MaxSendMsgSize, 4*1024*1024)
	}
	if config.MaxConcurrentStreams != 100 {
		t.Errorf("MaxConcurrentStreams = %d, want %d", config.MaxConcurrentStreams, 100)
	}
}

func TestGRPCServer_Use(t *testing.T) {
	engine := New()
	gs := NewGRPCServer(engine)

	middleware := func(c *Context) {
		// Middleware logic
	}

	gs.Use(middleware)

	if len(gs.interceptors) != 1 {
		t.Errorf("Interceptors count = %d, want 1", len(gs.interceptors))
	}
	if len(gs.streamInterceptors) != 1 {
		t.Errorf("Stream interceptors count = %d, want 1", len(gs.streamInterceptors))
	}
}

func TestGRPCServer_UseUnary(t *testing.T) {
	engine := New()
	gs := NewGRPCServer(engine)

	interceptor := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(ctx, req)
	}

	gs.UseUnary(interceptor)

	if len(gs.interceptors) != 1 {
		t.Errorf("Interceptors count = %d, want 1", len(gs.interceptors))
	}
}

func TestGRPCServer_UseStream(t *testing.T) {
	engine := New()
	gs := NewGRPCServer(engine)

	interceptor := func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, ss)
	}

	gs.UseStream(interceptor)

	if len(gs.streamInterceptors) != 1 {
		t.Errorf("Stream interceptors count = %d, want 1", len(gs.streamInterceptors))
	}
}

func TestGRPCServer_Build(t *testing.T) {
	engine := New()
	gs := NewGRPCServer(engine)

	server := gs.Build()

	if server == nil {
		t.Fatal("Build returned nil")
	}
	if gs.Server() != server {
		t.Error("Server() should return the built server")
	}
}

func TestGRPCContext_SetGet(t *testing.T) {
	ctx := context.Background()
	gc := NewGRPCContext(ctx)

	gc.Set("key", "value")

	val, exists := gc.Get("key")
	if !exists {
		t.Error("Key should exist")
	}
	if val != "value" {
		t.Errorf("Value = %v, want 'value'", val)
	}
}

func TestGRPCContext_MustGet(t *testing.T) {
	ctx := context.Background()
	gc := NewGRPCContext(ctx)

	gc.Set("key", "value")

	val := gc.MustGet("key")
	if val != "value" {
		t.Errorf("Value = %v, want 'value'", val)
	}
}

func TestGRPCContext_MustGet_Panic(t *testing.T) {
	ctx := context.Background()
	gc := NewGRPCContext(ctx)

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustGet should panic for non-existent key")
		}
	}()

	gc.MustGet("nonexistent")
}

func TestGRPCContext_Metadata(t *testing.T) {
	md := metadata.New(map[string]string{
		"authorization": "Bearer token123",
		"x-custom":      "value",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	gc := NewGRPCContext(ctx)

	if gc.GetMetadata("authorization") != "Bearer token123" {
		t.Errorf("Authorization = %s, want 'Bearer token123'", gc.GetMetadata("authorization"))
	}
	if gc.GetMetadata("x-custom") != "value" {
		t.Errorf("X-Custom = %s, want 'value'", gc.GetMetadata("x-custom"))
	}
	if gc.GetMetadata("nonexistent") != "" {
		t.Error("Nonexistent metadata should return empty string")
	}
}

func TestGRPCContext_Abort(t *testing.T) {
	ctx := context.Background()
	gc := NewGRPCContext(ctx)

	if gc.IsAborted() {
		t.Error("Should not be aborted initially")
	}

	gc.Abort()

	if !gc.IsAborted() {
		t.Error("Should be aborted after Abort()")
	}
}

func TestGRPCContext_Error(t *testing.T) {
	ctx := context.Background()
	gc := NewGRPCContext(ctx)

	gc.Error(status.Error(codes.InvalidArgument, "invalid"))
	gc.Error(nil) // Should be ignored

	if len(gc.Errors()) != 1 {
		t.Errorf("Errors count = %d, want 1", len(gc.Errors()))
	}
}

func TestGRPCLogger(t *testing.T) {
	interceptor := GRPCLogger()

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	resp, err := interceptor(context.Background(), "request", info, handler)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resp != "response" {
		t.Errorf("Response = %v, want 'response'", resp)
	}
}

func TestGRPCRecovery(t *testing.T) {
	interceptor := GRPCRecovery()

	handler := func(ctx context.Context, req any) (any, error) {
		panic("test panic")
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	resp, err := interceptor(context.Background(), "request", info, handler)

	if resp != nil {
		t.Error("Response should be nil after panic")
	}
	if err == nil {
		t.Error("Error should not be nil after panic")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Error("Error should be a gRPC status error")
	}
	if st.Code() != codes.Internal {
		t.Errorf("Code = %v, want Internal", st.Code())
	}
}

func TestGRPCAuth(t *testing.T) {
	authFunc := func(ctx context.Context) (context.Context, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "no metadata")
		}
		tokens := md.Get("authorization")
		if len(tokens) == 0 || tokens[0] != "Bearer valid-token" {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}
		return context.WithValue(ctx, "user", "authenticated"), nil
	}

	interceptor := GRPCAuth(authFunc)

	handler := func(ctx context.Context, req any) (any, error) {
		return ctx.Value("user"), nil
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "/test.Service/Method",
	}

	// Test with valid token
	md := metadata.New(map[string]string{"authorization": "Bearer valid-token"})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	resp, err := interceptor(ctx, "request", info, handler)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resp != "authenticated" {
		t.Errorf("Response = %v, want 'authenticated'", resp)
	}

	// Test with invalid token
	md = metadata.New(map[string]string{"authorization": "Bearer invalid-token"})
	ctx = metadata.NewIncomingContext(context.Background(), md)

	_, err = interceptor(ctx, "request", info, handler)
	if err == nil {
		t.Error("Should return error for invalid token")
	}

	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("Code = %v, want Unauthenticated", st.Code())
	}
}

func TestChainUnaryInterceptors(t *testing.T) {
	var order []int

	interceptor1 := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		order = append(order, 1)
		resp, err := handler(ctx, req)
		order = append(order, 4)
		return resp, err
	}

	interceptor2 := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		order = append(order, 2)
		resp, err := handler(ctx, req)
		order = append(order, 3)
		return resp, err
	}

	chained := chainUnaryInterceptors([]grpc.UnaryServerInterceptor{interceptor1, interceptor2})

	handler := func(ctx context.Context, req any) (any, error) {
		return "response", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test"}
	chained(context.Background(), "request", info, handler)

	expected := []int{1, 2, 3, 4}
	if len(order) != len(expected) {
		t.Errorf("Order = %v, want %v", order, expected)
	}
	for i, v := range expected {
		if i < len(order) && order[i] != v {
			t.Errorf("order[%d] = %d, want %d", i, order[i], v)
		}
	}
}

func TestChainStreamInterceptors(t *testing.T) {
	var order []int

	interceptor1 := func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		order = append(order, 1)
		err := handler(srv, ss)
		order = append(order, 4)
		return err
	}

	interceptor2 := func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		order = append(order, 2)
		err := handler(srv, ss)
		order = append(order, 3)
		return err
	}

	chained := chainStreamInterceptors([]grpc.StreamServerInterceptor{interceptor1, interceptor2})

	handler := func(srv any, ss grpc.ServerStream) error {
		return nil
	}

	info := &grpc.StreamServerInfo{FullMethod: "/test"}
	chained(nil, nil, info, handler)

	expected := []int{1, 2, 3, 4}
	if len(order) != len(expected) {
		t.Errorf("Order = %v, want %v", order, expected)
	}
}

func TestGRPCServer_MiddlewareIntegration(t *testing.T) {
	engine := New()
	gs := NewGRPCServer(engine)

	gs.Use(func(c *Context) {
		c.Set("test_key", "middleware_value")
	})

	// Build server
	server := gs.Build()
	if server == nil {
		t.Fatal("Failed to build server")
	}

	// The middleware should be converted to interceptor
	if len(gs.interceptors) != 1 {
		t.Errorf("Interceptors count = %d, want 1", len(gs.interceptors))
	}
}

func TestGetGRPCContext(t *testing.T) {
	gc := NewGRPCContext(context.Background())
	gc.Set("test", "value")

	ctx := context.WithValue(context.Background(), grpcContextKey{}, gc)

	retrieved := GetGRPCContext(ctx)
	if retrieved == nil {
		t.Fatal("GetGRPCContext returned nil")
	}

	val, _ := retrieved.Get("test")
	if val != "value" {
		t.Errorf("Value = %v, want 'value'", val)
	}
}

func TestGetGRPCContext_NotFound(t *testing.T) {
	ctx := context.Background()
	gc := GetGRPCContext(ctx)

	if gc != nil {
		t.Error("GetGRPCContext should return nil for context without GRPCContext")
	}
}

func TestWrappedServerStream(t *testing.T) {
	ctx := context.WithValue(context.Background(), "key", "value")
	wrapped := &wrappedServerStream{
		ctx: ctx,
	}

	if wrapped.Context().Value("key") != "value" {
		t.Error("Wrapped stream should return custom context")
	}
}

// Integration test with actual gRPC server
func TestGRPCServer_Integration(t *testing.T) {
	engine := New()
	gs := NewGRPCServer(engine)

	// Add interceptors
	gs.UseUnary(GRPCLogger())
	gs.UseUnary(GRPCRecovery())

	// Build server
	server := gs.Build()

	// Start server on random port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go func() {
		server.Serve(lis)
	}()
	defer gs.GracefulStop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create client connection
	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	// Server is running, connection established
	// In a real test, we would register services and make RPC calls
}

func TestGRPCServer_Stop(t *testing.T) {
	engine := New()
	gs := NewGRPCServer(engine)
	gs.Build()

	// Should not panic
	gs.Stop()
}

func TestGRPCServer_GracefulStop(t *testing.T) {
	engine := New()
	gs := NewGRPCServer(engine)
	gs.Build()

	// Should not panic
	gs.GracefulStop()
}

func TestGRPCServer_StopWithoutBuild(t *testing.T) {
	engine := New()
	gs := NewGRPCServer(engine)

	// Should not panic even without Build()
	gs.Stop()
	gs.GracefulStop()
}
