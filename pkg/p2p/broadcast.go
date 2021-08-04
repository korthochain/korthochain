package p2p

import (
	"github.com/hashicorp/memberlist"
)

// broadcast is an implementation of memberlist.
type broadcast struct {
	msg    []byte
	notify chan<- struct{}
}

func (b *broadcast) Invalidates(other memberlist.Broadcast) bool {
	return false
}

// implements memberlist.UniqueBroadcast
func (b *broadcast) UniqueBroadcast() {}

func (b *broadcast) Message() []byte {
	return b.msg
}

func (b *broadcast) Finished() {
	if b.notify != nil {
		close(b.notify)
	}
}
