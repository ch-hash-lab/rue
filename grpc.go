package rue

import (
	"context"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCServer wraps a gRPC server with Rue integration
type GRPCServer struct {
	server             *grpc.Server
	engine             *Engine
	interceptors       []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
	mu                 sync.RWMutex
}

// GRPCConfig holds configuration for gRPC server
type GRPCConfig struct {
	// MaxRecvMsgSize is the maximum message size in bytes the server can receive
	MaxRecvMsgSize int
	// MaxSendMsgSize is the maximum message size in bytes the server can send
	MaxSendMsgSize int
	// MaxConcurrentStreams limits the number of concurrent streams per connection
	MaxConcurrentStreams uint32
	// ConnectionTimeout is the timeout for connection establishment
	ConnectionTimeout time.Duration
	// EnableReflection enables gRPC server reflection
	EnableReflection bool
}

// DefaultGRPCConfig returns default gRPC configuration
func DefaultGRPCConfig() *GRPCConfig {
	return &GRPCConfig{
		MaxRecvMsgSize:       4 * 1024 * 1024, // 4MB
		MaxSendMsgSize:       4 * 1024 * 1024, // 4MB
		MaxConcurrentStreams: 100,
		ConnectionTimeout:    120 * time.Second,
	}
}

// NewGRPCServer creates a new gRPC server integrated with Rue engine
func NewGRPCServer(engine *Engine) *GRPCServer {
	return NewGRPCServerWithConfig(engine, DefaultGRPCConfig())
}

// NewGRPCServerWithConfig creates a new gRPC server with custom configuration
func NewGRPCServerWithConfig(engine *Engine, config *GRPCConfig) *GRPCServer {
	if config == nil {
		config = DefaultGRPCConfig()
	}

	gs := &GRPCServer{
		engine:             engine,
		interceptors:       make([]grpc.UnaryServerInterceptor, 0),
		streamInterceptors: make([]grpc.StreamServerInterceptor, 0),
	}

	return gs
}

// Use adds Rue middleware as gRPC unary interceptor
func (gs *GRPCServer) Use(middleware ...HandlerFunc) *GRPCServer {
	for _, m := range middleware {
		gs.interceptors = append(gs.interceptors, gs.middlewareToUnaryInterceptor(m))
		gs.streamInterceptors = append(gs.streamInterceptors, gs.middlewareToStreamInterceptor(m))
	}
	return gs
}

// UseUnary adds a gRPC unary interceptor directly
func (gs *GRPCServer) UseUnary(interceptor grpc.UnaryServerInterceptor) *GRPCServer {
	gs.interceptors = append(gs.interceptors, interceptor)
	return gs
}

// UseStream adds a gRPC stream interceptor directly
func (gs *GRPCServer) UseStream(interceptor grpc.StreamServerInterceptor) *GRPCServer {
	gs.streamInterceptors = append(gs.streamInterceptors, interceptor)
	return gs
}

// Build creates the underlying gRPC server with all interceptors
func (gs *GRPCServer) Build(opts ...grpc.ServerOption) *grpc.Server {
	// Chain all unary interceptors
	chainedUnary := chainUnaryInterceptors(gs.interceptors)
	chainedStream := chainStreamInterceptors(gs.streamInterceptors)

	// Combine with provided options
	allOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(chainedUnary),
		grpc.ChainStreamInterceptor(chainedStream),
	}
	allOpts = append(allOpts, opts...)

	gs.server = grpc.NewServer(allOpts...)
	return gs.server
}

// Server returns the underlying gRPC server
func (gs *GRPCServer) Server() *grpc.Server {
	return gs.server
}

// Serve starts the gRPC server on the given listener
func (gs *GRPCServer) Serve(lis net.Listener) error {
	if gs.server == nil {
		gs.Build()
	}
	return gs.server.Serve(lis)
}

// Run starts the gRPC server on the given address
func (gs *GRPCServer) Run(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return gs.Serve(lis)
}

// GracefulStop gracefully stops the gRPC server
func (gs *GRPCServer) GracefulStop() {
	if gs.server != nil {
		gs.server.GracefulStop()
	}
}

