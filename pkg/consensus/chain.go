package consensus

import (
	"bytes"
	"encoding/hex"
	"time"

	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/blockchain"
	"github.com/korthochain/korthochain/pkg/logger"
	"go.uber.org/zap"
)

//judge block by hash exist
func (b *BlockChain) BlockExists(hash []byte) bool {

	var Exists bool
	_, err := b.Bc.GetBlockByHash(hash)
	if err != nil {
		return false
	}
	Exists = true
	return Exists
}

//ProcessOrphan   if block prevhash is exist in orphan pool ,
//move all matched orphan blcoks to mainchain,
func (b *BlockChain) ProcessOrphan(block *block.Block) bool {

	var ok, main bool
	prevhashs := make([]Hash, MaxEqealBlockWeight)
	prevhashs[0] = BytesToHash(block.Hash)

	for len(prevhashs) > 0 {
		prevhash := prevhashs[0]
		prevhashs = prevhashs[1:]

		for i := 0; i < len(b.PrevOrphans[prevhash]); i++ {
			orphan := b.PrevOrphans[prevhash][i]
			if orphan == nil {
				continue
			}
			orphanhash := BytesToHash(orphan.Block.Hash)
			b.removeOrphanBlock(orphan)
			i--
			ok, main = b.maybeAcceptBlock(orphan.Block)
			if !ok {
				return main
			}
			prevhashs = append(prevhashs, orphanhash)
		}
	}
	return main

}

//delete orphan block data
func (b *BlockChain) removeOrphanBlock(orphan *OrphanBlock) {

	b.orphanLock.Lock()
	defer b.orphanLock.Unlock()

	orphanHash := BytesToHash(orphan.Block.Hash)

	delete(b.Oranphs, orphanHash)

	prevhash := BytesToHash(orphan.Block.PrevHash)

	orphans := b.PrevOrphans[prevhash]
	for i := 0; i < len(orphans); i++ {
		hash := BytesToHash(orphans[i].Block.Hash)
		if bytes.Equal(orphanHash[:], hash[:]) {
			copy(orphans[i:], orphans[i+1:])
			orphans[len(orphans)-1] = nil
			orphans = orphans[:len(orphans)-1]
			i--
		}

	}
	b.PrevOrphans[prevhash] = orphans
	if len(b.PrevOrphans[prevhash]) == 0 {
		delete(b.PrevOrphans, prevhash)
	}

}

//add block  to orphan pool,According to the expiration time to
//determine whether to retain
func (b *BlockChain) AddOrphanBlock(block *block.Block) {

	logger.Info("AddOrphanBlock", zap.String("hash", hex.EncodeToString(block.Hash)), zap.Uint64("height", block.Height))
	hash := BytesToHash(block.Hash)
	prevhash := BytesToHash(block.PrevHash)

	oBlock := &OrphanBlock{
		Expiration: time.Now().Add(time.Hour),
		Block:      block,
	}

	for _, orphan := range b.Oranphs {
		if time.Now().After(orphan.Expiration) {
			b.removeOrphanBlock(orphan)
			continue
		}

		if b.oldestOrphan == nil || orphan.Expiration.Before(b.oldestOrphan.Expiration) {
			b.oldestOrphan = orphan
		}

	}

	b.orphanLock.Lock()
	defer b.orphanLock.Unlock()

	//
	_, exist := b.Oranphs[hash]
	if exist {
		return
	}
	if oBlock != nil {
		b.Oranphs[hash] = oBlock
		b.PrevOrphans[prevhash] = append(b.PrevOrphans[prevhash], oBlock)
	}
	logger.Info("AddOrphanBlock end", zap.String("hash", hex.EncodeToString(block.Hash)), zap.Uint64("height", block.Height))

}

