package radio

import (
	"sync"
)

// Manager manages radio states for multiple guilds
type Manager struct {
	states map[string]*State
	mu     sync.RWMutex
}

// NewManager creates a new radio state manager
func NewManager() *Manager {
	return &Manager{
		states: make(map[string]*State),
	}
}

// GetOrCreate gets or creates a state for a guild
func (m *Manager) GetOrCreate(guildID string) *State {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[guildID]
	if !exists {
		state = NewState()
		m.states[guildID] = state
	}
	return state
}

// Get gets a state for a guild (read-only)
func (m *Manager) Get(guildID string) (*State, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, exists := m.states[guildID]
	return state, exists
}

// GetAllGuildIDs returns all guild IDs with states
func (m *Manager) GetAllGuildIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	guilds := make([]string, 0, len(m.states))
	for guildID := range m.states {
		guilds = append(guilds, guildID)
	}
	return guilds
}

// Remove removes a state for a guild
func (m *Manager) Remove(guildID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, guildID)
}

