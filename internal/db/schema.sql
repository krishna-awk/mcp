PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS projects (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT    NOT NULL,
  path       TEXT    NOT NULL UNIQUE,
  created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pages (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL,
  route      TEXT    NOT NULL,
  title      TEXT,
  file_path  TEXT,
  kind       TEXT,
  metadata   TEXT,
  created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_pages_project ON pages(project_id);

CREATE TABLE IF NOT EXISTS workflows (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL,
  name       TEXT    NOT NULL,
  description TEXT,
  steps      TEXT,
  status     TEXT,
  created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_workflows_project ON workflows(project_id);

CREATE TABLE IF NOT EXISTS tests (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id INTEGER NOT NULL,
  workflow_id INTEGER,
  name       TEXT    NOT NULL,
  file_path  TEXT    NOT NULL,
  framework  TEXT,
  code       TEXT,
  created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
  FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_tests_project ON tests(project_id);

CREATE TABLE IF NOT EXISTS runs (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  test_id     INTEGER NOT NULL,
  status      TEXT    NOT NULL,
  started_at  TEXT    NOT NULL,
  finished_at TEXT,
  duration_ms INTEGER,
  error       TEXT,
  log         TEXT,
  FOREIGN KEY (test_id) REFERENCES tests(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_runs_test ON runs(test_id);

CREATE TABLE IF NOT EXISTS artifacts (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  run_id     INTEGER,
  kind       TEXT    NOT NULL,
  path       TEXT    NOT NULL,
  label      TEXT,
  created_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_artifacts_run ON artifacts(run_id);

CREATE TABLE IF NOT EXISTS findings (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id     INTEGER NOT NULL,
  run_id         INTEGER,
  severity       TEXT,
  title          TEXT    NOT NULL,
  steps          TEXT,
  expected       TEXT,
  actual         TEXT,
  screenshot_path TEXT,
  trace_path     TEXT,
  created_at     TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
  FOREIGN KEY (run_id) REFERENCES runs(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_findings_project ON findings(project_id);
