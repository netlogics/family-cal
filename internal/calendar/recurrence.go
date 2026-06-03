package calendar

import (
	"fmt"
	"time"
)

// expand returns all occurrence start times for a recurring event between from and to (inclusive).
func expand(base Event, rule RecurrenceRule, from, to time.Time) []time.Time {
	var occurrences []time.Time
	duration := base.EndAt.Sub(base.StartAt)

	cursor := base.StartAt
	limit := to
	if rule.Until != nil && rule.Until.Before(limit) {
		limit = *rule.Until
	}

	interval := rule.Interval
	if interval < 1 {
		interval = 1
	}

	for !cursor.After(limit) {
		if !cursor.Before(from) {
			if rule.Frequency == FreqWeekly && rule.Weekdays != 0 {
				// For weekly with specific weekdays, check each day in the current week window
				_ = duration
			} else {
				occurrences = append(occurrences, cursor)
			}
		}

		switch rule.Frequency {
		case FreqDaily:
			cursor = cursor.AddDate(0, 0, interval)
		case FreqWeekly:
			if rule.Weekdays != 0 {
				cursor = nextWeekdayOccurrence(cursor, rule.Weekdays, interval, from, limit, &occurrences)
				return occurrences
			}
			cursor = cursor.AddDate(0, 0, 7*interval)
		case FreqMonthly:
			cursor = cursor.AddDate(0, interval, 0)
		case FreqYearly:
			cursor = cursor.AddDate(interval, 0, 0)
		default:
			return occurrences
		}
	}
	return occurrences
}

// nextWeekdayOccurrence handles weekly recurrence with a weekday bitmask,
// writing matching occurrences into out and returning a sentinel past limit.
func nextWeekdayOccurrence(start time.Time, weekdays, interval int, from, limit time.Time, out *[]time.Time) time.Time {
	// Walk day by day from the start of the first week; advance by interval weeks each cycle.
	weekStart := start.Truncate(24 * time.Hour)
	// Align to the start of that week (Sunday).
	for weekStart.Weekday() != time.Sunday {
		weekStart = weekStart.AddDate(0, 0, -1)
	}

	for !weekStart.After(limit) {
		for d := 0; d < 7; d++ {
			day := weekStart.AddDate(0, 0, d)
			if day.Before(start) {
				continue
			}
			if day.After(limit) {
				return limit.AddDate(0, 0, 1)
			}
			if weekdays&weekdayBit(day.Weekday()) != 0 {
				if !day.Before(from) {
					*out = append(*out, day)
				}
			}
		}
		weekStart = weekStart.AddDate(0, 0, 7*interval)
	}
	return limit.AddDate(0, 0, 1)
}

func occurrenceID(baseID int64, t time.Time) string {
	return fmt.Sprintf("%d-%s", baseID, t.Format("2006-01-02"))
}
