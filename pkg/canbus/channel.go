package canbus

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/brutella/can"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

type Channel struct {
	bitRate        int32
	ChannelOptions ChannelOptions

	bus        *can.Bus
	busHandler can.Handler

	log *logrus.Logger
}

func NewChannel(ctx context.Context, log *logrus.Logger, ChannelOptions ChannelOptions, opts ...ChannelOption) (*Channel, error) {
	c := Channel{
		ChannelOptions: ChannelOptions,
		log:            log,
	}

	// Apply defaults
	c.bitRate = DefaultBitRate

	// Apply functional options.
	for i := range opts {
		opts[i](&c)
	}

	if err := c.open(ctx); err != nil {
		return nil, errors.Wrap(err, "open connection to listen for messages")
	}

	return &c, nil
}

func (c *Channel) open(ctx context.Context) error {
	// Referencing https://github.com/angelodlfrtr/go-can/blob/master/transports/socketcan.go

	// Use netlink to make sure the interface is up
	link, err := netlink.LinkByName(c.ChannelOptions.CanInterfaceName)
	if err != nil {
		return fmt.Errorf("no link found for %v: %v", c.ChannelOptions.CanInterfaceName, err)
	}

	if link.Type() != "can" {
		return fmt.Errorf("invalid linktype %q", link.Type())
	}

	canLink := link.(*netlink.Can)

	if canLink.Attrs().OperState == netlink.OperUp {
		if canLink.BitRate != uint32(c.bitRate) {
			c.log.WithField("bitRate", canLink.BitRate).Info("Channel currentl has wrong bitrate, bringing down")

			cmd := exec.CommandContext(ctx, "ip", "link", "set", c.ChannelOptions.CanInterfaceName, "down")
			if output, err := cmd.Output(); err != nil {
				logBase := c.log.WithField("cmd", strings.Join(cmd.Args, " ")).WithField("output", string(output))
				if errCast, worked := err.(*exec.ExitError); !worked {
					logBase = logBase.WithField("stderr", string(errCast.Stderr))
				}
				logBase.Error("Ip link set down failed")
				return err
			}

			// Re-fetch info
			link, err = netlink.LinkByName(c.ChannelOptions.CanInterfaceName)
			if err != nil {
				return fmt.Errorf("no link found for %v: %v", c.ChannelOptions.CanInterfaceName, err)
			}

			canLink = link.(*netlink.Can)
		}
	}

	if canLink.Attrs().OperState == netlink.OperDown {
		c.log.WithField("canName", c.ChannelOptions.CanInterfaceName).WithField("bitRate", c.bitRate).Info("Link is down, bringing up link")

		// ip link set can1 up type can bitrate 250000
		cmd := exec.CommandContext(ctx, "ip", "link", "set", c.ChannelOptions.CanInterfaceName, "up", "type", "can", "bitrate", strconv.Itoa(int(c.bitRate)))
		if output, err := cmd.Output(); err != nil {
			logBase := c.log.WithField("cmd", strings.Join(cmd.Args, " ")).WithField("output", string(output))
			if errCast, worked := err.(*exec.ExitError); !worked {
				logBase = logBase.WithField("stderr", string(errCast.Stderr))
			}
			logBase.Error("Ip link set up failed")
			return err
		}

		// TODO(ddr): Someday figure out how the hell to make the netlink stuff work

		// fmt.Printf("CanAttrs: %+v\n", *canLink)
		// canLink.BitRate = 250000

		// if err := netlink.LinkModify(canLink); err != nil {
		// 	return err
		// }

		// fmt.Println("Modified link")

		// if err := netlink.LinkSetUp(canLink); err != nil {
		// 	return err
		// }
	}

	// Open the brutella can bus
	bus, err := can.NewBusForInterfaceWithName(c.ChannelOptions.CanInterfaceName)
	if err != nil {
		return err
	}

	// Spawn goroutine to listen for messages
	go bus.ConnectAndPublish() //nolint:errcheck // Why: it's a goroutine

	c.bus = bus
	c.busHandler = can.NewHandler(c.ChannelOptions.MessageHandler)
	c.bus.Subscribe(c.busHandler)

	c.log.WithField("canName", c.ChannelOptions.CanInterfaceName).
		Info("opened connection and listening")

	return nil
}

func (c *Channel) Close(ctx context.Context) error {
	if c.bus == nil {
		return nil
	}

	c.bus.Unsubscribe(c.busHandler)
	if err := c.bus.Disconnect(); err != nil {
		return errors.Wrap(err, "close underlying bus connection")
	}

	return nil
}

func (c *Channel) WriteFrame(frame can.Frame) error {
	return c.bus.Publish(frame)
}
