package consensus

import (
	"korthochain/pkg/block"
	"time"
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

	hash := block.Hash

	orphans := b.PrevOrphans[(*Hash)(&hash)]
	for _, orphan := range orphans {

		b.removeOrphanBlock(orphan)

		maybeAcceptBlock(*orphan.Block)

		ohash := orphan.Block.Hash
		orphans = b.PrevOrphans[(*Hash)(&ohash)]
		continue
	}

}

//delete orphan block data
func (b *BlockChain) removeOrphanBlock(orphan OrphanBlock) {

	delete(b.Oranphs, (*Hash)(&orphan.Block.Hash))

	prevhash := orphan.Block.PrevHash

	orphans := b.PrevOrphans[prevhash]
	for i := 0; i < len(orphans); i++ {
		hash := Hash(orphans[i].Block.Hash)
		if hash.IsEqual(&Hash{orphan.Block.Hash}) {

		}
	}
	orphans := b.PrevOrphans[prevhash]
	if len(b.PrevOrphans[prevhash]) == 0 {
		delete(b.PrevOrphans, b.PrevOrphans[prevhash])
	}

}

//add block  to orphan pool,According to the expiration time to
//determine whether to retain
func (b *BlockChain) AddOrphanBlock(block block.Block) {

	//孤块过期删除块
	for _, oBlock := range b.Oranphs {
		if time.Now().After(oBlock.Expiration) {
			b.removeOrphanBlock(oBlock)
			continue
		}

		if b.oldestOrphan == nil || oBlock.Expiration.Before(b.oldestOrphan.Expiration) {
			b.oldestOrphan = block
		}

	}
	//设置孤块过期时间
	oBlock := OrphanBlock{
		Expiration: time.Now().Add(time.Hour),
		Block:      &block,
	}
	//
	b.Oranphs[(*Hash)(&block.Hash)] = oBlock
	b.PrevOrphans[(*Hash)(&block.Hash)] = append(b.PrevOrphans[(*Hash)(&block.Hash)], oBlock)
}

//
func maybeAcceptBlock(b block.Block) bool {
	/*
		addBlock() */

	return false

}
