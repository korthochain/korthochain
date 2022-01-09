package miner

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/korthochain/korthochain/pkg/address"
	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/blockchain"
	"github.com/korthochain/korthochain/pkg/consensus"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/miner/hash"
	"github.com/korthochain/korthochain/pkg/p2p"
	"github.com/korthochain/korthochain/pkg/transaction"
	"github.com/korthochain/korthochain/pkg/txpool"
	"github.com/korthochain/korthochain/pkg/util/difficulty"
	"go.uber.org/zap"
)

var Cycle uint64 = 10

var cpuNum = runtime.NumCPU()

var (
	globalBits = uint32(0x20ffff80)
	localBits  = globalBits
	initBits   = globalBits
)

// hash/s
var lastPendingSub float64
var numberCalculations = uint64(0)

const maxPledge uint64 = 1200000000000000

func SetConf(avlcpunum, cycle int) error {
	if avlcpunum > 0 && avlcpunum <= runtime.NumCPU() {
		cpuNum = avlcpunum
	}

	if cycle > 0 {
		Cycle = uint64(cycle)
	}

	fmt.Println("++++++++++++++++++init+++++++++++++++++++++")
	fmt.Printf("+ cpu num: %4d                           +\n", cpuNum)
	fmt.Printf("+ cycle  : %4d                           +\n", Cycle)
	fmt.Println("+++++++++++++++++++++++++++++++++++++++++++")
	return nil
}

var powLimit = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 255), big.NewInt(1))

// Config is a descriptor containing the cpu miner configuration.
type Config struct {
	// MiningAddrs is a list of payment addresses to use for the generated
	// blocks.  Each generated block will randomly choose one of them.
	MiningAddr     string `yaml:"MiningAddr"`
	DifficultyBits string `yaml:"DifficultyBits"`
	GenesisHash    string `yaml:"GenesisHash"`
	NoMining       bool   `yaml:"NoMining"`
}

type Miner struct {
	tp              *txpool.Pool
	bc              *blockchain.Blockchain
	cbc             *consensus.BlockChain
	submitBlockLock sync.Mutex
	wg              sync.WaitGroup
	coinbaseAddr    address.Address //Miner  address
	c               chan block.Block
	p               chan *block.Block
	r               chan *block.Block
	startch         chan bool
	download        chan bool
	started         bool

	MiningSig          chan struct{}
	BreakMiningSig     chan struct{}
	node               *p2p.Node
	AdvertiseAddr      string
	miningTransactions []transaction.SignedTransaction
	InitBits           uint32
	GenesisHash        string
}

func (m *Miner) AcceptBlockFromP2P(b *block.Block) {
	m.p <- b
}

func (m *Miner) AcceptBlockFromRPC(b *block.Block) {
	m.r <- b
}

func (m *Miner) GetP2PBlockChan() chan *block.Block {
	return m.p
}

func (m *Miner) GetRPCBlockChan() chan *block.Block {
	return m.r
}

func closeStopCh(stopCh, toStopCh chan struct{}) {
	<-toStopCh
	lastPendingSub = time.Since(tmpTime).Seconds()
	tmpNum = atomic.LoadUint64(&numberCalculations)
	close(stopCh)
}

func (m *Miner) sectionCalcDifficulty(stopCh, toStopCh chan struct{}, b block.Block, start, end uint64, target *big.Int) {
	for i := start; i < end; i++ {
		select {
		case <-stopCh:
			return
		default:
			b.Nonce = i
			b.Difficulty = target
			if HashToBig(hash.Hash(b.MinerHash())).Cmp(target) < 0 {
				select {
				case toStopCh <- struct{}{}:
					m.c <- b
				default:
					return
				}
			}
			atomic.AddUint64(&numberCalculations, 1)
		}
	}
}

var tmpTime time.Time
var tmpNum uint64

