//Sync  Blockchain
package consensus

import (
	"encoding/hex"
	"time"

	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/blockchain"
)

//hash存在则丢弃块，
//prevhash 不存在，加入孤块列表
//prevhash 存在，则将块加入chain，判断加入当前块是否时最长链
//块加入后，hash在孤块列表,则延长当前链，筛选孤块列表中过期的块

//QUESTION
// 1.如何判断最长链的
// 2.孤块中超过了当前的最大块高，如何保证跟其他节点保持一致，
// 3.使用orphans中的块延长链时，如何确认最长链

const (
	maxEqealBlockHeight = 6
	maxExpiration       = time.Hour
	maxOrphanBlocks     = 100
)

func (b *BlockChain) ProcessBlock(newblock block.Block) bool {

	//newblcok hash is exist

	if blockchain.BlockExists(hex.EncodeToString(newblock.Hash)) {
		return false
	}

	//检查块数据健全性
	/* 	checkBlockRegular() */

	//判断prevhash是否存在，
	if !blockchain.BlockExists(hex.EncodeToString(newblock.PrevHash)) {
		//
		b.AddOrphanBlock()
		return false
	}

	//maybeAcceptBlock return longest chain flag
	mainChain := maybeAcceptBlock(newblock)

	b.ProcessOrphan(newblock)
	return mainChain
}
