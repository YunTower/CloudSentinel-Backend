package services

import (
	"fmt"
	"sync"
	"time"
)

// serviceMonitorRounds is the internal implementation of the monitoring-round
// seam. It owns each round's deadline and aggregation membership, so callers
// can only commit a monitor status after the whole round is complete.
type serviceMonitorRounds struct {
	mu      sync.Mutex
	pending map[string]*pendingServiceCheck
	latest  map[uint]string
}

type pendingServiceCheck struct {
	monitorID uint
	checkID   string
	commandID string
	expected  map[string]struct{}
	results   map[string]probeCheckResult
	timer     *time.Timer
}

func newServiceMonitorRounds() *serviceMonitorRounds {
	return &serviceMonitorRounds{
		pending: make(map[string]*pendingServiceCheck),
		latest:  make(map[uint]string),
	}
}

func (r *serviceMonitorRounds) Open(monitorID uint, checkID, commandID string, serverIDs []string, timeoutSec int, onDeadline func()) {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	expected := make(map[string]struct{}, len(serverIDs))
	for _, serverID := range serverIDs {
		if serverID != "" {
			expected[serverID] = struct{}{}
		}
	}
	pending := &pendingServiceCheck{
		monitorID: monitorID,
		checkID:   checkID,
		commandID: commandID,
		expected:  expected,
		results:   make(map[string]probeCheckResult, len(expected)),
	}
	pending.timer = time.AfterFunc(time.Duration(timeoutSec+2)*time.Second, onDeadline)

	key := pendingCheckKey(monitorID, checkID)
	r.mu.Lock()
	if old := r.pending[key]; old != nil && old.timer != nil {
		old.timer.Stop()
	}
	r.pending[key] = pending
	r.latest[monitorID] = checkID
	r.mu.Unlock()
}

func (r *serviceMonitorRounds) IsLatest(monitorID uint, checkID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.latest[monitorID] == checkID
}

// AddResult returns true only when all expected probes have reported. Results
// from a late round or an unexpected probe are intentionally ignored here; the
// caller still persists them as raw probe records.
func (r *serviceMonitorRounds) AddResult(monitorID uint, checkID string, result probeCheckResult) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	pending := r.pending[pendingCheckKey(monitorID, checkID)]
	if pending == nil {
		return false
	}
	if _, expected := pending.expected[result.probeID]; !expected {
		return false
	}
	pending.results[result.probeID] = result
	return len(pending.expected) > 0 && len(pending.results) == len(pending.expected)
}

// Complete removes a round exactly once. This gives the deadline callback and
// the final Agent result a single winner and prevents late results from
// mutating a completed monitor status.
func (r *serviceMonitorRounds) Complete(monitorID uint, checkID string) *pendingServiceCheck {
	key := pendingCheckKey(monitorID, checkID)
	r.mu.Lock()
	defer r.mu.Unlock()

	pending := r.pending[key]
	if pending == nil {
		return nil
	}
	delete(r.pending, key)
	if pending.timer != nil {
		pending.timer.Stop()
	}
	return pending
}

func (r *serviceMonitorRounds) CancelMonitor(monitorID uint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, pending := range r.pending {
		if pending.monitorID != monitorID {
			continue
		}
		if pending.timer != nil {
			pending.timer.Stop()
		}
		delete(r.pending, key)
	}
	delete(r.latest, monitorID)
}

func pendingCheckKey(monitorID uint, checkID string) string {
	return fmt.Sprintf("%d:%s", monitorID, checkID)
}
