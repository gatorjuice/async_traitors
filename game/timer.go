package game

import (
	"context"
	"sync"
	"time"
)

// gameTimerSet holds a shared cancellable context for all timer goroutines of a game.
type gameTimerSet struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// TimerManager manages per-game phase timers.
type TimerManager struct {
	mu     sync.Mutex
	timers map[int64]*gameTimerSet
}

// NewTimerManager creates a new TimerManager.
func NewTimerManager() *TimerManager {
	return &TimerManager{
		timers: make(map[int64]*gameTimerSet),
	}
}

// StartTimer starts a timer for a game that calls onExpire when it finishes.
// Cancels any existing timers (including scheduled callbacks) for the game.
func (tm *TimerManager) StartTimer(gameID int64, duration time.Duration, onExpire func()) {
	tm.mu.Lock()
	if ts, ok := tm.timers[gameID]; ok {
		ts.cancel()
		delete(tm.timers, gameID)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ts := &gameTimerSet{ctx: ctx, cancel: cancel}
	tm.timers[gameID] = ts
	tm.mu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(duration):
			tm.mu.Lock()
			// Only clean up if this is still the active timer set.
			if current, ok := tm.timers[gameID]; ok && current == ts {
				delete(tm.timers, gameID)
			}
			tm.mu.Unlock()
			onExpire()
		}
	}()
}

// ScheduleCallback spawns an additional goroutine under the same game context.
// When CancelTimer is called, all goroutines (main timer + callbacks) are cancelled together.
// Does nothing if there is no active timer for the game.
func (tm *TimerManager) ScheduleCallback(gameID int64, delay time.Duration, callback func()) {
	tm.mu.Lock()
	ts, ok := tm.timers[gameID]
	tm.mu.Unlock()
	if !ok {
		return
	}

	go func() {
		select {
		case <-ts.ctx.Done():
			return
		case <-time.After(delay):
			callback()
		}
	}()
}

// CancelTimer cancels any active timer for the given game.
func (tm *TimerManager) CancelTimer(gameID int64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if ts, ok := tm.timers[gameID]; ok {
		ts.cancel()
		delete(tm.timers, gameID)
	}
}

// HasActiveTimer checks if a game has an active timer.
func (tm *TimerManager) HasActiveTimer(gameID int64) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	_, ok := tm.timers[gameID]
	return ok
}
