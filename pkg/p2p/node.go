package p2p

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	"github.com/gofrs/uuid"
	"github.com/hashicorp/memberlist"
)

type Node struct {
	Config *Config

	memberlist *memberlist.Memberlist

	// members map[string]*memberState

	messageBuffer      []*receivedMessage
	messageBufferMutex sync.Mutex
	broadcasts         *memberlist.TransmitLimitedQueue

	messageClock LamportClock

	HandleFunc func([]byte) error
	stateLock  sync.Mutex
	state      nodeStatus

	logger *log.Logger
}

type nodeStatus int

const (
	nodeAlive nodeStatus = iota
	nodeLeave
	nodeShutdown
)

type nodeState struct {
	Name   string
	Addr   net.IP
	Port   uint16
	Tags   map[string]string
	Status nodeStatus
}

// Create creates a new P2P instance
func Create(conf Config) (*Node, error) {
	if len(conf.NodeName) == 0 {
		return nil, fmt.Errorf("NnodeName cannot be empty")
	}

	if len(conf.MemberlistConfig.AdvertiseAddr) == 0 {
		return nil, fmt.Errorf("invalid advertise address")
	}

	if conf.HandleFunc == nil {
		return nil, fmt.Errorf("HandleFunc cannot be empty")
	}
	node := &Node{
		Config: &conf,
	}

	logDest := conf.LogOutput
	if logDest == nil {
		logDest = os.Stderr
	}

	node.logger = conf.Logger
	if node.logger == nil {
		node.logger = log.New(logDest, "", log.LstdFlags)
	}

	node.messageBuffer = make([]*receivedMessage, conf.MessageBuffer)

	node.messageClock.Increment()
	conf.Logger.Printf("startup message ltime: %d", node.messageClock.Time())

	node.broadcasts = &memberlist.TransmitLimitedQueue{
		// TODO: NumNodes should return the current node number
		NumNodes:       func() int { return node.memberlist.NumMembers() },
		RetransmitMult: conf.MemberlistConfig.RetransmitMult,
	}

	conf.MemberlistConfig.Delegate = &delegate{node: node}
	conf.MemberlistConfig.Events = &eventDelegate{node: node}

	conf.MemberlistConfig.Logger = conf.Logger
	conf.MemberlistConfig.HandoffQueueDepth = 1024 * 10

	name, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}
	conf.MemberlistConfig.Name = name.String()

	memberlist, err := memberlist.Create(conf.MemberlistConfig)
	if err != nil {
		return nil, err
	}

	node.memberlist = memberlist

	return node, nil
}

func (n *Node) RegisterHandleFunc(f func([]byte) error) {
	n.HandleFunc = f
}

func (n *Node) handleMessage(msg *message) bool {
	msgc := *msg

	n.messageClock.Witness(msgc.LTime)
	// Check if this message is too old
	currTime := n.messageClock.Time()
	if currTime > LamportTime(len(n.messageBuffer)) && msgc.LTime < currTime-LamportTime(len(n.messageBuffer)) {
		return false
	}

	msgid := msgc.id()
	idx := msgc.LTime % LamportTime(len(n.messageBuffer))

	n.messageBufferMutex.Lock()
	seen := n.messageBuffer[idx]
	if seen != nil && seen.LTime == msgc.LTime {
		for _, id := range seen.IDs {
			if bytes.Equal(msgid, id) {
				n.messageBufferMutex.Unlock()
				return false
			}
		}
	} else {
		seen = &receivedMessage{LTime: msgc.LTime}
		n.messageBuffer[idx] = seen
	}
	seen.IDs = append(seen.IDs, msgid)
	n.messageBufferMutex.Unlock()

	if n.HandleFunc != nil {
		go n.HandleFunc(msgc.Payload)
	}

	return true
}

// SendMessage is a method that needs to be called by the client to send messages.
// messageType is used to distinguish message types
func (n *Node) SendMessage(msgType messageType, buf []byte) {
	msg := message{
		Type:    msgType,
		Payload: buf,
		LTime:   n.messageClock.Time(),
	}

	msgData, _ := encodeMessage(msgType, &msg)

	n.handleMessage(&msg)
	n.broadcasts.QueueBroadcast(&broadcast{msg: msgData})
}

// Join joins an existing P2P cluster. Returns the number of nodes successfully contacted.
// The returned error will be non-nil only in the case that no nodes could be contacted.
func (n *Node) Join(existing []string) (int, error) {
	if n.State() != nodeAlive {
		return 0, fmt.Errorf("node an't Join after Leave or Shutdown")
	}

	return n.memberlist.Join(existing)
}

// Leave leaves all nodes,and returns an error if it times out
func (n *Node) Leave() error {
	n.stateLock.Lock()
	if n.state == nodeLeave {
		n.stateLock.Unlock()
		return fmt.Errorf("Leave already in progress")
	} else if n.state == nodeShutdown {
		n.stateLock.Unlock()
		return fmt.Errorf("Leave called after Shutdown")
	} else {
		n.state = nodeLeave
	}
	n.stateLock.Unlock()

	if err := n.memberlist.Leave(n.Config.BroadcastTimeout); err != nil {
		return err
	}

	return nil
}

// State is the current state of this Node instance.
func (n *Node) State() nodeStatus {
	n.stateLock.Lock()
	defer n.stateLock.Unlock()
	return n.state
}

// Members returns the status information of all members
func (n *Node) Members() []nodeState {
	members := n.memberlist.Members()
	ret := make([]nodeState, 0)

	for _, v := range members {
		ns := nodeState{
			Name: v.Name,
			Addr: v.Addr,
			Port: v.Port,
			//TODO:Tags and Status
		}
		ret = append(ret, ns)
	}

	return ret
}
