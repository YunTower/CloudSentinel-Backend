package websocket

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManagerOptionsInitialStateAndCopyIsolation(t *testing.T) {
	notifier := func(string, bool) {}
	m := NewConnectionManager(WithServerStatusNotifier(notifier)).(*connectionManager)
	if m.oldConnectionCloseDelay != 2*time.Second || m.offlineGracePeriod != 30*time.Second || m.onServerStatusChange == nil {
		t.Fatalf("manager=%+v", m)
	}
	if m.GetAgentConnectionCount() != 0 || m.GetFrontendConnectionCount() != 0 {
		t.Fatal("new manager not empty")
	}
	copyAgents := m.GetAllAgentConnections()
	copyAgents["x"] = nil
	copyFrontends := m.GetAllFrontendConnections()
	copyFrontends["x"] = nil
	if m.GetAgentConnectionCount() != 0 || m.GetFrontendConnectionCount() != 0 {
		t.Fatal("returned maps exposed manager storage")
	}
}

func TestManagerLookupSendAndConnectionErrors(t *testing.T) {
	m := NewConnectionManager().(*connectionManager)
	if _, ok := m.GetAgentConnection("missing"); ok {
		t.Fatal("missing agent found")
	}
	if err := m.SendToAgent("missing", map[string]any{}); !errors.Is(err, ErrConnectionNotFound) {
		t.Fatalf("err=%v", err)
	}
	closed := NewAgentConnection(nil, DefaultConfig())
	closed.SetState(StateClosed)
	m.agentConnections["closed"] = closed
	if err := m.SendToAgent("closed", map[string]any{}); !errors.Is(err, ErrConnectionClosed) {
		t.Fatalf("err=%v", err)
	}
	if ErrConnectionNotFound.Error() != "连接不存在" || ErrConnectionClosed.Error() != "连接已关闭" {
		t.Fatal("error text mismatch")
	}
}

func TestManagerPingAndHeartbeatCheckerStopWithContext(t *testing.T) {
	m := NewConnectionManager().(*connectionManager)
	agent := NewAgentConnection(nil, DefaultConfig())
	frontend := NewFrontendConnection(nil, DefaultConfig())
	m.agentConnections["a"] = agent
	m.frontendConnections["f"] = frontend
	agentBefore := agent.GetLastPing()
	frontendBefore := frontend.GetLastPing()
	time.Sleep(time.Millisecond)
	m.UpdateAgentPing("a")
	m.UpdateFrontendPing("f")
	if !agent.GetLastPing().After(agentBefore) || !frontend.GetLastPing().After(frontendBefore) {
		t.Fatal("ping not updated")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.StartHeartbeatChecker(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("heartbeat checker did not stop")
	}
}

func TestFrontendRetrievalAndRemovalState(t *testing.T) {
	m := NewConnectionManager().(*connectionManager)
	conn := NewFrontendConnection(nil, DefaultConfig())
	conn.SetRemoteAddr("1.2.3.4")
	// Registration logging depends on a booted Goravel container; isolate the
	// manager's public lookup/count state here without faking framework internals.
	m.frontendConnections["f"] = conn
	if got, ok := m.GetFrontendConnection("f"); !ok || got != conn || m.GetFrontendConnectionCount() != 1 {
		t.Fatal("frontend not registered")
	}
	// Avoid nil socket Close in UnregisterFrontend: mark closed and remove directly tests lookup lifecycle.
	m.frontendMutex.Lock()
	delete(m.frontendConnections, "f")
	m.frontendMutex.Unlock()
	if _, ok := m.GetFrontendConnection("f"); ok || m.GetFrontendConnectionCount() != 0 {
		t.Fatal("frontend not removed")
	}
}
