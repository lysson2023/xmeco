-- XMECO Migration 009: Project-user assignment for access control
CREATE TABLE IF NOT EXISTS project_user (
    project_id INT NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    user_id    INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_project_user_user ON project_user(user_id);
