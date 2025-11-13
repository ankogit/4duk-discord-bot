package audio

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

// Streamer handles audio streaming to Discord
type Streamer struct {
	radioURL    string
	encoderPool *EncoderPool
	logger      *logrus.Logger
}

// NewStreamer creates a new audio streamer
func NewStreamer(radioURL string, encoderPool *EncoderPool, logger *logrus.Logger) *Streamer {
	return &Streamer{
		radioURL:    radioURL,
		encoderPool: encoderPool,
		logger:      logger,
	}
}

// Stream streams audio from URL to Discord voice connection
func (s *Streamer) Stream(ctx context.Context, vc *discordgo.VoiceConnection, guildID string, isActive func() bool) error {
	s.logger.Infof("[%s] Starting radio stream: %s", guildID, s.radioURL)

	// Wait a bit for voice connection to stabilize
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return ctx.Err()
	}

	// Double-check connection is still ready
	if vc == nil || vc.Status != discordgo.VoiceConnectionStatusReady {
		return fmt.Errorf("voice connection not ready after wait")
	}

	// Tell Discord we're speaking
	err := vc.Speaking(true)
	if err != nil {
		return fmt.Errorf("failed to set speaking: %w", err)
	}

	// Ensure we stop speaking when done
	defer func() {
		if vc != nil && vc.Status == discordgo.VoiceConnectionStatusReady {
			vc.Speaking(false)
		}
	}()

	// FFmpeg command to stream audio and convert to PCM
	// We'll encode PCM to Opus using the opus library
	ffmpegArgs := []string{
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-reconnect_at_eof", "1",
		"-i", s.radioURL,
		"-f", "s16le",
		"-ar", "48000",
		"-ac", "2",
		"-",
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
	cmd.Stderr = os.Stderr // Log ffmpeg errors

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		cmd.Wait()
	}()

	// Buffer for reading PCM data
	buffer := make([]int16, FrameSize*Channels)
	pcmBytes := make([]byte, PCMFrameSize)

	// Send audio in a loop
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if still active
		if !isActive() {
			return nil
		}

		// Check if voice connection is still valid
		if vc == nil || vc.Status != discordgo.VoiceConnectionStatusReady {
			return fmt.Errorf("voice connection not ready")
		}

		// Read PCM data (s16le format from ffmpeg)
		_, err := io.ReadFull(stdout, pcmBytes)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return fmt.Errorf("stream ended: %w", err)
			}
			return fmt.Errorf("error reading audio data: %w", err)
		}

		// Convert bytes to int16 samples (little-endian)
		for i := 0; i < len(buffer); i++ {
			buffer[i] = int16(binary.LittleEndian.Uint16(pcmBytes[i*2:]))
		}

		// Encode to Opus and send
		if err := s.sendFrame(vc, guildID, buffer); err != nil {
			return fmt.Errorf("error sending audio frame: %w", err)
		}
	}
}

// sendFrame sends a PCM frame to Discord voice connection
func (s *Streamer) sendFrame(vc *discordgo.VoiceConnection, guildID string, pcm []int16) error {
	if vc == nil || vc.Status != discordgo.VoiceConnectionStatusReady {
		return fmt.Errorf("voice connection not ready")
	}

	// Get encoder
	encoder, err := s.encoderPool.GetOrCreate(guildID)
	if err != nil {
		return fmt.Errorf("failed to get encoder: %w", err)
	}

	// Encode PCM to Opus
	opusFrame := make([]byte, 4000) // Opus frame buffer (max size ~4000 bytes)
	n, err := encoder.Encode(pcm, opusFrame)
	if err != nil {
		return fmt.Errorf("failed to encode opus: %w", err)
	}

	// Send Opus frame
	select {
	case vc.OpusSend <- opusFrame[:n]:
		return nil
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf("timeout sending opus frame")
	}
}
