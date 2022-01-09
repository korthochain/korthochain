package difficulty

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextMinerDifficulty(t *testing.T) {
	assert := assert.New(t)

	//Difficulty:526149281  globalDifficulty:523878437
	g := CompactToBig(523878437)
	t.Logf("g :%s", g.String())
	num, err := NextMinerDifficulty(g, 1200000000000000, 1200000000000000)
	assert.NoError(err)
	t.Logf("g :%s", g.String())

	gc := CompactToBig(523878437)
	t.Logf("gc:%s", gc.String())

	t.Logf("n :%s", num.String())

	c := BigToCompact(num)
	t.Logf("c :%d", c)

	d := CompactToBig(526149281)
	d.Mul(d, big.NewInt(100)).Div(d, gc)
	t.Logf("d :%s", d.String())

	num.Div(num, gc)
	t.Logf("n :%s", num.String())

	assert.Equal(int64(1000), num.Int64())
}

func TestNextMinerDifficultyForRat(t *testing.T) {
	assert := assert.New(t)

	input := big.NewInt(10000)
	{
		output, err := NextMinerDifficulty(input, 1, 10)
		assert.NoError(err)
		assert.Equal(int64(19000), output.Int64())
	}

	{
		output, err := NextMinerDifficulty(input, 0, 10)
		assert.NoError(err)
		assert.Equal(int64(10000), output.Int64())
	}

	{
		output, err := NextMinerDifficulty(input, 10, 10)
		assert.NoError(err)
		assert.Equal(int64(100000), output.Int64())
	}

	{
		output, err := NextMinerDifficulty(input, 1, 10000)
		assert.NoError(err)
		assert.Equal(int64(10009), output.Int64())
	}

	{
		output, err := NextMinerDifficulty(input, 1000, 10)
		assert.NoError(err)
		assert.Equal(int64(100000), output.Int64())
	}

}