func (bc *BlockChain) maybeAcceptBlock(b *block.Block) (bool, bool) {

	tipblock, err := bc.Bc.Tip()
	if err != nil {
		logger.Error("GetMaxBlockHeight err")
		return false, false
	}

	if bytes.Equal(b.PrevHash, tipblock.Hash) {
		if b.Height%10 != 2 && b.Height > blockchain.InitHeight+1 {
			if tipblock.GlobalDifficulty.Cmp(b.GlobalDifficulty) < 0 {
				logger.SugarLogger.Error("block Difficulty is easy", "myself : ", tipblock.GlobalDifficulty, "block : ", b.GlobalDifficulty)
				return false, false
			}
		}

		if b.Height != 1 {
			if err := bc.Bc.CheckBlockRegular(b); err != nil {
				logger.Error("CheckBlockRegular", zap.Error(err))
				return false, false
			}
		}

		err := bc.Bc.AddBlock(b)
		if err != nil {
			logger.SugarLogger.Errorf("-------------maybeAcceptBlock,hash:%s,height:%d,err=%v\n", hex.EncodeToString(b.Hash), b.Height, zap.Error(err))
			return false, false
		}
		return true, true
	}

	parent, err := bc.Bc.GetBlockByHash(b.PrevHash)
	if err != nil {
		logger.Error("AddUncleBlock", zap.String("PrevHash", hex.EncodeToString(b.PrevHash)), zap.Error(err))
		return false, false
	}

	//Determine whether the input block is on the longest chain
	if parent.Height == tipblock.Height {
		err := bc.Bc.AddUncleBlock(b)
		if err != nil {
			logger.Error("AddUncleBlock", zap.Uint64("height", b.Height), zap.Error(err))
			return false, false
		}

		hashList := make([][]byte, 0)
		hashList = append(hashList, b.Hash)
		branchhash, delHeight, err := bc.FindBranchPoint(parent.Hash, tipblock.Hash)
		if err != nil {
			logger.Error("FindBranchPoint", zap.Uint64("height", b.Height), zap.Error(err))
			return false, false
		}

		if len(branchhash) > 0 {
			hashList = append(hashList, branchhash...)
		}
		logger.WarnLogger.Printf(" ReorganizeChain!!\n\n")
		err = bc.Bc.ReorganizeChain(hashList, delHeight)
		if err != nil {
			logger.Error("ReorganizeChain", zap.Uint64("height", b.Height), zap.Error(err))
			bc.Bc.DeleteUncleBlock(b)
			return false, false
		}

	} else if parent.Height < tipblock.Height {
		err := bc.Bc.AddUncleBlock(b)
		if err != nil {
			return false, false
		}
		return true, false

	} else {
		logger.Error("blockchain db have heigher blockchain")
		return false, false
	}
	return true, true

}

//Finds a block where two chains intersect

func (bc *BlockChain) FindBranchPoint(prevhash, tiphash []byte) ([][]byte, uint64, error) {

	hashs := make([][]byte, 0)
	mainhash := tiphash
	branchchainhash := prevhash
	var height uint64
	for {

		if bytes.Equal(mainhash, branchchainhash) {
			break
		}
		hashs = append(hashs, branchchainhash)
		mblock, err := bc.Bc.GetBlockByHash(mainhash)
		if err != nil {
			return hashs, 0, err
		}
		mainhash = mblock.PrevHash
		bblock, err := bc.Bc.GetBlockByHash(branchchainhash)
		if err != nil {
			return hashs, 0, err
		}
		branchchainhash = bblock.PrevHash
		height = bblock.Height
	}
	return hashs, height, nil
}

func CheckNonce(nonceMp map[address.Address]uint64, bc blockchain.Blockchains, from address.Address, txNonce uint64) bool {
	var nonce uint64
	if n, ok := nonceMp[from]; ok {
		nonce = n
	} else {
		n, err := bc.GetNonce(from)
		if err != nil {
			return false
		}
		nonce = n
	}
	nonceMp[from] = nonce + 1

	return nonce == txNonce
}
