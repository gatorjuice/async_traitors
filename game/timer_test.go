package game

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestTimerExpires(t *testing.T) {
	tm := NewTimerManager()
	var fired atomic.Bool

	tm.StartTimer(1, 50*time.Millisecond, func() {
		fired.Store(true)
	})

	time.Sleep(150 * time.Millisecond)
	if !fired.Load() {
		t.Error("expected timer to fire")
	}
}

func TestTimerCancel(t *testing.T) {
	tm := NewTimerManager()
	var fired atomic.Bool

	tm.StartTimer(1, 50*time.Millisecond, func() {
		fired.Store(true)
	})

	tm.CancelTimer(1)
	time.Sleep(150 * time.Millisecond)

	if fired.Load() {
		t.Error("expected timer NOT to fire after cancel")
	}
}

func TestTimerReplace(t *testing.T) {
	tm := NewTimerManager()
	var first atomic.Bool
	var second atomic.Bool

	tm.StartTimer(1, 100*time.Millisecond, func() {
		first.Store(true)
	})

	// Replace with a shorter timer
	tm.StartTimer(1, 50*time.Millisecond, func() {
		second.Store(true)
	})

	time.Sleep(200 * time.Millisecond)

	if first.Load() {
		t.Error("expected first timer NOT to fire")
	}
	if !second.Load() {
		t.Error("expected second timer to fire")
	}
}

func TestScheduleCallback_Fires(t *testing.T) {
	tm := NewTimerManager()
	var mainFired atomic.Bool
	var callbackFired atomic.Bool

	tm.StartTimer(1, 200*time.Millisecond, func() {
		mainFired.Store(true)
	})

	tm.ScheduleCallback(1, 50*time.Millisecond, func() {
		callbackFired.Store(true)
	})

	time.Sleep(100 * time.Millisecond)
	if !callbackFired.Load() {
		t.Error("expected callback to fire")
	}

	time.Sleep(200 * time.Millisecond)
	if !mainFired.Load() {
		t.Error("expected main timer to fire")
	}
}

func TestScheduleCallback_CancelledWithTimer(t *testing.T) {
	tm := NewTimerManager()
	var callbackFired atomic.Bool

	tm.StartTimer(1, 200*time.Millisecond, func() {})

	tm.ScheduleCallback(1, 100*time.Millisecond, func() {
		callbackFired.Store(true)
	})

	tm.CancelTimer(1)
	time.Sleep(200 * time.Millisecond)

	if callbackFired.Load() {
		t.Error("expected callback NOT to fire after cancel")
	}
}

func TestScheduleCallback_NoTimer(t *testing.T) {
	tm := NewTimerManager()
	// Should not panic when no timer exists.
	tm.ScheduleCallback(999, 50*time.Millisecond, func() {})
}
