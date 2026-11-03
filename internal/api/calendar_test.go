package api_test

import (
	"net/http"
	"testing"
	"time"

	"homebase/internal/apitest"
	"homebase/internal/store"
)

func TestCalendarDefaultsToTheClocksMonthAndDay(t *testing.T) {
	t.Parallel()
	h := apitest.New(t)
	household := h.SeedHousehold()
	h.Clock.Set(time.Date(2031, time.May, 10, 12, 0, 0, 0, time.Local))

	calendar := apitest.Decode[store.CalendarMonth](t, household.Member.Get("/api/v1/calendar").Expect(http.StatusOK))

	if got := calendar.Month.In(time.Local).Format("2006-01"); got != "2031-05" {
		t.Errorf("month = %s, want 2031-05", got)
	}
	var today []string
	for _, day := range calendar.Days {
		if day.IsToday {
			today = append(today, day.Date.In(time.Local).Format("2006-01-02"))
		}
	}
	if len(today) != 1 || today[0] != "2031-05-10" {
		t.Errorf("today = %v, want [2031-05-10]", today)
	}
}
