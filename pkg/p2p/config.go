package p2p

import (
	"io"
	"log"
	"os"
	"time"

	"github.com/hashicorp/memberlist"
)

// Config is the configuration for creating a P2P-Node instance.
type Config struct {
	// NodeName of this node. This must be unique in the cluster.
	// If it is not set, set it to the host name of the running machine.
	NodeName string

	// BroadcastTimeout is the amount of time to wait for a broadcast message to be sent
	// to the cluster. If this is not set, a timeout of 5 seconds will be set.
	BroadcastTimeout time.Duration

	// MemberlistConfig is the memberlist configuration that P2P will
	// use to do the underlying membership management and gossip.
	MemberlistConfig *memberlist.Config

	// MessageBuffer is used to control how many messages are buffered.This is
	// used to prevent messages that have already been received from being redelivered.
	// The buffer must be large enough to handle all recent messages.
	MessageBuffer int

	// HandleFunc is a hook function used by the client to process messages and
	// must not be blocked.
	HandleFunc func([]byte) error

	// LogOutput is the location to write logs to. If this is not set,
	// logs will go to stderr.
	LogOutput io.Writer

	// Logger is a custom logger which you provide. If Logger is set, it will use
	// this for the internal logger. If Logger is not set, it will fall back to the
	// behavior for using LogOutput. You cannot specify both LogOutput and Logger
	// at the same time.
	Logger *log.Logger
}

// DefaultConfig provides a default p2p node configuration 
func DefaultConfig() Config {
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	config := Config{
		NodeName:         hostname,
		BroadcastTimeout: 5,
		MessageBuffer:    10000,
		Logger:           log.New(log.Writer(), "[p2p]", 1),

		MemberlistConfig: memberlist.DefaultLocalConfig(),
	}

	config.HandleFunc = func(buf []byte) error {
		if config.Logger != nil {
			config.Logger.Printf("handle func :%s", string(buf))
		}
		return nil
	}

	return config
}
