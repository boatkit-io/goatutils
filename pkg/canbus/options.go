package canbus

import "github.com/brutella/can"

// ChannelOptions is a type that contains required options on a Channel.
type ChannelOptions struct {
	CanInterfaceName string
	MessageHandler   can.HandlerFunc
}

// ChannelOption is the type used to apply function options to Channel.
type ChannelOption func(*Channel)

// DefaultBitRate is the bitrate that Channel defaults to using.
const DefaultBitRate = 250000

// WithBitRate overrides the DefaultBitRate set on Channel.
func WithBitRate(br int32) ChannelOption {
	return func(c *Channel) {
		c.bitRate = br
	}
}
