package audio

import (
	"fmt"
	"sync"

	"github.com/hraban/opus"
)

// EncoderPool manages Opus encoders for multiple guilds
type EncoderPool struct {
	encoders map[string]*opus.Encoder
	mu       sync.RWMutex
}

// NewEncoderPool creates a new encoder pool
func NewEncoderPool() *EncoderPool {
	return &EncoderPool{
		encoders: make(map[string]*opus.Encoder),
	}
}

// GetOrCreate gets or creates an Opus encoder for a guild
func (p *EncoderPool) GetOrCreate(guildID string) (*opus.Encoder, error) {
	p.mu.RLock()
	encoder, exists := p.encoders[guildID]
	p.mu.RUnlock()

	if exists && encoder != nil {
		return encoder, nil
	}

	// Create new encoder
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check after acquiring write lock
	if encoder, exists := p.encoders[guildID]; exists && encoder != nil {
		return encoder, nil
	}

	encoder, err := opus.NewEncoder(SampleRate, Channels, opus.AppAudio)
	if err != nil {
		return nil, fmt.Errorf("failed to create opus encoder: %w", err)
	}

	p.encoders[guildID] = encoder
	return encoder, nil
}

// Remove removes an encoder for a guild
func (p *EncoderPool) Remove(guildID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.encoders, guildID)
}

// Clear removes all encoders
func (p *EncoderPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.encoders = make(map[string]*opus.Encoder)
}
