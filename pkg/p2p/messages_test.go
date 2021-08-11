package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessageCodec(t *testing.T) {
	assert := assert.New(t)
	msg := message{
		LTime:   LamportTime(10),
		Type:    1,
		Payload: []byte{215, 107, 11, 147, 201, 41, 120, 88, 133, 22, 237, 60, 113, 122, 93, 210, 7, 56, 133, 215, 192, 220, 83, 0, 54, 122, 173, 194, 70, 161, 154, 139},
	}
	data, err := encodeMessage(msg.Type, &msg)
	assert.NoError(err)

	maybeMsg := message{}
	err = decodeMessage(data[1:], &maybeMsg)
	assert.NoError(err)

	assert.Equal(msg, maybeMsg)
}
