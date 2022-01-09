package consensus

import (
	"sync"
	"time"

	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/blockchain"
)

type Hash [HashSize]byte

const HashSize int = 32

// Blockchain synchronization state management, including adding and
// removing blocks. The longest blockchain selection, as well as update

type BlockChain struct {
	BlockHeader *block.Block
	Bc          blockchain.Blockchains
	//blockchain  db
	//accounts db

	Oranphs      map[Hash]*OrphanBlock
	PrevOrphans  map[Hash][]*OrphanBlock
	oldestOrphan *OrphanBlock
	orphanLock   sync.RWMutex
}

func New(bc *blockchain.Blockchain) *BlockChain {
	return &BlockChain{
		Bc:          bc,
		Oranphs:     make(map[Hash]*OrphanBlock),
		PrevOrphans: make(map[Hash][]*OrphanBlock),
	}
}

//orphan Block data structure
type OrphanBlock struct {
	Block      *block.Block
	Expiration time.Time
}

func (b *BlockChain) OrphanBlockIsExist(hash []byte) (*block.Block, bool) {
	b.orphanLock.Lock()
	defer b.orphanLock.Unlock()
	h := BytesToHash(hash)
	oranph, ok := b.Oranphs[h]
	if ok {
		return oranph.Block, ok
	}

	return nil, false
}

func BytesToHash(in []byte) Hash {

	var tmp Hash
	if len(in) != HashSize {
		return tmp
	}
	for i, b := range in {
		tmp[i] = b
	}
	return tmp

}
