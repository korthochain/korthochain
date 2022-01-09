//Sync  Blockchain
package consensus

import (
	"encoding/hex"
	"math/big"
	"time"

	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/blockchain"
	"github.com/korthochain/korthochain/pkg/logger"
	"go.uber.org/zap"
)

const (
	MaxEqealBlockWeight = 10
	MaxExpiration       = time.Hour
	MaxOrphanBlocks     = 200
)

// ProcessBlock is the main workhorse for handling insertion of new blocks into
// the block chain.  It includes functionality such as rejecting duplicate
// blocks, ensuring blocks follow all rules, orphan handling, and insertion into
// the block chain along with best chain selection and reorganization.
//
// When no errors occurred during processing, the first return value indicates
// whether or not the block is on the main chain and the second indicates
// whether or not the block is an orphan.
func (b *BlockChain) ProcessBlock(newblock *block.Block, globalDifficulty *big.Int, basePledge uint64) bool {

	//newblcok hash is exist
	defer logger.Info(" ProcessBlock  end ", zap.Uint64("height", newblock.Height), zap.String("hash", hex.EncodeToString(newblock.Hash)))

	if b.BlockExists(newblock.Hash) {
		logger.SugarLogger.Info("Block is exist", hex.EncodeToString(newblock.Hash))
		return false
	}

	hash := BytesToHash(newblock.Hash)
	if _, exist := b.Oranphs[hash]; exist {
		logger.Info("orphan is exist", zap.String("hash", hex.EncodeToString(newblock.Hash)))
		return false
	}

	maxHeight, err := b.Bc.GetMaxBlockHeight()
	if err != nil {
		logger.SugarLogger.Error("GetBlockByHeight err", err)
		return false
	}
	if maxHeight == blockchain.InitHeight {
		logger.SugarLogger.Error("==========first blcok============", hex.EncodeToString(newblock.Hash))
		err = b.Bc.AddBlock(newblock)
		if err != nil {
			logger.SugarLogger.Error("first blcok addblock err", err)
			return false
		}
		return true
	}

	if !b.BlockExists(newblock.PrevHash) {
		logger.Info("prevhash not exist")
		b.AddOrphanBlock(newblock)
		return false
	}

	//maybeAcceptBlock return longest chain flag
	succ, mainChain := b.maybeAcceptBlock(newblock)
	if !succ {
		return false
	}
	ok := b.ProcessOrphan(newblock)
	if ok {
		mainChain = ok
	}

	return mainChain
}
