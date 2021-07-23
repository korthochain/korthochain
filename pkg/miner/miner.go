package miner

import (
	"fmt"
	"sync"

	//"github.com/kto/block"
)

// Config is a descriptor containing the cpu miner configuration.
type Config struct {
	// MiningAddrs is a list of payment addresses to use for the generated
	// blocks.  Each generated block will randomly choose one of them.
	MiningAddrs []byte
}

type Miner struct {
	submitBlockLock sync.Mutex
	wg              sync.WaitGroup
	coinbaseAddr    []byte        	//矿工地址
	c               chan struct{} 	//难度通道  接收block
	p               chan struct{}	//p2p通道  接收block
	r               chan struct{}	//rpc通道  接收block
	startch         chan bool
	download        chan bool
	started         bool
}

//挖矿，计算难度
func (m *Miner) CalcDifficulty() {
	/*
		b := blockchain.Newblock()
			for{
					if !m.started{
						contiune
					}

					h := caculDiftiy(b)
					if HashCacul < h(){
						//计算成功
						m.c <- Difficulty
					}
				}

	*/
	fmt.Println("计算难度成功")
}

//处理p2p,rpc，挖矿成功的区块，进行接收验证区块，然后打包出块
func (m *Miner) KtocoinMiner() {
	for {

		select {

		case cal := <-m.c:

			//难度算出来了
			b := block.NewBlock()
		//	b.Difficulty = cal
		//入库
		//	CommitBlcok(b)
		//	p2p.Broadcast(b)

		case p := <-m.bp:

			//验证p2p散播过来的区块
		//	VerifyBlock(p)
		//处理可能出现的分叉
		//	ProcessBlock()
		//入库
		//	CommitBlcok(b)

		case br := <-m.r:

			//验证Rpc散播过来的区块
			//	VerifyBlock(br)
			//处理可能出现的分叉
			//	ProcessBlock(br)
			//入库
			//	CommitBlcok(br)

		case d := <-m.download:
			//同步区块，暂停挖矿
			// m.Download()
			m.Stop()

		case s := <-m.startch:
			//同步区块，暂停挖矿
			// m.Download()
			m.Start()
		}

	}
	//m.wg.Done()
}

// start miner
func (m *Miner) Start() {
	if m.started {
		return
	}
	m.started = true

}

// stop miner
func (m *Miner) Stop() {
	if !m.started {
		return
	}
	m.started = false
}

func (m *Miner) Mining() bool {
	return m.started
}

// New returns a new instance of a CPU miner for the provided configuration.
// Use Start to begin the mining process.  See the documentation for CPUMiner
// type for more details.
func New(cfg *Config) *Miner {
	m := &Miner{
		coinbaseAddr: cfg.MiningAddrs,
		c:            make(chan struct{}),
		p:            make(chan struct{}),
		r:            make(chan struct{}),
		download:     make(chan bool),
		started:      false,
	}
	go m.KtocoinMiner()
	go m.CalcDifficulty()

	return m
}
