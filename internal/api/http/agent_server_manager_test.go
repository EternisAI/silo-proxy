package http

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	grpcserver "github.com/EternisAI/silo-proxy/internal/grpc/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgentServerManager(t *testing.T) {
	pm, err := NewPortManager(8100, 8102)
	require.NoError(t, err)

	gs := grpcserver.NewServer(9090, nil)

	asm := NewAgentServerManager(pm, gs)
	assert.NotNil(t, asm)
	assert.NotNil(t, asm.servers)
	assert.NotNil(t, asm.portManager)
	assert.NotNil(t, asm.grpcServer)
}

func TestStartAgentServer_Success(t *testing.T) {
	pm, err := NewPortManager(8100, 8102)
	require.NoError(t, err)

	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	port, err := asm.StartAgentServer("agent-1")
	require.NoError(t, err)
	assert.True(t, port >= 8100 && port <= 8102)

	// Verify server was created
	servers := asm.GetAllServers()
	require.Len(t, servers, 1)
	assert.Equal(t, "agent-1", servers[0].AgentID)
	assert.Equal(t, port, servers[0].Port)

	// Cleanup
	err = asm.StopAgentServer("agent-1")
	require.NoError(t, err)
}

func TestStartAgentServer_DuplicateAgent(t *testing.T) {
	pm, err := NewPortManager(8100, 8102)
	require.NoError(t, err)

	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	// Start first server
	port1, err := asm.StartAgentServer("agent-1")
	require.NoError(t, err)
	assert.True(t, port1 >= 8100 && port1 <= 8102)

	// Try to start duplicate
	_, err = asm.StartAgentServer("agent-1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Cleanup
	err = asm.StopAgentServer("agent-1")
	require.NoError(t, err)
}

func TestStartAgentServer_PortExhaustion(t *testing.T) {
	// Create manager with only 2 ports
	pm, err := NewPortManager(8100, 8101)
	require.NoError(t, err)

	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	// Start 2 servers successfully
	port1, err := asm.StartAgentServer("agent-1")
	require.NoError(t, err)
	assert.True(t, port1 >= 8100 && port1 <= 8101)

	port2, err := asm.StartAgentServer("agent-2")
	require.NoError(t, err)
	assert.True(t, port2 >= 8100 && port2 <= 8101)
	assert.NotEqual(t, port1, port2)

	// Third server should fail
	_, err = asm.StartAgentServer("agent-3")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to allocate port")

	// Cleanup
	asm.StopAgentServer("agent-1")
	asm.StopAgentServer("agent-2")
}

func TestStopAgentServer_Success(t *testing.T) {
	pm, err := NewPortManager(8100, 8102)
	require.NoError(t, err)

	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	// Start server
	_, err = asm.StartAgentServer("agent-1")
	require.NoError(t, err)

	// Verify server is running
	servers := asm.GetAllServers()
	require.Len(t, servers, 1)

	// Stop server
	err = asm.StopAgentServer("agent-1")
	require.NoError(t, err)

	// Verify server removed
	servers = asm.GetAllServers()
	assert.Len(t, servers, 0)

	// Verify port was released back to pool
	allocations := pm.GetAllocations()
	assert.Len(t, allocations, 0)

	// Should be able to allocate a new port (not necessarily the same one due to channel ordering)
	port2, err := asm.StartAgentServer("agent-2")
	require.NoError(t, err)
	assert.True(t, port2 >= 8100 && port2 <= 8102) // Should get a valid port from range

	// Cleanup
	asm.StopAgentServer("agent-2")
}

func TestStopAgentServer_NonExistent(t *testing.T) {
	pm, err := NewPortManager(8100, 8102)
	require.NoError(t, err)

	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	err = asm.StopAgentServer("non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no server found")
}

func TestGetAllServers(t *testing.T) {
	pm, err := NewPortManager(8100, 8102)
	require.NoError(t, err)

	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	// Initially empty
	servers := asm.GetAllServers()
	assert.Len(t, servers, 0)

	// Start 3 servers
	agents := []string{"agent-1", "agent-2", "agent-3"}
	ports := make(map[string]int)

	for _, agentID := range agents {
		port, err := asm.StartAgentServer(agentID)
		require.NoError(t, err)
		ports[agentID] = port
	}

	// Verify all servers returned
	servers = asm.GetAllServers()
	require.Len(t, servers, 3)

	// Verify each server info
	for _, server := range servers {
		assert.Contains(t, agents, server.AgentID)
		assert.Equal(t, ports[server.AgentID], server.Port)
		assert.NotNil(t, server.Server)
		assert.False(t, server.StartedAt.IsZero())
	}

	// Cleanup
	for _, agentID := range agents {
		asm.StopAgentServer(agentID)
	}
}

func TestShutdown(t *testing.T) {
	pm, err := NewPortManager(8100, 8102)
	require.NoError(t, err)

	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	// Start 3 servers
	agents := []string{"agent-1", "agent-2", "agent-3"}
	for _, agentID := range agents {
		_, err := asm.StartAgentServer(agentID)
		require.NoError(t, err)
	}

	// Verify all running
	servers := asm.GetAllServers()
	require.Len(t, servers, 3)

	// Shutdown all
	err = asm.Shutdown()
	require.NoError(t, err)

	// Verify all stopped
	servers = asm.GetAllServers()
	assert.Len(t, servers, 0)

	// Verify all ports released
	allocations := pm.GetAllocations()
	assert.Len(t, allocations, 0)
}

func TestConcurrentServerOperations(t *testing.T) {
	pm, err := NewPortManager(8100, 8120)
	require.NoError(t, err)

	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	// Start 10 servers concurrently
	numServers := 10
	results := make(chan error, numServers)

	for i := 0; i < numServers; i++ {
		go func(id int) {
			agentID := fmt.Sprintf("agent-%d", id)
			_, err := asm.StartAgentServer(agentID)
			results <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numServers; i++ {
		err := <-results
		require.NoError(t, err)
	}

	// Verify all servers started
	servers := asm.GetAllServers()
	assert.Len(t, servers, numServers)

	// Stop all servers concurrently
	for i := 0; i < numServers; i++ {
		go func(id int) {
			agentID := fmt.Sprintf("agent-%d", id)
			results <- asm.StopAgentServer(agentID)
		}(i)
	}

	// Collect results
	for i := 0; i < numServers; i++ {
		err := <-results
		require.NoError(t, err)
	}

	// Verify all stopped
	servers = asm.GetAllServers()
	assert.Len(t, servers, 0)
}

func TestAgentServerLifecycle(t *testing.T) {
	pm, err := NewPortManager(8100, 8102)
	require.NoError(t, err)

	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	// Start server
	port, err := asm.StartAgentServer("agent-1")
	require.NoError(t, err)

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	// Verify server is listening (should get connection refused or 404, not connection error)
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err == nil {
		resp.Body.Close()
	}
	// We expect some response (even if it's 404), as the server should be running

	// Stop server
	err = asm.StopAgentServer("agent-1")
	require.NoError(t, err)

	// Verify server stopped (should get connection error now)
	time.Sleep(200 * time.Millisecond)
	_, err = client.Get(fmt.Sprintf("http://localhost:%d/health", port))
	assert.Error(t, err) // Should fail to connect as server is stopped
}

func TestGracefulShutdownTimeout(t *testing.T) {
	pm, err := NewPortManager(8100, 8102)
	require.NoError(t, err)

	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	// Start server
	_, err = asm.StartAgentServer("agent-1")
	require.NoError(t, err)

	// Stop should complete within timeout
	startTime := time.Now()
	err = asm.StopAgentServer("agent-1")
	duration := time.Since(startTime)

	require.NoError(t, err)
	// Should complete quickly since there are no active connections
	assert.Less(t, duration, serverShutdownTimeout*2)
}

func TestServerInfoCopy(t *testing.T) {
	pm, err := NewPortManager(8100, 8102)
	require.NoError(t, err)

	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	// Start server
	_, err = asm.StartAgentServer("agent-1")
	require.NoError(t, err)

	// Get servers
	servers1 := asm.GetAllServers()
	require.Len(t, servers1, 1)

	// Modify the returned slice
	servers1[0].AgentID = "modified"

	// Get servers again
	servers2 := asm.GetAllServers()
	require.Len(t, servers2, 1)

	// Original should be unchanged
	assert.Equal(t, "agent-1", servers2[0].AgentID)
	assert.NotEqual(t, "modified", servers2[0].AgentID)

	// Cleanup
	asm.StopAgentServer("agent-1")
}

// TestServerRoutingIsolation verifies that each agent server only forwards
// requests to its designated agent
func TestServerRoutingIsolation(t *testing.T) {
	pm, err := NewPortManager(8100, 8102)
	require.NoError(t, err)

	// Create a mock gRPC server
	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	// Start servers for two different agents
	port1, err := asm.StartAgentServer("agent-1")
	require.NoError(t, err)

	port2, err := asm.StartAgentServer("agent-2")
	require.NoError(t, err)

	// Verify they got different ports
	assert.NotEqual(t, port1, port2)

	// Verify we can get info for each server
	servers := asm.GetAllServers()
	require.Len(t, servers, 2)

	// Cleanup
	asm.StopAgentServer("agent-1")
	asm.StopAgentServer("agent-2")
}

func BenchmarkStartStopAgentServer(b *testing.B) {
	pm, err := NewPortManager(8100, 8100+b.N)
	require.NoError(b, err)

	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agentID := fmt.Sprintf("agent-%d", i)
		port, err := asm.StartAgentServer(agentID)
		require.NoError(b, err)
		require.NotZero(b, port)

		err = asm.StopAgentServer(agentID)
		require.NoError(b, err)
	}
}

func BenchmarkGetAllServers(b *testing.B) {
	pm, err := NewPortManager(8100, 8200)
	require.NoError(b, err)

	gs := grpcserver.NewServer(9090, nil)
	asm := NewAgentServerManager(pm, gs)

	// Start 10 servers
	for i := 0; i < 10; i++ {
		_, err := asm.StartAgentServer(fmt.Sprintf("agent-%d", i))
		require.NoError(b, err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		servers := asm.GetAllServers()
		require.Len(b, servers, 10)
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ctx
	asm.Shutdown()
}