func (m *Miner) MultiCalcDifficulty() {
	tmpTime = time.Now()
	for range m.MiningSig {

		txs, err := m.tp.Pending()
		if err != nil {
			logger.Error("CalcDifficulty pending", zap.Error(err))
			return
		}
		stxs := make([]*transaction.SignedTransaction, len(txs))
		for i, _ := range txs {
			stxs[i] = &txs[i]
		}

		m.miningTransactions = make([]transaction.SignedTransaction, len(txs))
		copy(m.miningTransactions, txs)

		b, err := m.bc.NewBlock(stxs, m.coinbaseAddr)
		if err != nil {
			logger.Error("CalcDifficulty NewBlock", zap.Error(err))
			return
		}

		tmpBits := atomic.LoadUint32(&localBits)
		target := CompactToBig(tmpBits)

		sectionNum := math.MaxUint64 / uint64(cpuNum)
		stopCh, toStopCh := make(chan struct{}), make(chan struct{})

		go closeStopCh(stopCh, toStopCh)
		for i := uint64(0); i < uint64(cpuNum); i++ {
			go m.sectionCalcDifficulty(stopCh, toStopCh, *b, i*sectionNum, (i+1)*sectionNum, target)
		}
		logger.InfoLogger.Printf(" Mining......\n\n")

		go func() {

			select {
			case <-m.BreakMiningSig:
				toStopCh <- struct{}{}
			case <-stopCh:
				return
			}

		}()
		<-stopCh

		{
			tmpTime = time.Now()
			atomic.StoreUint64(&numberCalculations, 0)
		}
	}
}

