package contractServer

import (
	"log"
	"net/http"
	"os"

	"github.com/korthochain/korthochain/pkg/blockchain"
	"github.com/korthochain/korthochain/pkg/config"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/server/contractServer/api"
	"github.com/korthochain/korthochain/pkg/txpool"
)

func RunMetamaskServer(bc blockchain.Blockchains, tp *txpool.Pool, cfg *config.CfgInfo) {
	s := api.NewMetamaskServer(bc, tp, cfg)
	http.HandleFunc("/", s.HandRequest)

	logger.InfoLogger.Println("Running contractServer...", cfg.MetamaskCfg.ListenPort)
	err := http.ListenAndServe(cfg.MetamaskCfg.ListenPort, nil)
	if err != nil {
		log.Println("start fasthttp fail:", err.Error())
		os.Exit(1)
	}
}
