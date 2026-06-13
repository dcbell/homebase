CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_open_routine_due
ON tasks(routine_id, due_at)
WHERE routine_id IS NOT NULL AND due_at IS NOT NULL AND status = 'open';