func (m *Miner) KtocoinMiner(flag int) {

	t0 := time.Now()
	lastblocktime := time.Now().Unix()

	for {
		select {
		case b := <-m.c:
			t0 = time.Now()
			tmpTime := time.Now()
			logger.Info("start add block", zap.Int64("timestamp", t0.Unix()))
			b.Difficulty = CompactToBig(localBits)
			b.GlobalDifficulty = CompactToBig(globalBits)
			if t := time.Now().Unix() - lastblocktime; t > 0 {
				b.UsedTime = uint64(t)
			}

			if err := b.SetHash(); err != nil {
				panic(err)
			}

			logger.InfoLogger.Printf(" Myself Mined Succ	block Height=[%d]	 miner=[%v]    hash=[%v]\n\n", b.Height, b.Miner, hex.EncodeToString(b.Hash))
			logger.SugarLogger.Infof("success	time:%fs", time.Since(t0).Seconds())
			logger.Info("MYSELF Mined Succ", zap.String("miner", b.Miner.String()), zap.Uint64("height", b.Height), zap.String("block hash", hex.EncodeToString(b.Hash)))
			basePledge, err := m.GetBasePledge()
			if err != nil {
				continue
			}
			ok := m.cbc.ProcessBlock(&b, CompactToBig(globalBits), basePledge)

			//p2p

			t0 = time.Now()
			logger.Info("end add block", zap.Int64("timestamp", t0.Unix()))
			if ok {
				lastblocktime = tmpTime.Unix()

				if flag == 0 {

					blockHead := block.BlockHead{
						Height:      b.Height,
						Hash:        b.Hash,
						Host:        m.AdvertiseAddr,
						Port:        "10001",
						GenesisHash: m.GenesisHash,
					}

					data, _ := json.Marshal(blockHead)
					go m.node.SendMessage(0, append([]byte{2}, data...))
				} else {
					go func() {

						var succ int
						for host, _ := range Hosts {
							client, err := NewInsideClient(host+":10001", m.bc, m.tp, m)
							if err != nil {
								logger.Error("KtocoinMiner SendBlock", zap.Error(err), zap.String("ip", host))
								RemoveHostaddr(host)
								continue
							}

							err = client.SendBlcok(&b)
							if err != nil {
								logger.Error("KtocoinMiner SendBlock", zap.Error(err))
								client.Close()
								RemoveHostaddr(host)
								continue
							}

							client.Close()
							logger.Info("SendBlcok succ")
							succ++
							if succ > 5 {
								break
							}

						}
					}()
				}

				if err := m.calcNextRequiredDifficulty(m.coinbaseAddr); err != nil {
					logger.Error("KtocoinMiner_calcNextRequiredDifficulty", zap.Error(err))
				}
			}

			stList := []transaction.SignedTransaction{}
			for _, ft := range b.Transactions {
				stList = append(stList, ft.SignedTransaction)
			}
			m.tp.FilterTransaction(stList)

			t0 = time.Now()
			logger.Info("send block", zap.Int64("timestamp", t0.Unix()))
			m.Start()

		case p := <-m.p:
			m.Stop()
			t0 = time.Now()
			tmpTime := time.Now()
			logger.Info("start add block", zap.Int64("timestamp", t0.Unix()))
			logger.Info("KtocoinMiner-p2p", zap.Uint64("height", p.Height), zap.String("reciver block hash", hex.EncodeToString(p.Hash)))
			basePledge, err := m.GetBasePledge()
			if err != nil {
				continue
			}
			ok := m.cbc.ProcessBlock(p, CompactToBig(globalBits), basePledge)
			t0 = time.Now()
			logger.Info("end add block", zap.Int64("timestamp", t0.Unix()))
			if ok {
				lastblocktime = tmpTime.Unix()
				m.calcNextRequiredDifficulty(m.coinbaseAddr)

				stList := []transaction.SignedTransaction{}
				for _, ft := range p.Transactions {
					stList = append(stList, ft.SignedTransaction)
				}
				m.tp.FilterTransaction(stList)
			}

			t0 = time.Now()
			logger.Info("send block", zap.Int64("timestamp", t0.Unix()))
			m.Start()

		case p := <-m.r:
			m.Stop()
			t0 = time.Now()
			tmpTime := time.Now()
			logger.Info("start add block", zap.Int64("timestamp", t0.Unix()))
			logger.Info("KtocoinMiner--RPC", zap.Uint64("height", p.Height), zap.String("reciver block hash", hex.EncodeToString(p.Hash)))
			basePledge, err := m.GetBasePledge()
			if err != nil {
				continue
			}
			ok := m.cbc.ProcessBlock(p, CompactToBig(globalBits), basePledge)
			t0 = time.Now()
			logger.Info("end add block", zap.Int64("timestamp", t0.Unix()))
			b := *p

			if ok {
				lastblocktime = tmpTime.Unix()

				if flag == 0 {

					blockHead := block.BlockHead{
						Height:      b.Height,
						Hash:        b.Hash,
						Host:        m.AdvertiseAddr,
						Port:        "10001",
						GenesisHash: m.GenesisHash,
					}

					data, _ := json.Marshal(blockHead)
					go m.node.SendMessage(0, append([]byte{2}, data...))
				}
				m.calcNextRequiredDifficulty(m.coinbaseAddr)

				stList := []transaction.SignedTransaction{}
				for _, ft := range b.Transactions {
					stList = append(stList, ft.SignedTransaction)
				}
				m.tp.FilterTransaction(stList)
			}

			t0 = time.Now()
			logger.Info("send block", zap.Int64("timestamp", t0.Unix()))
			m.Start()
		case <-m.download:
			m.Stop()

		case <-m.startch:
			m.Start()
		}

	}
}

// start miner
func (m *Miner) Start() {
	if m.MiningSig == nil {
		return
	}

	select {
	case m.MiningSig <- struct{}{}:
	default:
	}

	if m.started {
		return
	}
	m.started = true
}

