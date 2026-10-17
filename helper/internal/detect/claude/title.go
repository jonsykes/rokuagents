package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
)

// MaxTitleBytes bounds a title; cards show two or three lines at most.
const MaxTitleBytes = 160

// titleMarker is in every line that can carry a title, so other lines are
// skipped without decoding them.
var titleMarker = []byte(`-title"`)

// titleEntry declares only the two title fields. Titles are what the Claude
// app shows in its sidebar: one the user set (custom-title) wins over the one
// Claude Code generated (ai-title).
type titleEntry struct {
	Type        string `json:"type"`
	CustomTitle string `json:"customTitle"`
	AITitle     string `json:"aiTitle"`
}

// titleReader follows transcripts as they grow. The first read of a
// transcript scans it whole, once; later reads only scan what was appended,
// so following a session costs nothing while it is quiet.
type titleReader struct {
	mu    sync.Mutex
	files map[string]*titleState
}

type titleState struct {
	offset     int64 // bytes scanned: always the end of a complete line
	custom, ai string
	seen       bool // read since the last sweep
}

// sweep forgets transcripts not read since the previous sweep, so closed
// sessions do not accumulate.
func (r *titleReader) sweep() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for path, state := range r.files {
		if !state.seen {
			delete(r.files, path)
			continue
		}
		state.seen = false
	}
}

func (r *titleReader) read(path string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.files == nil {
		r.files = map[string]*titleState{}
	}
	state := r.files[path]
	if state == nil {
		state = &titleState{}
		r.files[path] = state
	}
	state.seen = true
	if err := state.scan(path); err != nil {
		return "", err
	}
	title := state.custom
	if title == "" {
		title = state.ai
	}
	return truncateUTF8(title, MaxTitleBytes), nil
}

func (s *titleState) scan(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() < s.offset {
		*s = titleState{seen: true} // rewritten: start over
	}
	if info.Size() == s.offset {
		return nil
	}
	if _, err := f.Seek(s.offset, io.SeekStart); err != nil {
		return err
	}
	reader := bufio.NewReaderSize(f, 64<<10)
	for {
		line, err := reader.ReadBytes('\n')
		if errors.Is(err, io.EOF) {
			return nil // a partial last line is read again once it is complete
		}
		if err != nil {
			return err
		}
		s.offset += int64(len(line))
		if !bytes.Contains(line, titleMarker) {
			continue
		}
		var e titleEntry
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		switch e.Type {
		case "custom-title":
			s.custom = e.CustomTitle
		case "ai-title":
			s.ai = e.AITitle
		}
	}
}

// truncateUTF8 cuts s to at most n bytes without splitting a character.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}
