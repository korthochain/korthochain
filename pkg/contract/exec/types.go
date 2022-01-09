package exec

import (
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/korthochain/korthochain/pkg/contract/parser"
	"github.com/korthochain/korthochain/pkg/storage/store"
)

type Exec interface {
	Root() []byte
	Flush() error
}

type exec struct {
	db  store.Transaction
	sdb *state.StateDB
	mp  map[string]string
}

type scriptDealFunc (func(store.Transaction, *state.StateDB, string, map[string]string, *parser.Script) error)
