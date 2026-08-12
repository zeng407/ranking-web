package posttrend

import (
	"errors"
	"testing"
	"time"
)

// The trap this package is most likely to be broken by. The Kernel runs
// `make:post-trend day`, the command maps that to createTodayPostTrends(), and the
// executor passes TrendTimeRange::TODAY — so the wire value "day" must become the
// stored value "today". Writing "day" into time_range would produce rows nothing
// reads while the real "today" trend stayed frozen.
func TestRangeFromScheduleArgumentMapsDayToToday(t *testing.T) {
	got, err := RangeFromScheduleArgument("day")
	if err != nil {
		t.Fatalf("RangeFromScheduleArgument(\"day\") error = %v", err)
	}
	if got != RangeToday {
		t.Fatalf("got %q, want %q", got, RangeToday)
	}
	if string(got) == "day" {
		t.Fatal("the stored value must not be the argument value")
	}
}

func TestRangeFromScheduleArgumentAcceptsEveryScheduledValue(t *testing.T) {
	// The four arguments the Kernel actually schedules, plus year which the command
	// supports but nothing schedules.
	for argument, want := range map[string]TimeRange{
		"all":   RangeAll,
		"month": RangeMonth,
		"week":  RangeWeek,
		"day":   RangeToday,
		"year":  RangeYear,
		// Accepted so a message carrying the stored value round-trips.
		"today": RangeToday,
	} {
		got, err := RangeFromScheduleArgument(argument)
		if err != nil {
			t.Errorf("RangeFromScheduleArgument(%q) error = %v", argument, err)
			continue
		}
		if got != want {
			t.Errorf("RangeFromScheduleArgument(%q) = %q, want %q", argument, got, want)
		}
		if !got.Valid() {
			t.Errorf("%q resolved to an invalid range %q", argument, got)
		}
	}
}

func TestRangeFromScheduleArgumentRejectsAnythingElse(t *testing.T) {
	for _, argument := range []string{"", "daily", "DAY", "hour", "alltime"} {
		if _, err := RangeFromScheduleArgument(argument); !errors.Is(err, ErrUnknownRange) {
			t.Errorf("RangeFromScheduleArgument(%q) error = %v, want ErrUnknownRange", argument, err)
		}
	}
}

func TestWindowStartMatchesTheExecutorSwitch(t *testing.T) {
	taipei, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	// A Thursday, mid-month, mid-year, with a time of day that must be discarded.
	now := time.Date(2026, time.August, 6, 14, 37, 9, 0, taipei)

	tests := map[TimeRange]string{
		RangeYear:  "2026-01-01",
		RangeMonth: "2026-08-01",
		// Monday of that week.
		RangeWeek:  "2026-08-03",
		RangeToday: "2026-08-06",
	}
	for rangeValue, want := range tests {
		start := WindowStart(rangeValue, now)
		if start == nil {
			t.Errorf("WindowStart(%q) = nil, want %s", rangeValue, want)
			continue
		}
		if got := start.Format("2006-01-02"); got != want {
			t.Errorf("WindowStart(%q) = %s, want %s", rangeValue, got, want)
		}
		if start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 {
			t.Errorf("WindowStart(%q) kept a time of day: %s", rangeValue, start)
		}
		if start.Location() != taipei {
			t.Errorf("WindowStart(%q) location = %s, want Asia/Taipei", rangeValue, start.Location())
		}
	}

	// The all-time range has no window; its statistics rows are keyed by each post's
	// own creation date instead.
	if start := WindowStart(RangeAll, now); start != nil {
		t.Errorf("WindowStart(all) = %v, want nil", start)
	}
}

// The week must start on Monday, which is Carbon's ISO default and is what all
// 135,530 week rows in the production post_trends table show. Sunday is the case a
// naive implementation gets wrong: it is weekday 0 in Go but the *last* day of an ISO
// week, so it needs a six-day step back rather than none.
func TestWindowStartTreatsSundayAsTheEndOfTheWeek(t *testing.T) {
	utc := time.UTC
	// 2026-08-03 is a Monday, so 2026-08-09 is the Sunday that closes the same week.
	for _, day := range []int{3, 4, 5, 6, 7, 8, 9} {
		now := time.Date(2026, time.August, day, 12, 0, 0, 0, utc)
		start := WindowStart(RangeWeek, now)
		if start == nil {
			t.Fatalf("WindowStart(week) on %s = nil", now.Format("2006-01-02 Mon"))
		}
		if got := start.Format("2006-01-02"); got != "2026-08-03" {
			t.Errorf("WindowStart(week) on %s = %s, want 2026-08-03",
				now.Format("2006-01-02 Mon"), got)
		}
		if start.Weekday() != time.Monday {
			t.Errorf("week start on %s is a %s, want Monday",
				now.Format("2006-01-02 Mon"), start.Weekday())
		}
	}

	// The next Monday starts a new week.
	next := WindowStart(RangeWeek, time.Date(2026, time.August, 10, 0, 30, 0, 0, utc))
	if got := next.Format("2006-01-02"); got != "2026-08-10" {
		t.Errorf("WindowStart(week) on the following Monday = %s, want 2026-08-10", got)
	}
}

// A month or year boundary must not roll into the neighbouring period.
func TestWindowStartHandlesPeriodBoundaries(t *testing.T) {
	utc := time.UTC
	cases := []struct {
		now       time.Time
		rangeName TimeRange
		want      string
	}{
		{time.Date(2026, time.January, 1, 0, 0, 0, 0, utc), RangeYear, "2026-01-01"},
		{time.Date(2026, time.December, 31, 23, 59, 59, 0, utc), RangeYear, "2026-01-01"},
		{time.Date(2026, time.March, 1, 0, 0, 0, 0, utc), RangeMonth, "2026-03-01"},
		{time.Date(2026, time.March, 31, 23, 59, 59, 0, utc), RangeMonth, "2026-03-01"},
		// A leap day, to catch date arithmetic that normalises badly.
		{time.Date(2028, time.February, 29, 6, 0, 0, 0, utc), RangeMonth, "2028-02-01"},
		{time.Date(2028, time.February, 29, 6, 0, 0, 0, utc), RangeWeek, "2028-02-28"},
	}
	for _, test := range cases {
		start := WindowStart(test.rangeName, test.now)
		if start == nil {
			t.Fatalf("WindowStart(%q) on %s = nil", test.rangeName, test.now.Format("2006-01-02"))
		}
		if got := start.Format("2006-01-02"); got != test.want {
			t.Errorf("WindowStart(%q) on %s = %s, want %s",
				test.rangeName, test.now.Format("2006-01-02"), got, test.want)
		}
	}
}

func TestTimeRangeValidity(t *testing.T) {
	for _, valid := range []TimeRange{RangeAll, RangeYear, RangeMonth, RangeWeek, RangeToday} {
		if !valid.Valid() {
			t.Errorf("%q should be valid", valid)
		}
	}
	for _, invalid := range []TimeRange{"", "day", "hour", "ALL"} {
		if invalid.Valid() {
			t.Errorf("%q should not be valid", invalid)
		}
	}
}
