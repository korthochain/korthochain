package miner

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/blockchain"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/server/rpcserver/pb"
	"github.com/korthochain/korthochain/pkg/transaction"
	"github.com/korthochain/korthochain/pkg/txpool"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ipSampleInterval    = time.Second
	txsSampleInterval   = time.Second
	blockSampleInterval = time.Second * 2
	Hosts               = make(map[string]struct{})
)

var grpcPool *pool

func init() {
	grpcPool = newPool()
}

type InsideClient struct {
	bc      *blockchain.Blockchain
	cli     pb.InsideGreeterClient
	tp      *txpool.Pool
	conn    *grpc.ClientConn
	m       *Miner
	NetAddr string
}

func (i *InsideClient) Close() {
	if i == nil {
		return
	}

	// i.conn.Close()
	grpcPool.PutClient(i.NetAddr, i.conn)
	i = nil
}

func addressmanager(ip string) {

	_, exist := Hosts[ip]
	if !exist {
		return
	}
	delete(Hosts, ip)
}

func RemoveHostaddr(ip string) {

	if len(Hosts) < 6 {
		return
	}

	delete(Hosts, ip)
}

func NewInsideClient(address string, bc *blockchain.Blockchain, tp *txpool.Pool, miner *Miner) (*InsideClient, error) {

	//conn, err := grpc.Dial(address, grpc.WithInsecure(), grpc.WithTimeout(time.Second*2))
	conn, err := grpcPool.GetClient(address)
	if err != nil {
		logger.Info("grpc  dial err", zap.Error(err))
		return nil, err
	}

	client := pb.NewInsideGreeterClient(conn)

	inside := &InsideClient{cli: client, tp: tp, bc: bc, conn: conn, m: miner, NetAddr: address}
	err = inside.VerifyVersion()
	return inside, err

}

func Timeout(times int64) (context.Context, context.CancelFunc) {
	clienttime := time.Now().Add(time.Duration(times * int64(time.Second)))
	ctx, cancel := context.WithDeadline(context.Background(), clienttime)

	return ctx, cancel
}

func TimeOutErr(in error) error {
	stat, ok := status.FromError(in)
	if ok {

		if stat.Code() == codes.DeadlineExceeded {

			return errors.New("request timeout")
		}
	}
	return nil
}

func InitBlockChain(host string, bc *blockchain.Blockchain, tp *txpool.Pool, miner *Miner) error {
	var Client *InsideClient
	Client, err := NewInsideClient(host+":10001", bc, tp, miner)
	if err != nil {
		logger.ErrorLogger.Println("Connection failed :", err)
		return err
	}
	/* 	defer inside.Close() */

	err = Client.GetIPAddress()
	if err != nil {
		logger.ErrorLogger.Println("Sync peers addr err", err)
		return err
	}

	cliCh := make(chan *InsideClient, 1)
	start := make(chan int, 1)

	start <- 1
	go func() {

		for ip, _ := range Hosts {
			logger.SugarLogger.Info("new  client ip", ip)
			<-start
			Inclient, err := NewInsideClient(ip+":10001", bc, tp, miner)
			if err != nil {
				logger.InfoLogger.Println("Network connection failed,addr:", ip)
				continue
			}
			cliCh <- Inclient

		}
		panic("Network connection failed !!!")
	}()

	for {
		num, err := Client.GetBlockInitchains()
		if err != nil {
			Client.Close()
			start <- 1
			Client = <-cliCh
			continue
		}
		logger.SugarLogger.Info("num==", num)
		if num <= 0 {
			logger.SugarLogger.Error("*************Data initialization Finish************")
			Client.Close()
			break
		}
	}

	miner.UpdateDifficultyFromLastBlock()
	return nil
}

