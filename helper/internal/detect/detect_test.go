package detect

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/tonylook/managents/helper/internal/agent"
)

type fakeSource struct {
	name     string
	sessions []agent.Session
	err      error
}

func (f fakeSource) Name() string { return f.name }

func (f fakeSource) Sessions(context.Context, time.Time) ([]agent.Session, error) {
	return f.sessions, f.err
}

func TestAllMergesAndAttributesErrors(t *testing.T) {
	errLocked := errors.New("database is locked")
	healthy := fakeSource{name: "healthy", sessions: []agent.Session{{ID: "healthy:1"}, {ID: "healthy:2"}}}
	broken := fakeSource{name: "broken", sessions: []agent.Session{{ID: "broken:3"}}, err: errLocked}
	tests := []struct {
		name       string
		sources    []Source
		wantIDs    []string
		wantSource string // the source blamed for the error; empty for no error
	}{
		{"all healthy", []Source{healthy, fakeSource{name: "empty"}}, []string{"healthy:1", "healthy:2"}, ""},
		{"one failing", []Source{broken, healthy}, []string{"broken:3", "healthy:1", "healthy:2"}, "broken"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions, err := All(context.Background(), time.Now(), tt.sources)

			var ids []string
			for _, s := range sessions {
				ids = append(ids, s.ID)
			}
			if !reflect.DeepEqual(ids, tt.wantIDs) {
				t.Errorf("sessions = %v, want %v: a failing source must not hide the others", ids, tt.wantIDs)
			}

			var sourceErr *SourceError
			switch {
			case tt.wantSource == "" && err != nil:
				t.Errorf("err = %v, want none", err)
			case tt.wantSource == "":
			case !errors.As(err, &sourceErr) || sourceErr.Source != tt.wantSource || !errors.Is(err, errLocked):
				t.Errorf("err = %v, want the %s source's error", err, tt.wantSource)
			case err.Error() != "broken: database is locked":
				t.Errorf("message = %q, want it prefixed with the source name", err.Error())
			}
		})
	}
}
