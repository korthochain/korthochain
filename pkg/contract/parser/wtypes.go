package parser

import "io"

const (
	EOF = -1
	RET = -2
	ERR = -3
)

const (
	ROLLSIZE = 2
	BUFFSIZE = 1024
)

const (
	IDENTIFIER = iota

	INT_CONSTANT
	STRING_CONSTANT
)

type scriptLex struct {
	err        error
	fd         io.Reader
	curr       *scriptWord // Current word
	buffer     *scriptLexBuffer
	rollBuffer *scriptLexRollBuffer
}

type scriptLexRollBuffer struct {
	cnt    int
	buffer []int
}

type scriptLexBuffer struct {
	pos    int
	lim    int
	buffer []byte
}

type scriptWord struct {
	typ   int
	name  string
	value interface{}
}

var charTypeList = []int{
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
	ERR, ERR, ERR, ERR, ERR, ERR, ERR, ERR,
}
