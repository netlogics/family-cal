package calendar

import "time"

type RecurrenceFrequency string

const (
	FreqDaily   RecurrenceFrequency = "daily"
	FreqWeekly  RecurrenceFrequency = "weekly"
	FreqMonthly RecurrenceFrequency = "monthly"
	FreqYearly  RecurrenceFrequency = "yearly"
)

type RecurrenceRule struct {
	ID        int64
	Frequency RecurrenceFrequency
	Interval  int
	Weekdays  int // bitmask: Sun=1, Mon=2, Tue=4, Wed=8, Thu=16, Fri=32, Sat=64
	Until     *time.Time
}

type Event struct {
	ID           int64
	Title        string
	Description  string
	StartAt      time.Time
	EndAt        time.Time
	AllDay       bool
	CreatorID    int64
	ColorOverride string
	RecurrenceID  *int64
	Recurrence   *RecurrenceRule
	CreatedAt    time.Time
}

// EventInstance is a concrete (possibly virtual) occurrence of an event.
// For recurring events the ID is "base-YYYY-MM-DD".
type EventInstance struct {
	ID            string
	BaseID        int64
	Title         string
	Description   string
	StartAt       time.Time
	EndAt         time.Time
	AllDay        bool
	CreatorID     int64
	ColorOverride string
	IsRecurring   bool
	OccurrenceDate string // YYYY-MM-DD, for recurring instances
}

func weekdayBit(d time.Weekday) int {
	return 1 << uint(d)
}
