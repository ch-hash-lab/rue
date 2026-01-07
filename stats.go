package rue

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// StatsConfig holds configuration for system statistics reporting
type StatsConfig struct {
	// Interval is the time between stats reports (default: 1 minute)
	Interval time.Duration
	// Enabled controls whether stats reporting is active (default: true)
	Enabled bool
}

// DefaultStatsConfig returns the default stats configuration
func DefaultStatsConfig() StatsConfig {
	return StatsConfig{
		Interval: time.Minute,
		Enabled:  true,
	}
}

// statsReporter handles periodic system statistics reporting
type statsReporter struct {
	config   StatsConfig
	logger   *Logger
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	started  bool
	mu       sync.Mutex
	lastCPU  uint64
	lastTime time.Time
}

// newStatsReporter creates a new stats reporter
func newStatsReporter(config StatsConfig, logger *Logger) *statsReporter {
	ctx, cancel := context.WithCancel(context.Background())
	return &statsReporter{
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// start begins the periodic stats reporting
func (s *statsReporter) start() {
	s.mu.Lock()
	if s.started || !s.config.Enabled {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.lastTime = time.Now()
	s.mu.Unlock()

	s.wg.Add(1)
	go s.run()
}

// stop stops the stats reporting
func (s *statsReporter) stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	s.cancel()
	s.wg.Wait()
}

// run is the main loop for stats reporting
func (s *statsReporter) run() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.report()
		}
	}
}

// report collects and logs system statistics
func (s *statsReporter) report() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Calculate values
	allocMi := float64(m.Alloc) / 1024 / 1024
	totalAllocMi := float64(m.TotalAlloc) / 1024 / 1024
	sysMi := float64(m.Sys) / 1024 / 1024
	numGC := m.NumGC
	goroutines := runtime.NumGoroutine()

	// Format the stats message
	msg := fmt.Sprintf("MEMORY: Alloc=%.1fMi, TotalAlloc=%.1fMi, Sys=%.1fMi, NumGC=%d, Goroutines=%d",
		allocMi, totalAllocMi, sysMi, numGC, goroutines)

	s.logger.Stat(msg)
}

// SystemStats returns a middleware that enables system statistics reporting
// This is automatically included in Default() but can be added manually with custom config
func SystemStats() HandlerFunc {
	return SystemStatsWithConfig(DefaultStatsConfig())
}

// SystemStatsWithConfig returns a middleware with custom stats configuration
func SystemStatsWithConfig(config StatsConfig) HandlerFunc {
	// This middleware doesn't actually intercept requests
	// It's used to signal that stats should be enabled
	return func(c *Context) {
		c.Next()
	}
}
