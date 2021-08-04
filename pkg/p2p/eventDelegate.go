package p2p

import (
	"github.com/hashicorp/memberlist"
)

type eventDelegate struct {
	node *Node
}

func (ed *eventDelegate) NotifyJoin(node *memberlist.Node) {
	ed.node.logger.Printf("A node has joined: %s", node.String())
}

func (ed *eventDelegate) NotifyLeave(node *memberlist.Node) {
	ed.node.logger.Printf("A node has left: %s", node.String())
}

func (ed *eventDelegate) NotifyUpdate(node *memberlist.Node) {
	ed.node.logger.Printf("A node was updated: %s", node.String())
}
