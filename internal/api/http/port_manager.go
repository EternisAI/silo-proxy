package http

import (
	"fmt"
	"log/slog"
	"sync"
)

// PortManager manages dynamic port allocation for per-agent HTTP servers.
// It uses a channel-based pool for thread-safe port allocation and tracks
// port-to-agent mappings for administrative purposes.
type PortManager struct {
	availablePorts chan int       // Buffered channel for thread-safe port pool
	allocatedPorts map[int]string // Port -> agent_id mapping
	mu             sync.RWMutex   // Protects allocatedPorts map
	rangeStart     int            // First port in range
	rangeEnd       int            // Last port in range (inclusive)
}

// NewPortManager creates a new PortManager with the specified port range.
// It pre-fills the port pool with all available ports.
// Returns an error if the port range is invalid (start > end or start < 1).
func NewPortManager(start, end int) (*PortManager, error) {
	if start > end {
		return nil, fmt.Errorf("invalid port range: start (%d) must be <= end (%d)", start, end)
	}
	if start < 1 || end < 1 {
		return nil, fmt.Errorf("invalid port range: ports must be >= 1 (start: %d, end: %d)", start, end)
	}
	if end > 65535 {
		return nil, fmt.Errorf("invalid port range: end port (%d) must be <= 65535", end)
	}

	poolSize := end - start + 1
	pm := &PortManager{
		availablePorts: make(chan int, poolSize),
		allocatedPorts: make(map[int]string),
		rangeStart:     start,
		rangeEnd:       end,
	}

	// Pre-fill the port pool
	for port := start; port <= end; port++ {
		pm.availablePorts <- port
	}

	slog.Info("PortManager initialized",
		"range_start", start,
		"range_end", end,
		"pool_size", poolSize)

	return pm, nil
}

// Allocate assigns an available port to the specified agent.
// Returns an error if no ports are available (pool exhausted).
// This operation is thread-safe and non-blocking.
func (pm *PortManager) Allocate(agentID string) (int, error) {
	// Non-blocking receive from channel
	select {
	case port := <-pm.availablePorts:
		pm.mu.Lock()
		pm.allocatedPorts[port] = agentID
		pm.mu.Unlock()

		slog.Info("Port allocated",
			"port", port,
			"agent_id", agentID,
			"available_ports", len(pm.availablePorts))

		return port, nil
	default:
		// Pool exhausted
		slog.Error("Port allocation failed: pool exhausted",
			"agent_id", agentID,
			"range_start", pm.rangeStart,
			"range_end", pm.rangeEnd)
		return 0, fmt.Errorf("no available ports in range %d-%d", pm.rangeStart, pm.rangeEnd)
	}
}

// Release returns a port to the available pool.
// This operation is idempotent - releasing an unallocated port is a safe no-op.
// Logs a warning if attempting to release a port not in the allocation map.
func (pm *PortManager) Release(port int) {
	pm.mu.Lock()
	agentID, exists := pm.allocatedPorts[port]
	if !exists {
		pm.mu.Unlock()
		slog.Warn("Attempted to release unallocated port", "port", port)
		return
	}
	delete(pm.allocatedPorts, port)
	pm.mu.Unlock()

	// Return port to pool
	pm.availablePorts <- port

	slog.Info("Port released",
		"port", port,
		"agent_id", agentID,
		"available_ports", len(pm.availablePorts))
}

// GetAllocations returns a thread-safe copy of current port allocations.
// The returned map is a snapshot and will not reflect subsequent changes.
func (pm *PortManager) GetAllocations() map[int]string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Return a copy to prevent external modification
	allocations := make(map[int]string, len(pm.allocatedPorts))
	for port, agentID := range pm.allocatedPorts {
		allocations[port] = agentID
	}

	return allocations
}
