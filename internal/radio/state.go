package radio

import (
	"sync"
)

// State represents the state of radio for a guild
type State struct {
	Active             bool
	ChannelID          string
	AutoChannelID      string // Channel ID for auto-join when users are present
	AutoConnectEnabled bool   // Whether auto-connect is enabled
	ReconnectAttempts  int
	mu                 sync.Mutex
}

// NewState creates a new radio state
func NewState() *State {
	return &State{
		Active:            false,
		ChannelID:         "",
		ReconnectAttempts: 0,
	}
}

// SetActive sets the active state
func (s *State) SetActive(active bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Active = active
}

// IsActive returns whether radio is active
func (s *State) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Active
}

// SetChannelID sets the channel ID
func (s *State) SetChannelID(channelID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ChannelID = channelID
}

// GetChannelID returns the channel ID
func (s *State) GetChannelID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ChannelID
}

// IncrementReconnectAttempts increments reconnect attempts
func (s *State) IncrementReconnectAttempts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ReconnectAttempts++
	return s.ReconnectAttempts
}

// ResetReconnectAttempts resets reconnect attempts
func (s *State) ResetReconnectAttempts() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ReconnectAttempts = 0
}

// GetReconnectAttempts returns reconnect attempts
func (s *State) GetReconnectAttempts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ReconnectAttempts
}

// Reset resets the state
func (s *State) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Active = false
	s.ChannelID = ""
	s.ReconnectAttempts = 0
	// Note: AutoChannelID is NOT reset, so it persists
}

// SetAutoChannelID sets the auto-join channel ID
func (s *State) SetAutoChannelID(channelID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AutoChannelID = channelID
}

// GetAutoChannelID returns the auto-join channel ID
func (s *State) GetAutoChannelID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.AutoChannelID
}

// SetAutoConnectEnabled sets whether auto-connect is enabled
func (s *State) SetAutoConnectEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AutoConnectEnabled = enabled
}

// IsAutoConnectEnabled returns whether auto-connect is enabled
func (s *State) IsAutoConnectEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.AutoConnectEnabled
}
