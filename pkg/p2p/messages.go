package p2p

import (
	"bytes"
	"crypto/sha256"

	"github.com/fxamacker/cbor/v2"
)

type messageType = uint8

const (
	PayloadMessageType messageType = iota
	pullPushMessageType
)

type message struct {
	LTime   LamportTime
	Type    messageType
	Payload []byte
}

type pullPushMessage struct {
	LTime LamportTime
}

type receivedMessage struct {
	LTime LamportTime
	IDs   [][]byte
}

func (m *message) id() []byte {
	hash := sha256.Sum256(m.Payload)
	return hash[:]
}

func encodeMessage(msgType messageType, msg interface{}) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte(byte(msgType))

	encoder := cbor.NewEncoder(buf)
	err := encoder.Encode(msg)
	return buf.Bytes(), err
}

func decodeMessage(buf []byte, out interface{}) error {
	return cbor.NewDecoder(bytes.NewReader(buf)).Decode(out)
}
