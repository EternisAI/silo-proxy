package server

import (
	"context"
	"testing"
	"time"

	"github.com/EternisAI/silo-proxy/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

// MockAgentServerManager is a mock implementation of AgentServerManager
type MockAgentServerManager struct {
	mock.Mock
}

func (m *MockAgentServerManager) StartAgentServer(agentID string) (int, error) {
	args := m.Called(agentID)
	return args.Int(0), args.Error(1)
}

func (m *MockAgentServerManager) StopAgentServer(agentID string) error {
	args := m.Called(agentID)
	return args.Error(0)
}

func (m *MockAgentServerManager) Shutdown() error {
	args := m.Called()
	return args.Error(0)
}

// MockStream is a mock implementation of proto.ProxyService_StreamServer
type MockStream struct {
	mock.Mock
	ctx context.Context
}

func NewMockStream() *MockStream {
	return &MockStream{
		ctx: context.Background(),
	}
}

func (m *MockStream) Send(*proto.ProxyMessage) error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockStream) Recv() (*proto.ProxyMessage, error) {
	args := m.Called()
	return nil, args.Error(0)
}

func (m *MockStream) SetHeader(md metadata.MD) error {
	return nil
}

func (m *MockStream) SendHeader(md metadata.MD) error {
	return nil
}

func (m *MockStream) SetTrailer(md metadata.MD) {
}

func (m *MockStream) Context() context.Context {
	return m.ctx
}

func (m *MockStream) SendMsg(msg interface{}) error {
	return nil
}

func (m *MockStream) RecvMsg(msg interface{}) error {
	return nil
}

func TestNewConnectionManager(t *testing.T) {
	cm := NewConnectionManager(nil, nil)
	assert.NotNil(t, cm)
	assert.NotNil(t, cm.agents)
	assert.NotNil(t, cm.stopCh)
	assert.Nil(t, cm.agentServerManager)

	cm.Stop()
}

func TestNewConnectionManager_WithAgentServerManager(t *testing.T) {
	mockASM := new(MockAgentServerManager)
	cm := NewConnectionManager(mockASM, nil)

	assert.NotNil(t, cm)
	assert.NotNil(t, cm.agentServerManager)

	mockASM.On("Shutdown").Return(nil)
	cm.Stop()
}

func TestConnectionManager_Register_WithoutServerManager(t *testing.T) {
	cm := NewConnectionManager(nil, nil)
	defer cm.Stop()

	mockStream := NewMockStream()
	conn, err := cm.Register("agent-1", mockStream)

	require.NoError(t, err)
	assert.NotNil(t, conn)
	assert.Equal(t, "agent-1", conn.ID)
	assert.Equal(t, 0, conn.Port) // No port allocated without server manager
	assert.NotNil(t, conn.SendCh)
}

func TestConnectionManager_Register_WithServerManager(t *testing.T) {
	mockASM := new(MockAgentServerManager)
	mockASM.On("StartAgentServer", "agent-1").Return(8100, nil)
	mockASM.On("StopAgentServer", "agent-1").Return(nil)
	mockASM.On("Shutdown").Return(nil)

	cm := NewConnectionManager(mockASM, nil)

	mockStream := NewMockStream()
	conn, err := cm.Register("agent-1", mockStream)

	require.NoError(t, err)
	assert.NotNil(t, conn)
	assert.Equal(t, "agent-1", conn.ID)
	assert.Equal(t, 8100, conn.Port)

	// Cleanup and verify mock expectations
	cm.Stop()
	mockASM.AssertExpectations(t)
}

func TestConnectionManager_Register_ServerManagerError(t *testing.T) {
	mockASM := new(MockAgentServerManager)
	mockASM.On("StartAgentServer", "agent-1").Return(0, assert.AnError)
	mockASM.On("Shutdown").Return(nil)

	cm := NewConnectionManager(mockASM, nil)

	mockStream := NewMockStream()
	conn, err := cm.Register("agent-1", mockStream)

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "failed to start agent HTTP server")

	// Cleanup and verify mock expectations
	cm.Stop()
	mockASM.AssertExpectations(t)
}

