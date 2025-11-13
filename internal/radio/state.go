package radio

import (
	"sync"
)

// State represents the state of radio for a guild
type State struct {
	Active            bool
	ChannelID         string
	ReconnectAttempts int
	mu                sync.Mutex
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
}

