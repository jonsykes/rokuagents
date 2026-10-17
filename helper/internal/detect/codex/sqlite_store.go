package codex

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite" // pure-Go driver: no cgo, cross-compiles everywhere
)

// SQLiteStore finds threads in Codex's state database, state_<N>.sqlite in
// the Codex home. Only the id-like columns are read: never a title, a preview
// or the first message.
type SQLiteStore struct {
	Home string
}

// DefaultHome is where Codex keeps its data for the user whose home directory
// is home: $CODEX_HOME when it is set, otherwise ~/.codex.
func DefaultHome(home string) string {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".codex")
}

// The newest thread of the folder that is not archived, not a subagent's and
// not run by `codex exec` or the MCP server (which have no card). Codex
// creates a thread when the TUI starts, so a new session wins over the last
// one in the folder at once; resuming an old thread brings it back on top with
// its first write. Codex versions without subagents have no
// thread_spawn_edges table: the second query is for them.
const lastThreadQuery = `
SELECT t.rollout_path FROM threads t
WHERE t.cwd = ? AND t.archived = 0 AND t.source NOT IN ('exec', 'mcp')
  AND NOT EXISTS (SELECT 1 FROM thread_spawn_edges e WHERE e.child_thread_id = t.id)
ORDER BY t.updated_at DESC, t.created_at DESC
LIMIT 1`

const lastThreadQueryNoSubagents = `
SELECT t.rollout_path FROM threads t
WHERE t.cwd = ? AND t.archived = 0 AND t.source NOT IN ('exec', 'mcp')
ORDER BY t.updated_at DESC, t.created_at DESC
LIMIT 1`

// LastThread implements Store.
func (s SQLiteStore) LastThread(ctx context.Context, dir string) (Thread, bool, error) {
	path := s.databasePath()
	if path == "" {
		return Thread{}, false, nil // Codex never ran here: no history, not an error
	}
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return Thread{}, false, err
	}
	defer db.Close()

	var thread Thread
	for _, query := range []string{lastThreadQuery, lastThreadQueryNoSubagents} {
		err := db.QueryRowContext(ctx, query, dir).Scan(&thread.RolloutPath)
		switch {
		case err == nil:
			return thread, true, nil
		case strings.Contains(err.Error(), "no such table: thread_spawn_edges"):
			continue
		case errors.Is(err, sql.ErrNoRows):
			return Thread{}, false, nil
		default:
			return Thread{}, false, err
		}
	}
	return Thread{}, false, nil
}

// databasePath is the state database of the newest schema: Codex renames the
// file when its format changes (state_5.sqlite), and may leave the old one.
// It is looked up on every query, so an upgrade of Codex is picked up at once.
func (s SQLiteStore) databasePath() string {
	matches, _ := filepath.Glob(filepath.Join(s.Home, "state_*.sqlite"))
	best, bestVersion := "", -1
	for _, path := range matches {
		name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(path), "state_"), ".sqlite")
		if version, err := strconv.Atoi(name); err == nil && version > bestVersion {
			best, bestVersion = path, version
		}
	}
	return best
}

// dsn opens the database read-only. A lock held by Codex is waited for at
// most 250 ms, so a busy database cannot hold up the 1 s update loop for long.
func dsn(path string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path // Windows: file:///C:/Users/...
	}
	u := url.URL{Scheme: "file", Path: path}
	q := url.Values{}
	q.Set("mode", "ro")
	q.Add("_pragma", "busy_timeout(250)")
	u.RawQuery = q.Encode()
	return u.String()
}