func Start(host string, bc *blockchain.Blockchain, tp *txpool.Pool, miner *Miner) {

	logger.Info("rpc ip", zap.String("host=", host))
	inside, err := NewInsideClient(host+":10001", bc, tp, miner)
	if err != nil {
		panic(err)
	}
	inside.GetIPAddress()

	go inside.NewConnectBlockChain()

}

func (inside *InsideClient) NewConnectBlockChain() {
	for {
		time.Sleep(blockSampleInterval)
		logger.InfoLogger.Printf(" Synchronizing......\n\n")
		for ip, _ := range Hosts {
			tmpClient, err := NewInsideClient(ip+":10001", inside.bc, inside.tp, inside.m)
			if err != nil {
				logger.SugarLogger.Error("NewConnectBlockChain newinsideclient", zap.Error(err))
				RemoveHostaddr(ip)
				continue
			}

			err = tmpClient.GetIPAddress()
			if err != nil {
				logger.SugarLogger.Error("NewConnectBlockChain GetIPAddress", zap.Error(err))
				RemoveHostaddr(ip)
				tmpClient.Close()
				continue
			}
			tmpClient.GetTransactions()
			tmpClient.GetBlockchain()
			tmpClient.Close()
			//	logger.InfoLogger.Println("Sync  peer block data ! take time=", time.Now().Sub(start1))

		}

	}
}

func (inside *InsideClient) GetIPAddress() error {

	/* 	var wg sync.WaitGroup */

	ctx, cencal := Timeout(1)
	defer cencal()

	resp, err := inside.cli.GetIPAddress(ctx, &pb.Req_IPAddress{})
	if err != nil {
		return err
	}

	if len(resp.Address) == 0 {
		return nil
	}

	for _, addr := range resp.Address {
		if len(Hosts) > 10 {
			return nil
		}
		_, ok := Hosts[addr]
		if !ok {
			Hosts[addr] = struct{}{}
		}
	}

	return nil

}

