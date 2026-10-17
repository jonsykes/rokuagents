package agent

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// Card is a session ready for display: ordered and with a unique, short name.
type Card struct {
	Session
	Name string
}

// Arrange orders sessions by recent activity and gives each one a display
// name. It does not modify its input.
//
// Working sessions come first (they are active right now), in start order so
// they don't swap places with each other. Then every other session, most
// recent status change first: whatever has been waiting or idle the longest
// sinks to the end, which on the display means the later pages.
//
// Names are given in start order, so "api #1" is always the older of two
// sessions in the same folder, whatever they are doing. Ties are broken by
// ID, so the result never depends on the order sessions were detected in.
func Arrange(sessions []Session) []Card {
	cards := make([]Card, len(sessions))
	for i, s := range sessions {
		cards[i] = Card{Session: s}
	}

	slices.SortFunc(cards, func(a, b Card) int { return byStart(a.Session, b.Session) })
	dirs := make([]string, len(cards))
	for i, c := range cards {
		dirs[i] = c.Dir
	}
	for i, name := range DisplayNames(dirs) {
		cards[i].Name = name
	}

	slices.SortFunc(cards, func(a, b Card) int { return byActivity(a.Session, b.Session) })
	return cards
}

// byStart orders sessions oldest first.
func byStart(a, b Session) int {
	if c := a.StartedAt.Compare(b.StartedAt); c != 0 {
		return c
	}
	return strings.Compare(a.ID, b.ID)
}

// byActivity orders working sessions first, then the most recent change.
func byActivity(a, b Session) int {
	aWorking, bWorking := a.Status == StatusWorking, b.Status == StatusWorking
	switch {
	case aWorking && !bWorking:
		return -1
	case bWorking && !aWorking:
		return 1
	case aWorking || a.Since.Equal(b.Since):
		return byStart(a, b)
	default:
		return b.Since.Compare(a.Since)
	}
}

// DisplayNames turns working directories into short card names:
//   - the folder's base name ("/src/agent-lights" -> "agent-lights");
//   - "parent/name" when different folders share a base name;
//   - "name #1", "name #2" for several sessions in the same folder.
func DisplayNames(dirs []string) []string {
	bases := make([]string, len(dirs))
	for i, dir := range dirs {
		bases[i] = baseName(dir)
	}

	names := make([]string, len(dirs))
	for i, dir := range dirs {
		names[i] = bases[i]
		if count(bases, bases[i]) > count(dirs, dir) {
			if parent := baseName(filepath.Dir(trimSlash(dir))); parent != "" && parent != "." {
				names[i] = parent + "/" + bases[i]
			}
		}
	}

	seen := make(map[string]int, len(names))
	result := make([]string, len(names))
	for i, name := range names {
		if count(names, name) == 1 {
			result[i] = name
			continue
		}
		seen[name]++
		result[i] = fmt.Sprintf("%s #%d", name, seen[name])
	}
	return result
}

func baseName(dir string) string {
	trimmed := trimSlash(dir)
	if trimmed == "" {
		return dir
	}
	return filepath.Base(trimmed)
}

func trimSlash(dir string) string {
	return strings.TrimRight(dir, `/\`)
}

func count(values []string, value string) int {
	n := 0
	for _, v := range values {
		if v == value {
			n++
		}
	}
	return n
}
