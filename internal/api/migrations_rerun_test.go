package api_test

import (
	"testing"

	"homebase/internal/apitest"
	"homebase/internal/store/storetest"
)

// Migrations run again on every start. An Appointment's two Due Soon
// Reminders (24 hours and an hour before) share a stage, which the
// scheduled Reminders' key from before Reminder Times would reject.
func TestMigrationsRunAgainWithSeveralRemindersOfOneStagePending(t *testing.T) {
	t.Parallel()
	st := storetest.Open(t)
	h := apitest.NewWithStore(t, st)
	household := h.SeedHousehold()
	createAppointmentAt(t, household.Owner, at(17, 9, 0), household.Member.User.ID)

	if err := storetest.Migrate(t, st); err != nil {
		t.Fatalf("Migrate() again error = %v", err)
	}
}
