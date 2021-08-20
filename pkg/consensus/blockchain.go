package consensus

import (
	/* 	"korthochain/pkg/block" */
	"time"

	"github.com/korthochain/korthochain/pkg/block"
)

type Hash []byte

// Blockchain synchronization state management, including adding and
// removing blocks. The longest blockchain selection, as well as update

type BlockChain struct {
	BlockHeader *block.Block
	//blockchain  db
	//accounts db

	Oranphs      map[*Hash]OrphanBlock
	PrevOrphans  map[*Hash][]OrphanBlock
	oldestOrphan *OrphanBlock
}

//orphan Block data structure
type OrphanBlock struct {
	Block      *block.Block
	Expiration time.Time
}

func (hash *Hash) IsEqual(target *Hash) bool {
	if hash == nil && target == nil {
		return true
	}
	if hash == nil || target == nil {
		return false
	}
	return *hash == *target
}