// stop miner
func (m *Miner) Stop() {
	if m.BreakMiningSig == nil {
		return
	}

	select {
	case m.BreakMiningSig <- struct{}{}:
	default:
	}

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
func New(cfg *Config, bc *blockchain.Blockchain, tp *txpool.Pool, node *p2p.Node, cb *consensus.BlockChain, mode int, AdvertiseAddr string) (*Miner, error) {

	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}
	logger.InfoLogger.Println("Peer profile version:", cfg.GenesisHash)
	logger.InfoLogger.Println("peer miner address:", cfg.MiningAddr)
	miningAddr, err := address.NewAddrFromString(cfg.MiningAddr)
	if err != nil {
		fmt.Println("NewAddrFromString", err)
		return nil, err
	}

	m := &Miner{
		coinbaseAddr: miningAddr,
		c:            make(chan block.Block, 100),
		p:            make(chan *block.Block, 100),
		r:            make(chan *block.Block, 100),
		download:     make(chan bool),
		started:      false,
		tp:           tp,
		bc:           bc,
		cbc:          cb,

		node:          node,
		AdvertiseAddr: AdvertiseAddr,
		GenesisHash:   cfg.GenesisHash,
	}

	initBits, err := strconv.ParseUint(cfg.DifficultyBits, 0, 32)
	if err != nil {
		fmt.Println("ParseUint", err)
		return nil, err
	}

	m.InitBits = uint32(initBits)
	globalBits = m.InitBits

	m.calcNextRequiredDifficulty(m.coinbaseAddr)

	if !cfg.NoMining {
		m.MiningSig = make(chan struct{}, 1)
		m.BreakMiningSig = make(chan struct{}, 1)
		go m.MultiCalcDifficulty()
	}

	go m.KtocoinMiner(mode)

	//m.MiningSig <- struct{}{}
	return m, nil
}

func (m *Miner) GetBasePledge() (uint64, error) {
	total, pledgeTotal, err := m.bc.GetWholeNetworkPledgeAndNum()
	if err != nil {
		return 0, err
	}

	bigTotal := big.NewInt(0).SetUint64(total)
	bigPledgeTotal := big.NewInt(0).SetUint64(pledgeTotal)

	var rate uint64
	if bigTotal.Cmp(big.NewInt(0)) != 0 {
		rate = bigPledgeTotal.Mul(bigPledgeTotal, big.NewInt(100)).Div(bigPledgeTotal, bigTotal).Uint64()
	} else {
		rate = 0
	}

	if rate > 100 {
		return 0, fmt.Errorf("rate(%d) is not in the range of 0~100", rate)
	}

	basePledge := maxPledge * (100 - rate) / 100
	return basePledge, nil
}

func (m *Miner) UpdateDifficultyFromLastBlock() (*block.Block, error) {
	newGlobalBits := CompactToBig(globalBits)
	tip, err := m.bc.Tip()
	if err != nil {
		logger.Error("get tip", zap.Error(err))
	}

	if tip != nil {
		newGlobalBits = tip.GlobalDifficulty
	}
	basePledge, err := m.GetBasePledge()
	if err != nil {
		logger.Error("get GetBasePledge", zap.Error(err))
		return nil, err
	}

	newlocalBits, err := difficulty.NextMinerDifficulty(newGlobalBits, 0, basePledge)
	if err != nil {
		logger.Error("get NextMinerDifficulty", zap.Error(err))
		return nil, err
	}
	atomic.CompareAndSwapUint32(&globalBits, globalBits, BigToCompact(newGlobalBits))
	atomic.CompareAndSwapUint32(&localBits, localBits, BigToCompact(newlocalBits))
	return tip, nil
}

