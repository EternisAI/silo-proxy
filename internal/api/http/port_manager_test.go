package http

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortManager_AllocateAndRelease(t *testing.T) {
	pm, err := NewPortManager(8100, 8105)
	require.NoError(t, err)

	// Allocate a port
	port, err := pm.Allocate("agent-1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, port, 8100)
	assert.LessOrEqual(t, port, 8105)

	// Verify allocation is tracked
	allocations := pm.GetAllocations()
	assert.Equal(t, "agent-1", allocations[port])

	// Release the port
	pm.Release(port)

	// Verify port is removed from allocations
	allocations = pm.GetAllocations()
	assert.NotContains(t, allocations, port)

	// Verify port is back in pool by allocating again
	port2, err := pm.Allocate("agent-2")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, port2, 8100)
	assert.LessOrEqual(t, port2, 8105)

	// Verify the new allocation is tracked
	allocations = pm.GetAllocations()
	assert.Equal(t, "agent-2", allocations[port2])
}

func TestPortManager_Exhaustion(t *testing.T) {
	pm, err := NewPortManager(8100, 8102) // Only 3 ports available
	require.NoError(t, err)

	// Allocate all ports
	ports := make([]int, 0, 3)
	for i := 0; i < 3; i++ {
		port, err := pm.Allocate(string(rune('A' + i)))
		require.NoError(t, err)
		ports = append(ports, port)
	}

	// Verify all ports are unique
	uniquePorts := make(map[int]bool)
	for _, port := range ports {
		uniquePorts[port] = true
	}
	assert.Equal(t, 3, len(uniquePorts), "all allocated ports should be unique")

	// Next allocation should fail
	_, err = pm.Allocate("agent-overflow")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no available ports")

	// Release one port
	pm.Release(ports[0])

	// Now allocation should succeed
	port, err := pm.Allocate("agent-recovered")
	require.NoError(t, err)
	assert.Equal(t, ports[0], port, "should reuse the released port")
}

func TestPortManager_ConcurrentAllocations(t *testing.T) {
	pm, err := NewPortManager(8100, 8120) // 21 ports for 10 goroutines
	require.NoError(t, err)

	var wg sync.WaitGroup
	portsChan := make(chan int, 10)
	errorsChan := make(chan error, 10)

	// Launch 10 concurrent allocations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			port, err := pm.Allocate(string(rune('A' + id)))
			if err != nil {
				errorsChan <- err
				return
			}
			portsChan <- port
		}(i)
	}

	wg.Wait()
	close(portsChan)
	close(errorsChan)

	// Verify no errors occurred
	errors := make([]error, 0)
	for err := range errorsChan {
		errors = append(errors, err)
	}
	assert.Empty(t, errors, "no allocation errors should occur")

	// Verify all ports are unique
	allocatedPorts := make([]int, 0)
	for port := range portsChan {
		allocatedPorts = append(allocatedPorts, port)
	}

	uniquePorts := make(map[int]bool)
	for _, port := range allocatedPorts {
		assert.False(t, uniquePorts[port], "port %d was allocated more than once", port)
		uniquePorts[port] = true
	}

	assert.Equal(t, 10, len(uniquePorts), "should have 10 unique ports")

	// Verify allocations are tracked correctly
	allocations := pm.GetAllocations()
	assert.Equal(t, 10, len(allocations))
}

func TestPortManager_ReleaseIdempotency(t *testing.T) {
	pm, err := NewPortManager(8100, 8105)
	require.NoError(t, err)

	// Allocate a port
	port, err := pm.Allocate("agent-1")
	require.NoError(t, err)

	// Release it once
	pm.Release(port)

	// Release it again - should be safe no-op
	pm.Release(port)

	// Release a never-allocated port - should be safe no-op
	pm.Release(8106)

	// Verify no panics occurred and a port can be allocated
	port2, err := pm.Allocate("agent-2")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, port2, 8100)
	assert.LessOrEqual(t, port2, 8105)
}

func TestPortManager_AllocationTracking(t *testing.T) {
	pm, err := NewPortManager(8100, 8105)
	require.NoError(t, err)

	// Allocate multiple ports
	port1, err := pm.Allocate("agent-1")
	require.NoError(t, err)

	port2, err := pm.Allocate("agent-2")
	require.NoError(t, err)

	port3, err := pm.Allocate("agent-3")
	require.NoError(t, err)

	// Verify all allocations are tracked
	allocations := pm.GetAllocations()
	assert.Equal(t, 3, len(allocations))
	assert.Equal(t, "agent-1", allocations[port1])
	assert.Equal(t, "agent-2", allocations[port2])
	assert.Equal(t, "agent-3", allocations[port3])

	// Release one port
	pm.Release(port2)

	// Verify updated allocations
	allocations = pm.GetAllocations()
	assert.Equal(t, 2, len(allocations))
	assert.Equal(t, "agent-1", allocations[port1])
	assert.Equal(t, "agent-3", allocations[port3])
	assert.NotContains(t, allocations, port2)

	// Verify returned map is a copy (mutations don't affect internal state)
	allocations[9999] = "fake-agent"
	newAllocations := pm.GetAllocations()
	assert.NotContains(t, newAllocations, 9999, "external mutations should not affect internal state")
}

func TestPortManager_InvalidConfiguration(t *testing.T) {
	// Test start > end
	_, err := NewPortManager(8200, 8100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start")

	// Test port < 1
	_, err = NewPortManager(0, 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ports must be >= 1")

	// Test port > 65535
	_, err = NewPortManager(65000, 70000)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "65535")

	// Test valid configuration returns no error
	pm, err := NewPortManager(8100, 8200)
	assert.NoError(t, err)
	assert.NotNil(t, pm)
}

func TestPortManager_SimulateAgentChurn(t *testing.T) {
	pm, err := NewPortManager(8100, 8110) // 11 ports
	require.NoError(t, err)

	var wg sync.WaitGroup
	iterations := 50

	// Simulate 5 agents constantly connecting and disconnecting
	for agentNum := 0; agentNum < 5; agentNum++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			agentID := string(rune('A' + id))

			for i := 0; i < iterations; i++ {
				// Allocate
				port, err := pm.Allocate(agentID)
				if err != nil {
					// Pool might be temporarily exhausted, skip this iteration
					continue
				}

				// Verify allocation
				allocations := pm.GetAllocations()
				assert.Equal(t, agentID, allocations[port])

				// Release immediately
				pm.Release(port)
			}
		}(agentNum)
	}

	wg.Wait()

	// After churn, all ports should be available since we release immediately
	allocations := pm.GetAllocations()
	assert.Empty(t, allocations, "all ports should be released after churn")

	// Verify pool is healthy - allocate all 11 ports
	allocatedPorts := make([]int, 0, 11)
	for i := 0; i < 11; i++ {
		port, err := pm.Allocate(string(rune('A' + i)))
		require.NoError(t, err, "should be able to allocate all ports after churn")
		allocatedPorts = append(allocatedPorts, port)
	}

	// Verify all ports are unique
	uniquePorts := make(map[int]bool)
	for _, port := range allocatedPorts {
		uniquePorts[port] = true
	}
	assert.Equal(t, 11, len(uniquePorts), "all allocated ports should be unique")

	// Clean up
	for _, port := range allocatedPorts {
		pm.Release(port)
	}
}
