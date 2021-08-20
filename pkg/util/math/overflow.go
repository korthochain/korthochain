package math

import "fmt"

const (
	MINUINT64 = uint64(0)
	MAXUINT64 = ^MINUINT64
)

func AddUint64Overflow(a uint64, b ...uint64) (uint64, error) {
	for _, v := range b {
		if MAXUINT64-a < v {
			return 0, fmt.Errorf("uint64 add overflow")
		}
		a += v
	}

	return a, nil
}

func SubUint64Overflow(a uint64, b ...uint64) (uint64, error) {
	for _, v := range b {
		if a < v {
			return 0, fmt.Errorf("uint64 sub overflow")
		}
		a -= v
	}

	return a, nil
}
