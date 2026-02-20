package game

import (
	"context"
	"sync"
	"time"
)

// TimerManager manages per-game phase timers.
type TimerManager struct {
	mu      sync.Mutex
	timers  map[int64]context.CancelFunc
}

// NewTimerManager creates a new TimerManager.
func NewTimerManager() *TimerManager {
	return &TimerManager{
		timers: make(map[int64]context.CancelFunc),
	}
}

// StartTimer starts a timer for a game that calls onExpire when it finishes.
func (tm *TimerManager) StartTimer(gameID int64, duration time.Duration, onExpire func()) {
	tm.mu.Lock()
	if cancel, ok := tm.timers[gameID]; ok {
		cancel()
		delete(tm.timers, gameID)
	}
	ctx, cancel := context.WithCancel(context.Background())
	tm.timers[gameID] = cancel
	tm.mu.Unlock()

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(duration):
			tm.mu.Lock()
			delete(tm.timers, gameID)
			tm.mu.Unlock()
			onExpire()
		}
	}()
}

// CancelTimer cancels any active timer for the given game.
func (tm *TimerManager) CancelTimer(gameID int64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if cancel, ok := tm.timers[gameID]; ok {
		cancel()
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