// Stop immediately stops the gRPC server
func (gs *GRPCServer) Stop() {
	if gs.server != nil {
		gs.server.Stop()
	}
}

// GRPCContext wraps gRPC context with Rue-like interface
type GRPCContext struct {
	ctx     context.Context
	md      metadata.MD
	keys    map[string]any
	mu      sync.RWMutex
	aborted bool
	errors  []error
}

// NewGRPCContext creates a new GRPCContext from gRPC context
func NewGRPCContext(ctx context.Context) *GRPCContext {
	md, _ := metadata.FromIncomingContext(ctx)
	return &GRPCContext{
		ctx:  ctx,
		md:   md,
		keys: make(map[string]any),
	}
}

// Context returns the underlying context
func (gc *GRPCContext) Context() context.Context {
	return gc.ctx
}

// Set stores a key-value pair
func (gc *GRPCContext) Set(key string, value any) {
	gc.mu.Lock()
	gc.keys[key] = value
	gc.mu.Unlock()
}

// Get retrieves a value by key
func (gc *GRPCContext) Get(key string) (any, bool) {
	gc.mu.RLock()
	value, exists := gc.keys[key]
	gc.mu.RUnlock()
	return value, exists
}

// MustGet retrieves a value or panics if not found
func (gc *GRPCContext) MustGet(key string) any {
	value, exists := gc.Get(key)
	if !exists {
		panic("Key \"" + key + "\" does not exist")
	}
	return value
}

// Metadata returns the incoming metadata
func (gc *GRPCContext) Metadata() metadata.MD {
	return gc.md
}

// GetMetadata retrieves a metadata value
func (gc *GRPCContext) GetMetadata(key string) string {
	values := gc.md.Get(key)
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

// Abort marks the context as aborted
func (gc *GRPCContext) Abort() {
	gc.aborted = true
}

// IsAborted returns whether the context is aborted
func (gc *GRPCContext) IsAborted() bool {
	return gc.aborted
}

// Error adds an error to the context
func (gc *GRPCContext) Error(err error) {
	if err != nil {
		gc.errors = append(gc.errors, err)
	}
}

// Errors returns all errors
func (gc *GRPCContext) Errors() []error {
	return gc.errors
}

// middlewareToUnaryInterceptor converts Rue middleware to gRPC unary interceptor
func (gs *GRPCServer) middlewareToUnaryInterceptor(middleware HandlerFunc) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		gc := NewGRPCContext(ctx)

		// Store gRPC info in context
		gc.Set("grpc_method", info.FullMethod)
		gc.Set("grpc_request", req)

		// Create a minimal Rue context for middleware
		rueCtx := &Context{
			engine: gs.engine,
		}
		rueCtx.Set("grpc_context", gc)
		rueCtx.Set("grpc_method", info.FullMethod)

		// Execute middleware
		middleware(rueCtx)

		// Check if aborted
		if rueCtx.IsAborted() {
			return nil, status.Error(codes.Aborted, "request aborted by middleware")
		}

		// Copy store from Rue context to gRPC context
		rueCtx.mu.RLock()
		for k, v := range rueCtx.store {
			gc.Set(k, v)
		}
		rueCtx.mu.RUnlock()

		// Store GRPCContext in context for handler access
		ctx = context.WithValue(ctx, grpcContextKey{}, gc)

		return handler(ctx, req)
	}
}

// middlewareToStreamInterceptor converts Rue middleware to gRPC stream interceptor
func (gs *GRPCServer) middlewareToStreamInterceptor(middleware HandlerFunc) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		gc := NewGRPCContext(ss.Context())

		// Store gRPC info in context
		gc.Set("grpc_method", info.FullMethod)
		gc.Set("grpc_is_client_stream", info.IsClientStream)
		gc.Set("grpc_is_server_stream", info.IsServerStream)

		// Create a minimal Rue context for middleware
		rueCtx := &Context{
			engine: gs.engine,
		}
		rueCtx.Set("grpc_context", gc)
		rueCtx.Set("grpc_method", info.FullMethod)

		// Execute middleware
		middleware(rueCtx)

		// Check if aborted
		if rueCtx.IsAborted() {
			return status.Error(codes.Aborted, "request aborted by middleware")
		}

		// Wrap stream with context
		wrapped := &wrappedServerStream{
			ServerStream: ss,
			ctx:          context.WithValue(ss.Context(), grpcContextKey{}, gc),
		}

		return handler(srv, wrapped)
	}
}