//100 block  commit
func (inside *InsideClient) GetBlockInitchains() (int, error) {

	defer logger.Info("*******************Data initialization End****************")

	var blocknum int

	h, err := inside.bc.GetMaxBlockHeight()
	if err != nil {
		logger.SugarLogger.Error("GetMaxBlockHeight err", zap.Error(err))
		return -1, err
	}

	for i := h; i >= blockchain.InitHeight; i-- {
		tipblock := &block.Block{}
		var respinfo *pb.RespBlockchain

		blocks := make([]*block.Block, 0)
		/* respinfo = nil */
		if i > 0 {
			tipblock, err = inside.bc.GetBlockByHeight(i)
			if err != nil {
				logger.SugarLogger.Error("get Tip blcok  err", zap.Error(err), i)
				return -1, err
			}
		}
		ctx, cencal := Timeout(2)
		defer cencal()
		respinfo, err = inside.cli.SyncBlockchain(ctx, &pb.ReqBlockchain{Hash: tipblock.Hash, Height: tipblock.Height})
		if err != nil {
			logger.SugarLogger.Error("SyncBlockchain err", err, tipblock.Height, "hash", hex.EncodeToString(tipblock.Hash))
			return -1, err
		}
		if respinfo.Code == -1 {
			logger.SugarLogger.Error("SyncBlockchain  code err ", respinfo.Code)
			return -1, err
		}

		if respinfo.Issamechain {
			err := inside.bc.DeleteBlock(i + 1)
			if err != nil {
				logger.SugarLogger.Error("DeleteBlock err ", err)
				return -1, err
			}

			for _, hash := range respinfo.Hashs {

				respblock, err := inside.cli.GetBlock(context.Background(), &pb.ReqBlock{Hash: hash})
				if err != nil {
					logger.SugarLogger.Error("get  GetBlock  err", err)
					return -1, err
				}
				block, err := block.Deserialize(respblock.Data)
				if err != nil {
					logger.SugarLogger.Error("Deserialize err", err)
					return -1, err
				}
				if respinfo.Height-block.Height < 200 {

					logger.SugarLogger.Error("start one  to one AddBlock ")
					if len(blocks) != 0 {
						err = inside.bc.AddBlocks(blocks, 100)
						if err != nil {
							logger.SugarLogger.Error("AddTempBlock err", err)
							return -1, err
						}
						blocks = nil
					}
					err := inside.bc.AddBlock(block)
					if err != nil {
						logger.SugarLogger.Error("AddBlock err", err)
						return -1, err
					}
					continue
				}
				blocks = append(blocks, block)
				if len(blocks) == 100 {
					err = inside.bc.AddBlocks(blocks, 100)
					if err != nil {
						logger.SugarLogger.Error("AddBlocks err", err)
						return -1, err
					}
					blocks = nil
					continue
				}

			}

			err = inside.bc.AddBlocks(blocks, 100)
			if err != nil {
				logger.SugarLogger.Error("AddBlocks err", err)
				return -1, err
			}
			blocknum = len(respinfo.Hashs)
			break

		} else if respinfo.Isbranchchain {
			parenthash := respinfo.Hashs[0]

			logger.SugarLogger.Info("branch point hash=[", hex.EncodeToString(parenthash), "]")

			b, err := inside.bc.GetBlockByHash(parenthash)
			if err != nil {
				logger.SugarLogger.Error("GetBlockchain GetBlockByHash err===", err)
				return -1, err
			}

			err = inside.bc.DeleteBlock(b.Height + 1)
			if err != nil {
				logger.SugarLogger.Error("GetBlockchain DeleteTempBlock err", err)
				return -1, err
			}

			for _, hash := range respinfo.Hashs[1:] {
				respblock, err := inside.cli.GetBlock(context.Background(), &pb.ReqBlock{Hash: hash})
				if err != nil {
					logger.SugarLogger.Error("get  GetBlock  err", err)
					return -1, err
				}
				b, err := block.Deserialize(respblock.Data)
				if err != nil {
					logger.SugarLogger.Error("Deserialize err", err)
					return -1, err
				}
				blocks = append(blocks, b)
				if len(blocks) == 100 {
					err = inside.bc.AddBlocks(blocks, 100)
					if err != nil {
						logger.SugarLogger.Error("AddTempBlock err", err)
						return -1, err
					}
					blocks = nil
					continue
				}

			}

			err = inside.bc.AddBlocks(blocks, 100)
			if err != nil {
				logger.SugarLogger.Error("AddBlocks err", err)
				return -1, err
			}

			blocknum = len(respinfo.Hashs)
			break
		}

	}
	//	logger.Info("*******************Data initialization end*******************")

	return blocknum, nil
}

