package audio

const (
	// SampleRate is the audio sample rate required by Discord (48kHz)
	SampleRate = 48000
	// Channels is the number of audio channels (stereo)
	Channels = 2
	// FrameSize is the frame size for 20ms at 48kHz (48000 * 0.02)
	FrameSize = 960
	// PCMFrameSize is the size of PCM frame in bytes (FrameSize * 2 bytes per sample * Channels)
	PCMFrameSize = FrameSize * 2 * Channels // 960 * 2 * 2 = 3840 bytes
)
