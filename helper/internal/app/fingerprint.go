package app

import (
	"encoding/json"

	"github.com/tonylook/managents/helper/internal/protocol"
)

// Fingerprint identifies what a frame shows apart from the values the display
// advances by itself (clock and ages), so that a mere tick is not a change.
func Fingerprint(state protocol.State) string {
	state.Now = 0
	agents := make([]protocol.AgentEntry, len(state.Agents))
	for i, a := range state.Agents {
		a.Age = 0
		agents[i] = a
	}
	state.Agents = agents
	data, _ := json.Marshal(state) // a State always marshals
	return string(data)
}
