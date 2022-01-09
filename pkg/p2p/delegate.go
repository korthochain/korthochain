package p2p

import (
	"github.com/hashicorp/memberlist"
)

type delegate struct {
	node *Node
}

var _ memberlist.Delegate = &delegate{}

func (d *delegate) NodeMeta(limit int) []byte {
	return []byte{}
}

func (d *delegate) NotifyMsg(buf []byte) {
	if len(buf) == 0 {
		return
	}

	msgType := messageType(buf[0])
	var rebroadcast = false
	switch msgType {
	case PayloadMessageType:
		msg := &message{}
		decodeMessage(buf[1:], msg)
		rebroadcast = d.node.handleMessage(msg)
	default:
		d.node.logger.Printf("unkown message type:%d", msgType)
		return
	}

	if rebroadcast {
		d.node.broadcasts.QueueBroadcast(&broadcast{msg: buf})
	}
}

func (d *delegate) GetBroadcasts(overhead, limit int) [][]byte {
	return d.node.broadcasts.GetBroadcasts(overhead, limit)
}

func (d *delegate) LocalState(join bool) []byte {
	ppmsg := pullPushMessage{
		LTime: d.node.messageClock.Time(),
	}

	data, err := encodeMessage(pullPushMessageType, &ppmsg)
	if err != nil {
		return []byte{}
	}

	return data
}

func (d *delegate) MergeRemoteState(buf []byte, join bool) {
	var ppmsg pullPushMessage

	if err := decodeMessage(buf[1:], &ppmsg); err != nil {
		return
	}

	if ppmsg.LTime > 0 {
		d.node.messageClock.Witness(ppmsg.LTime - 1)
	}

}
