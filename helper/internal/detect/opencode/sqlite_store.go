package opencode

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver: no cgo, cross-compiles everywhere
)

// SQLiteStore reads OpenCode's opencode.db. All JSON fields are extracted by
// SQLite itself, so message content never reaches this process.
type SQLiteStore struct {
	Path string
}

// DefaultDatabasePath is where OpenCode keeps its database for the user whose
// home directory is home. OpenCode follows the XDG layout on every platform:
// $XDG_DATA_HOME when it is set, otherwise ~/.local/share.
func DefaultDatabasePath(home string) string {
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(data, "opencode", "opencode.db")
}

// OpenCode 2 keeps sessions in session_v2 and their messages in
// session_message, one row per turn step. Its old message and part tables stay
// in the database but are no longer written, so they are only the fallback
// for OpenCode 1 (the *V1 queries).
//
// A message of the new kind is the newest user, assistant, compaction or idle
// row of the folder. "idle" is written when a turn ends, with its outcome. An
// assistant step that finished with "tool-calls" is not the end of the turn.
const lastMessageQuery = `
SELECT CASE m.type WHEN 'user' THEN 'user' ELSE 'assistant' END,
       CASE WHEN m.type = 'idle' THEN CASE json_extract(m.data, '$.outcome') WHEN 'failed' THEN 'failed' ELSE '' END
            ELSE coalesce(json_extract(m.data, '$.error.type'), '') END,
       m.type = 'idle'
         OR (m.type = 'assistant' AND json_extract(m.data, '$.time.completed') IS NOT NULL
             AND coalesce(json_extract(m.data, '$.finish'), '') <> 'tool-calls')
         OR (m.type = 'compaction' AND json_extract(m.data, '$.status') IN ('completed', 'failed')),
       m.time_updated,
       CASE WHEN m.type = 'user' THEN m.time_created
            ELSE coalesce((SELECT max(u.time_created) FROM session_message u
                           WHERE u.session_id = m.session_id AND u.type = 'user' AND u.seq < m.seq), m.time_created) END
FROM session_message m JOIN session_v2 s ON s.id = m.session_id
WHERE s.directory = ? AND m.type IN ('user', 'assistant', 'compaction', 'idle')
ORDER BY m.time_created DESC, m.seq DESC
LIMIT 1`

// The tool calls of an assistant step are in its content. Only the newest
// step is opened, so a long history is never parsed.
const lastToolPartQuery = `
SELECT json_extract(j.value, '$.state.status'), m.time_updated
FROM session_message m, json_each(m.data, '$.content') j
WHERE m.id = (SELECT a.id FROM session_message a JOIN session_v2 s ON s.id = a.session_id
              WHERE s.directory = ? AND a.type = 'assistant'
              ORDER BY a.time_created DESC, a.seq DESC LIMIT 1)
  AND json_extract(j.value, '$.type') = 'tool'
ORDER BY j.key DESC
LIMIT 1`

const lastMessageQueryV1 = `
SELECT json_extract(m.data, '$.role'),
       coalesce(json_extract(m.data, '$.error.name'), ''),
       json_extract(m.data, '$.time.completed') IS NOT NULL,
       m.time_updated,
       coalesce((SELECT u.time_created FROM message u WHERE u.id = json_extract(m.data, '$.parentID')),
                m.time_created)
FROM message m JOIN session s ON s.id = m.session_id
WHERE s.directory = ?
ORDER BY m.time_created DESC
LIMIT 1`

const lastToolPartQueryV1 = `
SELECT json_extract(p.data, '$.state.status'), p.time_updated
FROM part p
WHERE p.session_id = (SELECT id FROM session WHERE directory = ? ORDER BY time_updated DESC LIMIT 1)
  AND json_extract(p.data, '$.type') = 'tool'
ORDER BY p.time_created DESC
LIMIT 1`

// LastMessage implements Store.
func (s SQLiteStore) LastMessage(ctx context.Context, dir string) (Message, bool, error) {
	var msg Message
	var role sql.NullString
	var updated, turnStarted int64
	found, err := s.queryRow(ctx, dir, []string{lastMessageQuery, lastMessageQueryV1},
		&role, &msg.ErrorName, &msg.Completed, &updated, &turnStarted)
	if !found || err != nil {
		return Message{}, false, err
	}
	msg.Role = role.String
	msg.UpdatedAt = time.UnixMilli(updated)
	msg.TurnStartedAt = time.UnixMilli(turnStarted)
	return msg, true, nil
}

// LastToolPart implements Store.
func (s SQLiteStore) LastToolPart(ctx context.Context, dir string) (ToolPart, bool, error) {
	var part ToolPart
	var state sql.NullString
	var updated int64
	found, err := s.queryRow(ctx, dir, []string{lastToolPartQuery, lastToolPartQueryV1}, &state, &updated)
	if !found || err != nil {
		return ToolPart{}, false, err
	}
	part.State = state.String
	part.UpdatedAt = time.UnixMilli(updated)
	return part, true, nil
}

// queryRow runs the first query that can run and scans its row. A database
// without the tables of a query (OpenCode 1 has no session_v2) moves on to the
// next one; one that has them is never asked again, so the stale rows of the
// old tables do not answer for a folder OpenCode 2 has no history of.
func (s SQLiteStore) queryRow(ctx context.Context, dir string, queries []string, dest ...any) (bool, error) {
	if _, err := os.Stat(s.Path); errors.Is(err, os.ErrNotExist) {
		return false, nil // OpenCode never ran here: no history, not an error
	}
	db, err := sql.Open("sqlite", s.dsn())
	if err != nil {
		return false, err
	}
	defer db.Close()

	for _, query := range queries {
		err := db.QueryRowContext(ctx, query, dir).Scan(dest...)
		switch {
		case err == nil:
			return true, nil
		case strings.Contains(err.Error(), "no such table"):
			continue
		case errors.Is(err, sql.ErrNoRows):
			return false, nil
		default:
			return false, err
		}
	}
	return false, nil
}

// dsn opens the database read-only. A lock held by OpenCode is waited for at
// most 250 ms, so a busy database cannot hold up the 1 s update loop for long.
func (s SQLiteStore) dsn() string {
	path := filepath.ToSlash(s.Path)
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
