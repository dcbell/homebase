CREATE TABLE IF NOT EXISTS project_folders (
    id BIGSERIAL PRIMARY KEY,
    household_id BIGINT NOT NULL REFERENCES households(id) ON DELETE CASCADE,
    project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS project_folder_id BIGINT REFERENCES project_folders(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_project_folders_project ON project_folders(project_id, sort_order, title);
CREATE INDEX IF NOT EXISTS idx_tasks_project_folder ON tasks(project_folder_id);