func (inside *InsideClient) GetBlockInitchain() (int, error) {

	defer logger.Info("*******************Data initialization End****************")

	var blocknum int

	h, err := inside.bc.GetMaxBlockHeight()
	if err != nil {
		logger.SugarLogger.Error("GetMaxBlockHeight err", zap.Error(err))
		return -1, err
	}

	for i := h; i > blockchain.InitHeight; i-- {
		tipblock := &block.Block{}
		var respinfo *pb.RespBlockchain
		/* respinfo = nil */
		if i > 0 {
			tipblock, err = inside.bc.GetBlockByHeight(i)
			if err != nil {
				logger.SugarLogger.Error("get Tip blcok  err", zap.Error(err), i)
				return -1, err
			}
		}
		ctx, cencal := Timeout(2)
		defer cencal()
		respinfo, err = inside.cli.SyncBlockchain(ctx, &pb.ReqBlockchain{Hash: tipblock.Hash, Height: tipblock.Height})
		if err != nil {
			logger.SugarLogger.Error("SyncBlockchain err", zap.Error(err))
			return -1, err
		}
		if respinfo.Code == -1 {
			logger.SugarLogger.Error("SyncBlockchain  code err ", respinfo.Code)
			return -1, err
		}

		if respinfo.Issamechain {
			err := inside.bc.DeleteBlock(i + 1)
			if err != nil {
				logger.SugarLogger.Error("DeleteBlock err ", err)
				return -1, err
			}

			for _, hash := range respinfo.Hashs {

				respblock, err := inside.cli.GetBlock(context.Background(), &pb.ReqBlock{Hash: hash})
				if err != nil {
					logger.SugarLogger.Error("get  GetBlock  err", err)
					return -1, err
				}
				block, err := block.Deserialize(respblock.Data)
				if err != nil {
					logger.SugarLogger.Error("Deserialize err", err)
					return -1, err
				}

				//				logger.SugarLogger.Error("mainchain", zap.Uint64("height", block.Height), zap.String("hash", hex.EncodeToString(block.Hash)))

				/* 	err = inside.bc.AddTempBlock(block, db)
				if err != nil {
					logger.SugarLogger.Error("AddTempBlock err", err)
					return -1, err
				} */

				err = inside.bc.AddBlock(block)
				if err != nil {
					logger.SugarLogger.Error("AddTempBlock err", err)
					return -1, err
				}
			}
			blocknum = len(respinfo.Hashs)
			break

		} else if respinfo.Isbranchchain {
			parenthash := respinfo.Hashs[0]

			logger.SugarLogger.Info("branch point hash=[", hex.EncodeToString(parenthash), "]")

			b, err := inside.bc.GetBlockByHash(parenthash)
			if err != nil {
				logger.SugarLogger.Error("GetBlockchain GetBlockByHash err===", err)
				return -1, err
			}

			err = inside.bc.DeleteBlock(b.Height + 1)
			if err != nil {
				logger.SugarLogger.Error("GetBlockchain DeleteTempBlock err", err)
				return -1, err
			}

			for _, hash := range respinfo.Hashs[1:] {
				respblock, err := inside.cli.GetBlock(context.Background(), &pb.ReqBlock{Hash: hash})
				if err != nil {
					logger.SugarLogger.Error("get  GetBlock  err", err)
					return -1, err
				}
				b, err := block.Deserialize(respblock.Data)
				if err != nil {
					logger.SugarLogger.Error("Deserialize err", err)
					return -1, err
				}
				/* 		err = inside.bc.AddTempBlock(b, db)
				if err != nil {
					logger.SugarLogger.Error("GetBlockchain AddTempBlock err", err)
					return -1, err
				} */
				err = inside.bc.AddBlock(b)
				if err != nil {
					logger.SugarLogger.Error("GetBlockchain AddTempBlock err", err)
					return -1, err
				}
			}
			blocknum = len(respinfo.Hashs)
			break
		}

	}

	return blocknum, nil
}

