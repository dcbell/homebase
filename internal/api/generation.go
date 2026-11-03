package api

import (
	"context"
	"fmt"

	"homebase/internal/notify"
)

// GenerateDueTasks is the background check: it creates the Task for every
// Routine and Maintenance Schedule due as of the Server's Clock and reports
// each Generated Task to the Notifier. No Member performs this, so Generated
// Tasks always notify their assignee.
func (s *Server) GenerateDueTasks(ctx context.Context) error {
	now := s.clock.Now()
	routineTasks, err := s.store.GenerateDueRoutineTasks(ctx, now)
	if err != nil {
		return fmt.Errorf("generate due routine tasks: %w", err)
	}
	for _, task := range routineTasks {
		s.taskChanged(ctx, notify.TaskChange{After: task})
	}
	if len(routineTasks) > 0 {
		s.logger.Info("generated due routine tasks", "count", len(routineTasks))
	}

	maintenanceTasks, err := s.store.GenerateDueAssetMaintenanceTasks(ctx, now)
	if err != nil {
		return fmt.Errorf("generate due asset maintenance tasks: %w", err)
	}
	for _, task := range maintenanceTasks {
		s.taskChanged(ctx, notify.TaskChange{After: task})
	}
	if len(maintenanceTasks) > 0 {
		s.logger.Info("generated due asset maintenance tasks", "count", len(maintenanceTasks))
	}
	return nil
}
