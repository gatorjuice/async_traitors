package game

import "time"

// IsInHiatus returns true if the given time falls within the quiet-hours window.
// Returns false if hiatus is not configured (empty strings).
func IsInHiatus(hiatusStart, hiatusEnd, tz string, now time.Time) bool {
	if hiatusStart == "" || hiatusEnd == "" {
		return false
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		return false
	}

	local := now.In(loc)
	startMin := parseHHMM(hiatusStart)
	endMin := parseHHMM(hiatusEnd)
	if startMin < 0 || endMin < 0 {
		return false
	}

	nowMin := local.Hour()*60 + local.Minute()

	if startMin < endMin {
		// e.g. 02:00 - 07:00
		return nowMin >= startMin && nowMin < endMin
	}
	// midnight wrap, e.g. 22:00 - 07:00
	return nowMin >= startMin || nowMin < endMin
}

// TimeUntilHiatusEnd returns how long until hiatus ends from the given time.
// Returns 0 if not in hiatus or hiatus is not configured.
func TimeUntilHiatusEnd(hiatusStart, hiatusEnd, tz string, now time.Time) time.Duration {
	if !IsInHiatus(hiatusStart, hiatusEnd, tz, now) {
		return 0
	}

	loc, _ := time.LoadLocation(tz)
	local := now.In(loc)
	endMin := parseHHMM(hiatusEnd)

	endToday := time.Date(local.Year(), local.Month(), local.Day(), endMin/60, endMin%60, 0, 0, loc)
	if !endToday.After(now.In(loc)) {
		// End is tomorrow (midnight wrap case).
		endToday = endToday.AddDate(0, 0, 1)
	}
	return endToday.Sub(now)
}

// EffectiveWallDuration calculates the total wall-clock duration needed to accumulate
// activeDuration of non-hiatus time, starting from start. If hiatus is not configured,
// returns activeDuration unchanged.
func EffectiveWallDuration(start time.Time, activeDuration time.Duration, hiatusStart, hiatusEnd, tz string) time.Duration {
	if hiatusStart == "" || hiatusEnd == "" {
		return activeDuration
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		return activeDuration
	}

	startMin := parseHHMM(hiatusStart)
	endMin := parseHHMM(hiatusEnd)
	if startMin < 0 || endMin < 0 {
		return activeDuration
	}

	remaining := activeDuration
	cursor := start
	const step = time.Minute

	for remaining > 0 {
		if IsInHiatus(hiatusStart, hiatusEnd, tz, cursor) {
			// Skip to end of hiatus.
			local := cursor.In(loc)
			endToday := time.Date(local.Year(), local.Month(), local.Day(), endMin/60, endMin%60, 0, 0, loc)
			if !endToday.After(local) {
				endToday = endToday.AddDate(0, 0, 1)
			}
			cursor = endToday
			continue
		}

		// Calculate how much active time until the next hiatus starts.
		local := cursor.In(loc)
		nowMin := local.Hour()*60 + local.Minute()

		var minutesToHiatus int
		if startMin > nowMin {
			minutesToHiatus = startMin - nowMin
		} else if startMin < nowMin {
			// Hiatus start is tomorrow.
			minutesToHiatus = (24*60 - nowMin) + startMin
		} else {
			// Exactly at hiatus start.
			minutesToHiatus = 0
		}

		activeChunk := time.Duration(minutesToHiatus) * time.Minute
		if activeChunk <= 0 {
			// Edge case: cursor is exactly at hiatus start boundary.
			// Advance 1 minute into hiatus so the next iteration skips it.
			cursor = cursor.Add(step)
			continue
		}

		if activeChunk >= remaining {
			// All remaining active time fits before next hiatus.
			cursor = cursor.Add(remaining)
			remaining = 0
		} else {
			// Consume what we can, then hiatus kicks in.
			remaining -= activeChunk
			cursor = cursor.Add(activeChunk)
		}
	}

	return cursor.Sub(start)
}

// parseHHMM parses "HH:MM" into minutes since midnight. Returns -1 on error.
func parseHHMM(s string) int {
	if len(s) != 5 || s[2] != ':' {
		return -1
	}
	h := (int(s[0]-'0') * 10) + int(s[1]-'0')
	m := (int(s[3]-'0') * 10) + int(s[4]-'0')
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return -1
	}
	return h*60 + m
}