func (inside *InsideClient) GetBlockchain() {

	h, err := inside.bc.GetMaxBlockHeight()
	if err != nil {
		logger.SugarLogger.Error("GetMaxBlockHeight err", zap.Error(err))
		return
	}
	//

	for i := h; i >= blockchain.InitHeight; i-- {
		tipblock := &block.Block{}
		var respinfo *pb.RespBlockchain

		if i > 1 {
			tipblock, err = inside.bc.GetBlockByHeight(i)
			if err != nil {
				logger.SugarLogger.Error("get Tip blcok  err", zap.Error(err))
				return
			}
		}

		ctx, cencel := Timeout(2)
		defer cencel()
		respinfo, err = inside.cli.GetBlockchain(ctx, &pb.ReqBlockchain{Hash: tipblock.Hash, Height: tipblock.Height})
		if err != nil {
			logger.SugarLogger.Error("get  GetBlockchain  err", zap.Error(err))
			return
		}
		if respinfo.Code == -1 {
			logger.SugarLogger.Error("get  GetBlockchain  err", zap.Error(err))
			return
		}

		if respinfo.Issamechain {
			if len(respinfo.Hashs) > 0 {
				inside.m.Stop()
				defer inside.m.Start()
			}

			for _, hash := range respinfo.Hashs {
				ctx, cencel := Timeout(2)
				defer cencel()
				respblock, err := inside.cli.GetBlock(ctx, &pb.ReqBlock{Hash: hash})
				if err != nil {
					logger.SugarLogger.Error("get  GetBlock  err", err)
					return
				}
				block, err := block.Deserialize(respblock.Data)
				if err != nil {
					logger.SugarLogger.Error("Deserialize err", err)
					return
				}

				inside.m.GetRPCBlockChan() <- block
			}
			break

		} else if respinfo.Isbranchchain {

			inside.m.Stop()
			defer inside.m.Start()

			for _, hash := range respinfo.Hashs[1:] {

				ctx, cencel := Timeout(2)
				defer cencel()
				respblock, err := inside.cli.GetBlock(ctx, &pb.ReqBlock{Hash: hash})
				if err != nil {
					logger.SugarLogger.Error("get  GetBlock  err", err)
					return
				}
				b, err := block.Deserialize(respblock.Data)
				if err != nil {
					logger.SugarLogger.Error("Deserialize err", err)
					return
				}
				inside.m.GetRPCBlockChan() <- b

			}
			break
		} else if respinfo.Heigher {

			if h-respinfo.Height < 20 {
				return
			}

			logger.SugarLogger.Info("respinfo.height=", respinfo.Height, "respinfo.hash", hex.EncodeToString(respinfo.Hashs[0]))
			resp, err := inside.cli.GetBlockTip(context.Background(), &pb.ReqBlockTip{})
			if err != nil {
				logger.SugarLogger.Error("get  GetBlockchain  err", zap.Error(err))
				return
			}

			tblock, err := block.Deserialize(resp.Data)
			if err != nil {
				logger.SugarLogger.Error("Deserialize err", zap.Error(err))
				return
			}

			equalHeightBlock, err := inside.bc.GetBlockByHeight(tblock.Height)
			if err != nil {
				logger.SugarLogger.Error("GetBlockByHeight err", err)
				return
			}

			if tblock.GlobalDifficulty.Cmp(equalHeightBlock.GlobalDifficulty) < 0 {
				logger.SugarLogger.Info("tblock.height=", tblock.Height, "tblock.hash", hex.EncodeToString(tblock.Hash))
				logger.SugarLogger.Info("myblock.height=", equalHeightBlock.Height, "myblock.hash", hex.EncodeToString(equalHeightBlock.Hash))
				logger.SugarLogger.Info("tblock.GlobalDifficulty=", tblock.GlobalDifficulty, "equalHeightBlock", equalHeightBlock.GlobalDifficulty)

				inside.m.Stop()
				defer inside.m.Start()

				if h-respinfo.Height > 5000 {
					logger.Info("  Please Restart Miner !!!")
					return
				}

				branch := inside.FindBranchPoint(respinfo.Height)

				if branch == 0 {
					logger.SugarLogger.Error("FindBranchPoint err", err)
					return
				}

				resp, err := inside.cli.GetBlockHashsByHeight(context.Background(), &pb.ReqBlockheight{Height: branch})
				if err != nil {
					logger.SugarLogger.Error("GetBlockHashsByHeight err", err)
					return
				}
				num := len(resp.Hashs)
				hashList := make([][]byte, num)
				for i, hash := range resp.Hashs {
					hashList[num-i-1] = hash
					respb, err := inside.cli.GetBlock(context.Background(), &pb.ReqBlock{Hash: hash})
					if err != nil {
						logger.SugarLogger.Error("GetBlockHashsByHeight err", err)
						return
					}
					b, err := block.Deserialize(respb.Data)
					if err != nil {
						logger.SugarLogger.Error("Deserialize err", err)
						return
					}
					err = inside.bc.AddUncleBlock(b)
					if err != nil {
						logger.SugarLogger.Error("AddUncleBlock err", err)
						return
					}

				}
				err = inside.bc.ReorganizeChain(hashList, branch+1)
				if err != nil {
					logger.SugarLogger.Error("ReorganizeChain err", err)
					return
				}
				return
			} else {
				branch := inside.FindBranchPoint(respinfo.Height)
				if h-branch > 100 {
					return
				}

				Height := branch
				h, err := inside.bc.GetMaxBlockHeight()
				if err != nil {
					logger.SugarLogger.Error("GetMaxBlockHeight err", zap.Error(err))
					return
				}
				for {
					Height++
					if Height > h {
						return
					}
					block, err := inside.bc.GetBlockByHeight(Height)
					if err != nil {
						logger.SugarLogger.Error("GetMaxBlockHeight err", zap.Error(err))
						return
					}

					err = inside.SendBlcok(block)
					if err != nil {
						logger.SugarLogger.Error("SendBlock err", zap.Error(err))
						return
					}
				}
			}

		}

	}

}