func (m *Miner) calcNextRequiredDifficulty(coinbaseAddr address.Address) error {
	tip, err := m.UpdateDifficultyFromLastBlock()
	if err != nil {
		logger.Error("UpdateDifficultyFromLastBlock err", zap.Error(err))
		return err
	}

	subTime := uint64(0)
	if tip != nil && tip.Height > 1 && (tip.Height-1)%Cycle == 0 {
		var err error
		var ob *block.Block
		hash := tip.Hash
		for i := 0; i < 10; i++ {
			ob, err = m.bc.GetBlockByHash(hash)
			if err != nil {
				logger.Error("GetBlockByHash err", zap.Error(err), zap.String("hash", hex.EncodeToString(hash)))
				return err
			}

			subTime += ob.UsedTime
			hash = ob.PrevHash
		}

		newGlobalBits := difficulty.CalcNextGlobalRequiredDifficulty(0, int64(subTime), globalBits)
		logger.Info("update Difficulty", zap.Uint64("sub time", subTime), zap.Uint32("oldGlobalDifficultyBits", globalBits), zap.Uint32("newGlobalDifficultyBits", newGlobalBits))

		basePledge, err := m.bc.GetBasePledge()
		if err != nil {
			return err
		}

		newlocalBits, err := difficulty.NextMinerDifficulty(CompactToBig(newGlobalBits), 0, basePledge)
		if err != nil {
			return err
		}

		atomic.CompareAndSwapUint32(&globalBits, globalBits, newGlobalBits)
		atomic.CompareAndSwapUint32(&localBits, localBits, BigToCompact(newlocalBits))

		logger.SugarLogger.Debug(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
		logger.SugarLogger.Debugf("cycle:%d  avg time:%v  globalBits:%d,localBits:%d",
			Cycle, time.Unix(int64(tip.Timestamp), 0).Sub(time.Unix(int64(ob.Timestamp), 0))/time.Duration(Cycle), globalBits, localBits)
		logger.SugarLogger.Debug("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<")
	}

	return nil
}

func HashToBig(buf []byte) *big.Int {
	blen := len(buf)
	for i := 0; i < blen/2; i++ {
		buf[i], buf[blen-1-i] = buf[blen-1-i], buf[i]
	}
	return new(big.Int).SetBytes(buf[:])
}

func CompactToBig(compact uint32) *big.Int {
	mantissa := compact & 0x007fffff
	isNegative := compact&0x00800000 != 0
	exponent := uint(compact >> 24)
	var bn *big.Int
	if exponent <= 3 {
		mantissa >>= 8 * (3 - exponent)
		bn = big.NewInt(int64(mantissa))
	} else {
		bn = big.NewInt(int64(mantissa))
		bn.Lsh(bn, 8*(exponent-3))
	}
	if isNegative {
		bn = bn.Neg(bn)
	}
	return bn
}

func BigToCompact(n *big.Int) uint32 {
	if n.Sign() == 0 {
		return 0
	}
	var mantissa uint32
	exponent := uint(len(n.Bytes()))
	if exponent <= 3 {
		mantissa = uint32(n.Bits()[0])
		mantissa <<= 8 * (3 - exponent)
	} else {
		tn := new(big.Int).Set(n)
		mantissa = uint32(tn.Rsh(tn, 8*(exponent-3)).Bits()[0])
	}
	if mantissa&0x00800000 != 0 {
		mantissa >>= 8
		exponent++
	}
	compact := uint32(exponent<<24) | mantissa
	if n.Sign() < 0 {
		compact |= 0x00800000
	}
	return compact
}

func (m *Miner) MiningSignedTransaction() []transaction.SignedTransaction {
	return m.miningTransactions
}

func (m *Miner) OrphanBlockIsExist(hash []byte) (*block.Block, bool) {
	return m.cbc.OrphanBlockIsExist(hash)
}

type HasherInfo struct {
	UUID            string
	MinerAddr       address.Address
	HasherPerSecond float64
}

func (m *Miner) HashesPerSecond() HasherInfo {
	var HasherPerSecond float64
	if lastPendingSub == 0 {
		HasherPerSecond = 0
	} else {
		HasherPerSecond = float64(tmpNum) / lastPendingSub
	}

	h := HasherInfo{
		MinerAddr:       m.coinbaseAddr,
		HasherPerSecond: HasherPerSecond,
	}

	return h
}