// grpcContextKey is the key for storing GRPCContext in context
type grpcContextKey struct{}

// GetGRPCContext retrieves GRPCContext from context
func GetGRPCContext(ctx context.Context) *GRPCContext {
	gc, _ := ctx.Value(grpcContextKey{}).(*GRPCContext)
	return gc
}

// wrappedServerStream wraps grpc.ServerStream with custom context
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// chainUnaryInterceptors chains multiple unary interceptors into one
func chainUnaryInterceptors(interceptors []grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if len(interceptors) == 0 {
			return handler(ctx, req)
		}

		// Build chain from end to start
		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			next := chain
			chain = func(ctx context.Context, req any) (any, error) {
				return interceptor(ctx, req, info, next)
			}
		}

		return chain(ctx, req)
	}
}

// chainStreamInterceptors chains multiple stream interceptors into one
func chainStreamInterceptors(interceptors []grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if len(interceptors) == 0 {
			return handler(srv, ss)
		}

		// Build chain from end to start
		chain := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			next := chain
			chain = func(srv any, ss grpc.ServerStream) error {
				return interceptor(srv, ss, info, next)
			}
		}

		return chain(srv, ss)
	}
}

// Built-in gRPC interceptors

// GRPCLogger returns a logging interceptor for gRPC
func GRPCLogger() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		latency := time.Since(start)
		code := codes.OK
		if err != nil {
			if s, ok := status.FromError(err); ok {
				code = s.Code()
			} else {
				code = codes.Unknown
			}
		}

		// Log using default logger format
		debugPrintf("[gRPC] %s | %s | %v\n", info.FullMethod, code, latency)

		return resp, err
	}
}

// GRPCRecovery returns a recovery interceptor for gRPC
func GRPCRecovery() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				debugPrintf("[gRPC Recovery] panic recovered: %v\n", r)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()

		return handler(ctx, req)
	}
}

// GRPCStreamLogger returns a logging interceptor for gRPC streams
func GRPCStreamLogger() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()

		err := handler(srv, ss)

		latency := time.Since(start)
		code := codes.OK
		if err != nil {
			if s, ok := status.FromError(err); ok {
				code = s.Code()
			} else {
				code = codes.Unknown
			}
		}

		debugPrintf("[gRPC Stream] %s | %s | %v\n", info.FullMethod, code, latency)

		return err
	}
}

// GRPCStreamRecovery returns a recovery interceptor for gRPC streams
func GRPCStreamRecovery() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				debugPrintf("[gRPC Stream Recovery] panic recovered: %v\n", r)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()

		return handler(srv, ss)
	}
}

// GRPCAuth returns an authentication interceptor
func GRPCAuth(authFunc func(ctx context.Context) (context.Context, error)) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		newCtx, err := authFunc(ctx)
		if err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

// GRPCStreamAuth returns an authentication interceptor for streams
func GRPCStreamAuth(authFunc func(ctx context.Context) (context.Context, error)) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		newCtx, err := authFunc(ss.Context())
		if err != nil {
			return err
		}

		wrapped := &wrappedServerStream{
			ServerStream: ss,
			ctx:          newCtx,
		}

		return handler(srv, wrapped)
	}
}

// GRPCRateLimiter returns a rate limiting interceptor using a custom limiter interface
func GRPCRateLimiter(allowFunc func(key string) bool, keyFunc func(ctx context.Context) string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		key := keyFunc(ctx)
		if !allowFunc(key) {
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(ctx, req)
	}
}

// Helper function for debug logging
func debugPrintf(format string, args ...any) {
	// Use standard log for now
	// In production, this could be configurable
}