func TestConnectionManager_Register_ReplaceExisting(t *testing.T) {
	mockASM := new(MockAgentServerManager)
	mockASM.On("StartAgentServer", "agent-1").Return(8100, nil).Once()
	mockASM.On("StopAgentServer", "agent-1").Return(nil).Once() // for replacement
	mockASM.On("StartAgentServer", "agent-1").Return(8101, nil).Once()
	mockASM.On("StopAgentServer", "agent-1").Return(nil).Once() // for cm.Stop()
	mockASM.On("Shutdown").Return(nil)

	cm := NewConnectionManager(mockASM, nil)

	// Register first connection
	mockStream1 := NewMockStream()
	conn1, err := cm.Register("agent-1", mockStream1)
	require.NoError(t, err)
	assert.Equal(t, 8100, conn1.Port)

	// Register second connection (should replace first)
	mockStream2 := NewMockStream()
	conn2, err := cm.Register("agent-1", mockStream2)
	require.NoError(t, err)
	assert.Equal(t, 8101, conn2.Port)

	// Verify only one connection exists
	connections := cm.ListConnections()
	assert.Len(t, connections, 1)

	// Cleanup and verify mock expectations
	cm.Stop()
	mockASM.AssertExpectations(t)
}

func TestConnectionManager_Deregister_WithoutServerManager(t *testing.T) {
	cm := NewConnectionManager(nil, nil)
	defer cm.Stop()

	mockStream := NewMockStream()
	conn, err := cm.Register("agent-1", mockStream)
	require.NoError(t, err)
	require.NotNil(t, conn)

	cm.Deregister("agent-1")

	// Verify connection removed
	_, ok := cm.GetConnection("agent-1")
	assert.False(t, ok)
}

func TestConnectionManager_Deregister_WithServerManager(t *testing.T) {
	mockASM := new(MockAgentServerManager)
	mockASM.On("StartAgentServer", "agent-1").Return(8100, nil)
	mockASM.On("StopAgentServer", "agent-1").Return(nil)

	cm := NewConnectionManager(mockASM, nil)
	defer func() {
		mockASM.On("Shutdown").Return(nil)
		cm.Stop()
	}()

	mockStream := NewMockStream()
	conn, err := cm.Register("agent-1", mockStream)
	require.NoError(t, err)
	require.NotNil(t, conn)

	cm.Deregister("agent-1")

	// Verify connection removed
	_, ok := cm.GetConnection("agent-1")
	assert.False(t, ok)

	mockASM.AssertExpectations(t)
}

func TestConnectionManager_Deregister_NonExistent(t *testing.T) {
	cm := NewConnectionManager(nil, nil)
	defer cm.Stop()

	// Should not panic
	cm.Deregister("non-existent")
}

func TestConnectionManager_Stop_WithServerManager(t *testing.T) {
	mockASM := new(MockAgentServerManager)
	mockASM.On("StartAgentServer", "agent-1").Return(8100, nil)
	mockASM.On("StartAgentServer", "agent-2").Return(8101, nil)
	mockASM.On("StopAgentServer", "agent-1").Return(nil)
	mockASM.On("StopAgentServer", "agent-2").Return(nil)
	mockASM.On("Shutdown").Return(nil)

	cm := NewConnectionManager(mockASM, nil)

	mockStream1 := NewMockStream()
	_, err := cm.Register("agent-1", mockStream1)
	require.NoError(t, err)

	mockStream2 := NewMockStream()
	_, err = cm.Register("agent-2", mockStream2)
	require.NoError(t, err)

	cm.Stop()

	// Verify all connections removed
	connections := cm.ListConnections()
	assert.Len(t, connections, 0)

	mockASM.AssertExpectations(t)
}

func TestConnectionManager_GetConnection(t *testing.T) {
	cm := NewConnectionManager(nil, nil)
	defer cm.Stop()

	mockStream := NewMockStream()
	registered, err := cm.Register("agent-1", mockStream)
	require.NoError(t, err)

	conn, ok := cm.GetConnection("agent-1")
	assert.True(t, ok)
	assert.Equal(t, registered, conn)

	_, ok = cm.GetConnection("non-existent")
	assert.False(t, ok)
}

func TestConnectionManager_ListConnections(t *testing.T) {
	cm := NewConnectionManager(nil, nil)
	defer cm.Stop()

	// Initially empty
	connections := cm.ListConnections()
	assert.Len(t, connections, 0)

	// Register 3 agents
	for i := 1; i <= 3; i++ {
		mockStream := NewMockStream()
		agentID := "agent-" + string(rune('0'+i))
		_, err := cm.Register(agentID, mockStream)
		require.NoError(t, err)
	}

	connections = cm.ListConnections()
	assert.Len(t, connections, 3)
	assert.Contains(t, connections, "agent-1")
	assert.Contains(t, connections, "agent-2")
	assert.Contains(t, connections, "agent-3")
}

