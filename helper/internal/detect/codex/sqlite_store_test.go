package codex

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

// newDatabase creates a state database with the columns the store reads, in
// a Codex home of its own. withSubagents adds the table of Codex versions
// that spawn subagents.
func newDatabase(t *testing.T, home, name string, withSubagents bool) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(home, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	exec(t, db, `
CREATE TABLE threads (id TEXT PRIMARY KEY, rollout_path TEXT NOT NULL, created_at INTEGER NOT NULL,
                      updated_at INTEGER NOT NULL, source TEXT NOT NULL, cwd TEXT NOT NULL, title TEXT NOT NULL,
                      archived INTEGER NOT NULL DEFAULT 0, first_user_message TEXT NOT NULL DEFAULT '')`)
	if withSubagents {
		exec(t, db, `CREATE TABLE thread_spawn_edges (parent_thread_id TEXT NOT NULL,
                      child_thread_id TEXT NOT NULL PRIMARY KEY, status TEXT NOT NULL)`)
	}
	return db
}

func exec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func addThread(t *testing.T, db *sql.DB, id, cwd, source string, created, updated int64, archived bool) {
	t.Helper()
	exec(t, db, `INSERT INTO threads (id, rollout_path, created_at, updated_at, source, cwd, title, archived)
                 VALUES (?, ?, ?, ?, ?, ?, 'a title', ?)`,
		id, "/Users/ann/.codex/sessions/rollout-"+id+".jsonl", created, updated, source, cwd, archived)
}

func lastThread(t *testing.T, home, dir string) (string, bool) {
	t.Helper()
	thread, found, err := SQLiteStore{Home: home}.LastThread(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Base(thread.RolloutPath), found
}

func TestSQLiteStoreFindsTheFolderThread(t *testing.T) {
	home := t.TempDir()
	db := newDatabase(t, home, "state_5.sqlite", true)
	addThread(t, db, "old", "/w/api", "cli", 100, 200, false)
	addThread(t, db, "tui", "/w/api", "vscode", 300, 400, false)
	addThread(t, db, "same-second", "/w/api", "cli", 250, 400, false)
	addThread(t, db, "archived", "/w/api", "cli", 500, 900, true)
	addThread(t, db, "exec", "/w/api", "exec", 600, 900, false)
	addThread(t, db, "mcp", "/w/api", "mcp", 600, 900, false)
	addThread(t, db, "subagent", "/w/api", `{"subagent":"review"}`, 700, 900, false)
	addThread(t, db, "web", "/w/web", "cli", 800, 900, false)
	exec(t, db, `INSERT INTO thread_spawn_edges VALUES ('tui', 'subagent', 'open')`)

	got, found := lastThread(t, home, "/w/api")

	if !found || got != "rollout-tui.jsonl" {
		t.Errorf("thread = %q, %v; want the newest interactive one of the folder", got, found)
	}
	if _, found := lastThread(t, home, "/w/docs"); found {
		t.Error("found a thread for a folder Codex never ran in")
	}
}

func TestSQLiteStoreWithoutSubagents(t *testing.T) {
	home := t.TempDir()
	db := newDatabase(t, home, "state_5.sqlite", false)
	addThread(t, db, "tui", "/w/api", "cli", 300, 400, false)

	if got, found := lastThread(t, home, "/w/api"); !found || got != "rollout-tui.jsonl" {
		t.Errorf("thread = %q, %v; want it found without thread_spawn_edges", got, found)
	}
}

func TestSQLiteStoreReadsTheNewestSchema(t *testing.T) {
	home := t.TempDir()
	addThread(t, newDatabase(t, home, "state_4.sqlite", true), "stale", "/w/api", "cli", 900, 900, false)
	addThread(t, newDatabase(t, home, "state_12.sqlite", true), "current", "/w/api", "cli", 100, 100, false)
	newDatabase(t, home, "state_old.sqlite", true)

	if got, _ := lastThread(t, home, "/w/api"); got != "rollout-current.jsonl" {
		t.Errorf("thread = %q, want the one in state_12.sqlite", got)
	}
}

func TestSQLiteStoreWithoutDatabase(t *testing.T) {
	if _, found := lastThread(t, t.TempDir(), "/w/api"); found {
		t.Error("found a thread without a database")
	}
}

func TestSQLiteStoreReportsABrokenDatabase(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "state_5.sqlite"), []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, _, err := (SQLiteStore{Home: home}).LastThread(context.Background(), "/w/api"); err == nil {
		t.Error("no error for a file that is not a database")
	}
}

func TestDefaultHome(t *testing.T) {
	t.Setenv("CODEX_HOME", "")
	if got, want := DefaultHome("/Users/ann"), filepath.Join("/Users/ann", ".codex"); got != want {
		t.Errorf("DefaultHome = %q, want %q", got, want)
	}
	t.Setenv("CODEX_HOME", "/srv/codex")
	if got := DefaultHome("/Users/ann"); got != "/srv/codex" {
		t.Errorf("DefaultHome with CODEX_HOME = %q", got)
	}
}
