package ntp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdateTime(t *testing.T) {
	assert := assert.New(t)
	err := UpdateTimeFromNtp()
	assert.NoError(err)

}
