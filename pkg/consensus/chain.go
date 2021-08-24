package consensus

import (
	"time"

	"github.com/korthochain/korthochain/pkg/block"
)

//judge block by hash exist
func BlockExists(hash string) bool {

	var Exists bool
	//根据hash查询块，是否存在

	return Exists
}

//ProcessOrphan   if block prevhash is exist in orphan pool ,
//move all matched orphan blcoks to mainchain,
func (b *BlockChain) ProcessOrphan(block block.Block) {

	hash := BytesToHash(block.Hash)

	orphans := b.PrevOrphans[hash]
	for _, orphan := range orphans {

		b.removeOrphanBlock(orphan)

		maybeAcceptBlock(*orphan.Block)

		ohash := BytesToHash(orphan.Block.Hash)
		orphans = b.PrevOrphans[ohash]
		/* continue */
	}

}

//delete orphan block data
func (b *BlockChain) removeOrphanBlock(orphan *OrphanBlock) {
	orphanHash := BytesToHash(orphan.Block.Hash)

	delete(b.Oranphs, orphanHash)

	prevhash := BytesToHash(orphan.Block.PrevHash)

	orphans := b.PrevOrphans[prevhash]
	for i := 0; i < len(orphans); i++ {
		hash := BytesToHash(orphans[i].Block.Hash)
		if hash.IsEqual(&orphanHash) {

		}
	}
	orphans = b.PrevOrphans[prevhash]
	if len(b.PrevOrphans[prevhash]) == 0 {
		delete(b.PrevOrphans, prevhash)
	}

}

//add block  to orphan pool,According to the expiration time to
//determine whether to retain
func (b *BlockChain) AddOrphanBlock(block block.Block) {

	hash := BytesToHash(block.Hash)
	prevhash := BytesToHash(block.PrevHash)

	//设置孤块过期时间
	oBlock := &OrphanBlock{
		Expiration: time.Now().Add(time.Hour),
		Block:      &block,
	}

	//孤块过期删除块
	for _, oBlock := range b.Oranphs {
		if time.Now().After(oBlock.Expiration) {
			b.removeOrphanBlock(oBlock)
			continue
		}

		if b.oldestOrphan == nil || oBlock.Expiration.Before(b.oldestOrphan.Expiration) {
			b.oldestOrphan = oBlock
		}

	}

	//
	b.Oranphs[hash] = oBlock
	b.PrevOrphans[prevhash] = append(b.PrevOrphans[prevhash], oBlock)
}

//判断当前区块的PrevBlock是否是bestChain的tip
func maybeAcceptBlock(b block.Block) bool {
	/*
		addBlock() */

	return false

}
