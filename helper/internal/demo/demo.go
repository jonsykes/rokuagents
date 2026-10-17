// Package demo plays canned agent sessions through the normal pipeline, to
// try a display without any agent running and to check every layout and
// status on real hardware. All names are made up.
package demo

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tonylook/managents/helper/internal/agent"
)

// Scene is one demo screen.
type Scene struct {
	Title    string
	Sessions []agent.Session
}

// Source is a detect.Source that shows each scene for Hold, which must be
// positive, in a loop. It serves one Runner: Sessions must not be called
// concurrently.
type Source struct {
	Start time.Time
	Hold  time.Duration
	// Logger, if set, is told the title of each scene as it comes up.
	Logger *slog.Logger

	shown int // the number of the scene last returned, plus one
}

// Name implements detect.Source.
func (*Source) Name() string { return "demo" }

// Sessions implements detect.Source. Ages count from the moment the scene
// came up, so they advance while it is shown.
func (s *Source) Sessions(_ context.Context, now time.Time) ([]agent.Session, error) {
	scene, n := s.SceneAt(now)
	if s.Logger != nil && n+1 != s.shown {
		s.Logger.Info("showing", "scene", scene.Title)
	}
	s.shown = n + 1
	return scene.Sessions, nil
}

// SceneAt returns the scene shown at now and its number, counting the
// scenes shown since Start.
func (s *Source) SceneAt(now time.Time) (Scene, int) {
	n := max(int(now.Sub(s.Start)/s.Hold), 0)
	scenes := Scenes(s.Start.Add(time.Duration(n) * s.Hold))
	return scenes[n%len(scenes)], n
}

// Scenes returns the demo sequence for scenes that come up at shownAt. They
// cover every status, every agent, grids from one card to a full page of nine,
// a second page, and the naming of folders that share a name.
func Scenes(shownAt time.Time) []Scene {
	// session is open since id minutes, so ids also give the start order.
	session := func(id int, kind agent.Kind, dir string, status agent.Status, age time.Duration) agent.Session {
		return agent.Session{
			ID: fmt.Sprintf("%s:%d", kind, id), Kind: kind, Dir: "/demo/" + dir, Status: status,
			Since: shownAt.Add(-age), StartedAt: shownAt.Add(-time.Duration(id) * time.Minute),
		}
	}
	withContext := func(s agent.Session, used, limit int) agent.Session {
		s.Context = &agent.ContextUsage{Used: used, Limit: limit}
		return s
	}
	const claude, opencode, codex = agent.KindClaude, agent.KindOpenCode, agent.KindCodex
	const working, waiting, idle, failed = agent.StatusWorking, agent.StatusWaiting, agent.StatusIdle, agent.StatusError

	mixed := []agent.Session{
		withContext(session(1, claude, "atlas-api", working, 12*time.Second), 88000, 200000),
		withContext(session(2, opencode, "web-shop", waiting, 5*time.Minute), 31000, 0), // limit unknown
		session(3, claude, "data-pipeline", failed, 45*time.Second),
		withContext(session(4, claude, "docs-site", idle, 150*time.Minute), 240000, 1000000),
	}

	sameNames := []agent.Session{
		withContext(session(10, claude, "web-shop", working, 40*time.Second), 120000, 200000),
		session(11, opencode, "web-shop", waiting, 2*time.Minute), // "web-shop #1" and "#2"
		session(12, claude, "client/api", working, 8*time.Second),
		session(13, codex, "server/api", waiting, 30*time.Second), // "client/api" and "server/api"
		session(14, opencode, "café-orders", idle, 3*time.Hour),
		withContext(session(15, claude, "billing", failed, 20*time.Second), 64000, 200000),
		session(16, claude, "sandbox", idle, 26*time.Hour),
	}

	kinds := []agent.Kind{claude, claude, opencode, codex}
	statuses := []agent.Status{working, waiting, working, failed, idle, waiting}
	var longNames []agent.Session
	for i, dir := range []string{
		"payments-reconciliation-service", "customer-onboarding-portal", "infrastructure-terraform-modules",
		"mobile-app-release-train", "search-relevance-experiments", "observability-dashboards",
		"internal-developer-platform", "marketing-website-redesign", "data-warehouse-migrations",
	} {
		s := session(20+i, kinds[i%len(kinds)], dir, statuses[i%len(statuses)], time.Duration(i*83)*time.Second)
		longNames = append(longNames, withContext(s, 20000*(i+1), 200000))
	}

	var fourteen []agent.Session
	for i, dir := range []string{"api", "web", "core", "infra", "docs", "mobile", "billing", "search",
		"auth", "etl", "ml-models", "design-system", "sandbox", "notes"} {
		fourteen = append(fourteen, session(100+i, kinds[(i+2)%len(kinds)], dir, statuses[i%len(statuses)],
			time.Duration(i*97)*time.Second))
	}

	return []Scene{
		{Title: "one agent working", Sessions: mixed[:1]},
		{Title: "two agents", Sessions: mixed[:2]},
		{Title: "four statuses", Sessions: mixed},
		{Title: "context almost full", Sessions: []agent.Session{
			withContext(session(5, claude, "refactor-auth", working, 3*time.Minute), 150000, 200000),
			withContext(session(6, codex, "db-migration", waiting, 20*time.Second), 241000, 258400),
		}},
		{Title: "seven agents: folders that share a name", Sessions: sameNames},
		{Title: "nine agents: long folder names", Sessions: longNames},
		{Title: "fourteen agents: tap the screen for page two", Sessions: fourteen},
		{Title: "no agents", Sessions: nil},
	}
}
