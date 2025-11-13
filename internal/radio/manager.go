package radio

import (
	"encoding/json"
	"os"
	"sync"
)

// Manager manages radio states for multiple guilds
type Manager struct {
	states     map[string]*State
	configFile string
	mu         sync.RWMutex
}

// GuildConfig represents saved configuration for a guild
type GuildConfig struct {
	AutoChannelID      string `json:"auto_channel_id"`
	AutoConnectEnabled bool   `json:"auto_connect_enabled"`
}

// NewManager creates a new radio state manager
func NewManager() *Manager {
	// Use data directory for persistence
	configDir := "data"
	configFile := configDir + "/radio_config.json"
	
	// Create data directory if it doesn't exist
	_ = os.MkdirAll(configDir, 0755)
	
	m := &Manager{
		states:     make(map[string]*State),
		configFile: configFile,
	}
	m.LoadConfig()
	return m
}

// LoadConfig loads saved configuration from file
func (m *Manager) LoadConfig() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.configFile)
	if err != nil {
		// File doesn't exist yet, that's okay
		return
	}

	var configs map[string]GuildConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return
	}

	// Load saved configs into states
	for guildID, config := range configs {
		state := m.getOrCreateUnsafe(guildID)
		state.SetAutoChannelID(config.AutoChannelID)
		state.SetAutoConnectEnabled(config.AutoConnectEnabled)
	}
}

// SaveConfig saves current configuration to file
func (m *Manager) SaveConfig() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	configs := make(map[string]GuildConfig)
	for guildID, state := range m.states {
		configs[guildID] = GuildConfig{
			AutoChannelID:      state.GetAutoChannelID(),
			AutoConnectEnabled: state.IsAutoConnectEnabled(),
		}
	}

	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		// Log error but don't fail - use standard log for now
		return
	}

	// Ensure directory exists before writing
	configDir := "data"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return
	}

	// Write file atomically using temp file
	tmpFile := m.configFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return
	}
	
	// Atomic rename
	if err := os.Rename(tmpFile, m.configFile); err != nil {
		_ = os.Remove(tmpFile) // Clean up temp file on error
	}
}

// getOrCreateUnsafe gets or creates a state without locking (internal use)
func (m *Manager) getOrCreateUnsafe(guildID string) *State {
	state, exists := m.states[guildID]
	if !exists {
		state = NewState()
		m.states[guildID] = state
	}
	return state
}

// GetOrCreate gets or creates a state for a guild
func (m *Manager) GetOrCreate(guildID string) *State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getOrCreateUnsafe(guildID)
}

// SaveState saves state configuration to file
func (m *Manager) SaveState(guildID string) {
	m.SaveConfig()
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

