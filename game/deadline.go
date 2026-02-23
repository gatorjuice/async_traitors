package game

import (
	"fmt"
	"time"
)

// ParseDeadline parses a deadline string in several formats. Non-RFC3339 formats
// are interpreted in tz (falls back to UTC if empty or invalid).
func ParseDeadline(s, tz string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}

	loc := time.UTC
	if tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}

	for _, f := range []string{"2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(f, s, loc); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized deadline format: %s", s)
}

// CalculatedTimers holds the per-phase timer values computed from a deadline.
type CalculatedTimers struct {
	BreakfastMinutes  int
	RoundtableMinutes int
	NightMinutes      int
	MissionMinutes    int
	IsTight           bool // phases hit their floor values
	IsTooTight        bool // not enough time even with floor values
}

// Phase time ratios (must sum to 17).
const (
	ratioBreakfast  = 8
	ratioRoundtable = 4
	ratioNight      = 4
	ratioMission    = 1
	ratioSum        = ratioBreakfast + ratioRoundtable + ratioNight + ratioMission

	floorBreakfast  = 15
	floorRoundtable = 15
	floorNight      = 10
	floorMission    = 5
	floorSum        = floorBreakfast + floorRoundtable + floorNight + floorMission
)

// EstimateRounds returns a conservative estimate of rounds needed to finish
// a game with playerCount players.
func EstimateRounds(playerCount int) int {
	r := playerCount/2 + 1
	if r < 2 {
		r = 2
	}
	return r
}

// AvailableActiveMinutes calculates the total non-hiatus minutes between from and to.
// This is the inverse of EffectiveWallDuration: it walks the time range and subtracts
// hiatus periods.
func AvailableActiveMinutes(from, to time.Time, hiatusStart, hiatusEnd, tz string) int {
	if !to.After(from) {
		return 0
	}

	if hiatusStart == "" || hiatusEnd == "" {
		return int(to.Sub(from).Minutes())
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		return int(to.Sub(from).Minutes())
	}

	startMin := parseHHMM(hiatusStart)
	endMin := parseHHMM(hiatusEnd)
	if startMin < 0 || endMin < 0 {
		return int(to.Sub(from).Minutes())
	}

	active := 0
	cursor := from

	for cursor.Before(to) {
		if IsInHiatus(hiatusStart, hiatusEnd, tz, cursor) {
			// Skip to end of hiatus.
			local := cursor.In(loc)
			hiatusEndTime := time.Date(local.Year(), local.Month(), local.Day(), endMin/60, endMin%60, 0, 0, loc)
			if !hiatusEndTime.After(local) {
				hiatusEndTime = hiatusEndTime.AddDate(0, 0, 1)
			}
			cursor = hiatusEndTime
			continue
		}

		// Calculate how many active minutes until next hiatus or until 'to'.
		local := cursor.In(loc)
		nowMin := local.Hour()*60 + local.Minute()

		var minutesToHiatus int
		if startMin > nowMin {
			minutesToHiatus = startMin - nowMin
		} else if startMin < nowMin {
			minutesToHiatus = (24*60 - nowMin) + startMin
		} else {
			// Exactly at hiatus start.
			cursor = cursor.Add(time.Minute)
			continue
		}

		chunkEnd := cursor.Add(time.Duration(minutesToHiatus) * time.Minute)
		if chunkEnd.After(to) {
			chunkEnd = to
		}

		active += int(chunkEnd.Sub(cursor).Minutes())
		cursor = chunkEnd
	}

	return active
}

// CalculateTimersFromDeadline computes per-phase timer durations so the game
// finishes by deadline, respecting hiatus windows.
func CalculateTimersFromDeadline(now, deadline time.Time, playerCount int, hiatusStart, hiatusEnd, tz string) CalculatedTimers {
	availableMinutes := AvailableActiveMinutes(now, deadline, hiatusStart, hiatusEnd, tz)
	rounds := EstimateRounds(playerCount)
	minutesPerRound := availableMinutes / rounds

	breakfast := minutesPerRound * ratioBreakfast / ratioSum
	roundtable := minutesPerRound * ratioRoundtable / ratioSum
	night := minutesPerRound * ratioNight / ratioSum
	mission := minutesPerRound * ratioMission / ratioSum

	tight := false

	// Apply floors.
	if breakfast < floorBreakfast {
		breakfast = floorBreakfast
		tight = true
	}
	if roundtable < floorRoundtable {
		roundtable = floorRoundtable
		tight = true
	}
	if night < floorNight {
		night = floorNight
		tight = true
	}
	if mission < floorMission {
		mission = floorMission
		tight = true
	}

	// Check if even floors exceed budget.
	tooTight := rounds*floorSum > availableMinutes

	return CalculatedTimers{
		BreakfastMinutes:  breakfast,
		RoundtableMinutes: roundtable,
		NightMinutes:      night,
		MissionMinutes:    mission,
		IsTight:           tight,
		IsTooTight:        tooTight,
	}
}
