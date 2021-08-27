package txpool

import "go.uber.org/zap"

type Config struct {
	BlockChain IBlockchain

	Logger *zap.Logger
}
