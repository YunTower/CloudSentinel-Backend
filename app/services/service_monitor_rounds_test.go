package services

import (
	"errors"
	"testing"
)

func TestServiceMonitorRoundsCompletesOnlyAfterEveryExpectedProbe(t *testing.T) {
	rounds := newServiceMonitorRounds()
	rounds.Open(7, "round-1", "command-1", []string{"agent-a", "agent-b"}, 10, func() {})

	if rounds.AddResult(7, "round-1", probeCheckResult{probeID: "agent-a", status: "up"}) {
		t.Fatal("a partial monitoring round must not complete")
	}
	if !rounds.AddResult(7, "round-1", probeCheckResult{probeID: "agent-b", status: "up"}) {
		t.Fatal("the final expected probe must complete the monitoring round")
	}

	round := rounds.Complete(7, "round-1")
	if round == nil || len(round.results) != 2 {
		t.Fatalf("expected both probe results, got %#v", round)
	}
}

func TestServiceMonitorRoundsIgnoresLateAndUnexpectedResults(t *testing.T) {
	rounds := newServiceMonitorRounds()
	rounds.Open(7, "round-2", "command-2", []string{"agent-a"}, 10, func() {})

	if rounds.AddResult(7, "round-2", probeCheckResult{probeID: "agent-x", status: "down", err: errors.New("unexpected")}) {
		t.Fatal("an unexpected probe must not complete the monitoring round")
	}
	if round := rounds.Complete(7, "round-2"); round == nil || len(round.results) != 0 {
		t.Fatalf("unexpected probe must not be included, got %#v", round)
	}
	if rounds.AddResult(7, "round-2", probeCheckResult{probeID: "agent-a", status: "up"}) {
		t.Fatal("a result arriving after completion must not reopen the monitoring round")
	}
}

func TestServiceMonitorRoundsCancelMonitorRemovesAllRounds(t *testing.T) {
	rounds := newServiceMonitorRounds()
	rounds.Open(7, "round-a", "command-a", []string{"agent-a"}, 10, func() {})
	rounds.Open(7, "round-b", "command-b", []string{"agent-b"}, 10, func() {})
	rounds.Open(8, "round-c", "command-c", []string{"agent-c"}, 10, func() {})

	rounds.CancelMonitor(7)
	if rounds.Complete(7, "round-a") != nil || rounds.Complete(7, "round-b") != nil {
		t.Fatal("stopping a monitor must cancel all of its monitoring rounds")
	}
	if rounds.Complete(8, "round-c") == nil {
		t.Fatal("stopping one monitor must not cancel another monitor's round")
	}
}

func TestServiceMonitorRoundsOnlyNewestRoundCanCommit(t *testing.T) {
	rounds := newServiceMonitorRounds()
	rounds.Open(7, "round-old", "command-old", []string{"agent-a"}, 10, func() {})
	rounds.Open(7, "round-new", "command-new", []string{"agent-a"}, 10, func() {})

	if rounds.IsLatest(7, "round-old") {
		t.Fatal("an older monitoring round must not be allowed to overwrite a newer round")
	}
	if !rounds.IsLatest(7, "round-new") {
		t.Fatal("the newest monitoring round must be allowed to commit")
	}
}

func TestServiceMonitorRoundsCancelAllRemovesEveryRound(t *testing.T) {
	rounds := newServiceMonitorRounds()
	rounds.Open(7, "round-a", "command-a", []string{"agent-a"}, 10, func() {})
	rounds.Open(8, "round-b", "command-b", []string{"agent-b"}, 10, func() {})

	rounds.CancelAll()
	if rounds.Complete(7, "round-a") != nil || rounds.Complete(8, "round-b") != nil {
		t.Fatal("应用关闭时必须取消全部监测轮次")
	}
	if rounds.IsLatest(7, "round-a") || rounds.IsLatest(8, "round-b") {
		t.Fatal("应用关闭后不能保留最新轮次标记")
	}
}