func TestConnectionManager_UpdateLastSeen(t *testing.T) {
	cm := NewConnectionManager(nil, nil)
	defer cm.Stop()

	mockStream := NewMockStream()
	conn, err := cm.Register("agent-1", mockStream)
	require.NoError(t, err)

	initialTime := conn.LastSeen

	// Wait a bit and update
	time.Sleep(10 * time.Millisecond)
	cm.UpdateLastSeen("agent-1")

	// Verify last seen updated
	updatedConn, ok := cm.GetConnection("agent-1")
	require.True(t, ok)
	assert.True(t, updatedConn.LastSeen.After(initialTime))
}

func TestConnectionManager_RemoveStaleConnections_WithServerManager(t *testing.T) {
	// This test is tricky because it requires waiting for stale timeout
	// We'll test the logic by directly calling removeStaleConnections

	mockASM := new(MockAgentServerManager)
	mockASM.On("StartAgentServer", "agent-1").Return(8100, nil)
	mockASM.On("StopAgentServer", "agent-1").Return(nil)

	cm := NewConnectionManager(mockASM, nil)
	defer func() {
		mockASM.On("Shutdown").Return(nil)
		cm.Stop()
	}()

	mockStream := NewMockStream()
	conn, err := cm.Register("agent-1", mockStream)
	require.NoError(t, err)

	// Manually set LastSeen to past
	cm.mu.Lock()
	conn.LastSeen = time.Now().Add(-3 * time.Minute) // Beyond staleConnectionTimeout
	cm.mu.Unlock()

	// Trigger cleanup
	cm.removeStaleConnections()

	// Verify connection removed
	_, ok := cm.GetConnection("agent-1")
	assert.False(t, ok)

	mockASM.AssertExpectations(t)
}

func TestConnectionManager_ConcurrentRegistration(t *testing.T) {
	mockASM := new(MockAgentServerManager)

	// Setup expectations for 10 agents (start and stop)
	for i := 0; i < 10; i++ {
		agentID := "agent-" + string(rune('0'+i))
		port := 8100 + i
		mockASM.On("StartAgentServer", agentID).Return(port, nil)
		mockASM.On("StopAgentServer", agentID).Return(nil)
	}
	mockASM.On("Shutdown").Return(nil)

	cm := NewConnectionManager(mockASM, nil)

	// Register 10 agents concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			agentID := "agent-" + string(rune('0'+id))
			mockStream := NewMockStream()
			_, err := cm.Register(agentID, mockStream)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all registered
	connections := cm.ListConnections()
	assert.Len(t, connections, 10)

	// Cleanup and verify mock expectations
	cm.Stop()
	mockASM.AssertExpectations(t)
}

func TestConnectionManager_PortFieldPersistence(t *testing.T) {
	mockASM := new(MockAgentServerManager)
	mockASM.On("StartAgentServer", "agent-1").Return(8100, nil)
	mockASM.On("StopAgentServer", "agent-1").Return(nil)
	mockASM.On("Shutdown").Return(nil)

	cm := NewConnectionManager(mockASM, nil)

	mockStream := NewMockStream()
	_, err := cm.Register("agent-1", mockStream)
	require.NoError(t, err)

	// Retrieve connection and verify port
	conn, ok := cm.GetConnection("agent-1")
	require.True(t, ok)
	assert.Equal(t, 8100, conn.Port)

	// Update last seen and verify port still there
	cm.UpdateLastSeen("agent-1")
	conn, ok = cm.GetConnection("agent-1")
	require.True(t, ok)
	assert.Equal(t, 8100, conn.Port)

	// Cleanup and verify mock expectations
	cm.Stop()
	mockASM.AssertExpectations(t)
}

func TestAgentConnection_PortFieldZeroWithoutManager(t *testing.T) {
	cm := NewConnectionManager(nil, nil)
	defer cm.Stop()

	mockStream := NewMockStream()
	conn, err := cm.Register("agent-1", mockStream)
	require.NoError(t, err)

	// Port should be 0 when no server manager
	assert.Equal(t, 0, conn.Port)
}

func BenchmarkConnectionManager_Register(b *testing.B) {
	cm := NewConnectionManager(nil, nil)
	defer cm.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mockStream := NewMockStream()
		agentID := "agent-bench"
		cm.Register(agentID, mockStream)
		cm.Deregister(agentID)
	}
}

func BenchmarkConnectionManager_RegisterWithServerManager(b *testing.B) {
	mockASM := new(MockAgentServerManager)
	mockASM.On("StartAgentServer", mock.Anything).Return(8100, nil)
	mockASM.On("StopAgentServer", mock.Anything).Return(nil)
	mockASM.On("Shutdown").Return(nil)

	cm := NewConnectionManager(mockASM, nil)
	defer cm.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mockStream := NewMockStream()
		agentID := "agent-bench"
		cm.Register(agentID, mockStream)
		cm.Deregister(agentID)
	}
}