func (inside *InsideClient) FindBranchPoint(height uint64) uint64 {

	var branchPoint uint64
	for i := height; i > blockchain.InitHeight; i-- {
		myself, err := inside.bc.GetBlockByHeight(i)
		if err != nil {
			logger.SugarLogger.Error("get  GetBlockByHeight  err", zap.Error(err))
			return 0
		}
		branch, err := inside.cli.FindbranchPoint(context.Background(), &pb.ReqBranch{Hash: myself.Hash, Height: i})
		if err != nil {
			logger.SugarLogger.Error("get  GetBlockchain  err", zap.Error(err))
			return 0
		}
		if branch.Exist {
			branchPoint = branch.Height
			break
		}
	}
	return branchPoint
}

func (inside *InsideClient) SendBlcok(b *block.Block) error {

	genesisHash := inside.m.GenesisHash

	data, err := b.Serialize()
	if err != nil {
		logger.SugarLogger.Error("block  Serialize", err)
		log.Fatal(err)
	}
	ctx, cencal := Timeout(2)
	defer cencal()
	_, err = inside.cli.SendBlock(ctx, &pb.ReqSendBlcok{Block: data, Genesishash: genesisHash})
	if err != nil {
		logger.SugarLogger.Error("rpc SendBlock err", err)
		return err
	}
	return nil
}

func (inside *InsideClient) GetBlockByHash(hash []byte) (*block.Block, error) {
	resp, err := inside.cli.GetBlock(context.Background(), &pb.ReqBlock{Hash: hash})
	if err != nil {
		return nil, err
	}
	return block.Deserialize(resp.Data)
}

func (inside *InsideClient) GetTransactions() {

	ctx, cencel := Timeout(2)
	defer cencel()

	resptxs, err := inside.cli.GetTransactions(ctx, &pb.ReqTxhash{})
	if err != nil {
		logger.SugarLogger.Error("GetTransactions err", zap.Error(err))
		return
	}

	for _, tx := range resptxs.Txs {
		signtx, err := transaction.DeserializeSignaturedTransaction(tx)
		if err != nil {
			logger.SugarLogger.Error("DeserializeSignaturedTransaction err", zap.Error(err))
			return
		}
		err = inside.tp.Add(signtx)
		if err != nil {

			continue
		}
	}

}

func (inside *InsideClient) VerifyVersion() error {
	ctx, cencal := Timeout(2)
	defer cencal()
	CheckHash := inside.m.GenesisHash
	resp, err := inside.cli.VerifyVersion(ctx, &pb.ReqVersion{})
	if err != nil {
		logger.SugarLogger.Info("VerifyVersion err", err)
		return err
	}
	if CheckHash != resp.Versioninfo {
		return fmt.Errorf("resp Versioninfo=[%v]", resp.Versioninfo)
	}
	return nil

}
