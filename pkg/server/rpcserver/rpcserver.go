package rpcserver

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/korthochain/korthochain/pkg/block"
	"github.com/korthochain/korthochain/pkg/blockchain"
	"github.com/korthochain/korthochain/pkg/config"
	"github.com/korthochain/korthochain/pkg/logger"
	"github.com/korthochain/korthochain/pkg/miner"
	"github.com/korthochain/korthochain/pkg/p2p"
	"github.com/korthochain/korthochain/pkg/server"
	"github.com/korthochain/korthochain/pkg/server/rpcserver/pb"
	"github.com/korthochain/korthochain/pkg/storage/store"
	"github.com/korthochain/korthochain/pkg/txpool"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type InsideGreeter struct {
	Bc   *blockchain.Blockchain
	Tp   *txpool.Pool
	Cfg  *config.CfgInfo
	Node *p2p.Node
	m    *miner.Miner
}

func NewInsideGreeter(bc *blockchain.Blockchain, tp *txpool.Pool, node *p2p.Node, cfg *config.CfgInfo, min *miner.Miner) *InsideGreeter {
	return &InsideGreeter{Bc: bc, Tp: tp, Cfg: cfg, Node: node, m: min}
}

func (g *InsideGreeter) RunInsideGrpc() {
	lis, err := net.Listen("tcp", g.Cfg.SververCfg.InsiderpcAddress)
	if err != nil {
		logger.Error("net.Listen", zap.Error(err))
		os.Exit(-1)
	}

	server := grpc.NewServer(grpc.UnaryInterceptor(server.IpInterceptor))
	pb.RegisterInsideGreeterServer(server, g)

	server.Serve(lis)
	logger.SugarLogger.Error("RunInsideGrpc")
}

func (g *InsideGreeter) GetBlock(cxt context.Context, in *pb.ReqBlock) (*pb.RespBlock, error) {

	b, err := g.Bc.GetBlockByHash(in.Hash)
	if err != nil {
		return nil, err
	}
	data, err := b.Serialize()
	if err != nil {
		return nil, err
	}
	return &pb.RespBlock{Data: data, Code: 0}, nil

}

func (g *InsideGreeter) GetBlockHashsByHeight(cxt context.Context, in *pb.ReqBlockheight) (*pb.RespBlockhashs, error) {

	hash, err := GetAfterHashs(g.Bc, in.Height)
	if err != nil {
		return &pb.RespBlockhashs{}, err
	}
	return &pb.RespBlockhashs{Hashs: hash}, nil

}
func (g *InsideGreeter) GetBlockTip(cxt context.Context, in *pb.ReqBlockTip) (*pb.RespBlock, error) {
	b, err := g.Bc.Tip()
	if err != nil {
		logger.Error("inside rpc GetBlockTip err", zap.Error(err))
		return nil, err
	}
	data, err := b.Serialize()
	if err != nil {
		logger.Error("inside rpc Serialize err", zap.Error(err))
		return nil, err
	}
	return &pb.RespBlock{Data: data, Code: 0}, nil
}
func (g *InsideGreeter) GetBlockchain(cxt context.Context, in *pb.ReqBlockchain) (*pb.RespBlockchain, error) {

	hashs := make([][]byte, 0)

	if in.Hash == nil && in.Height == 1 {
		hashs, err := GetAfterHashs(g.Bc, 0)
		if err != nil {
			logger.SugarLogger.Error("get GetBlockByHash", err)
			return &pb.RespBlockchain{Code: -1}, err
		}
		return &pb.RespBlockchain{Hashs: hashs, Code: 0, Issamechain: true}, nil
	}

	tipblock, err := g.Bc.Tip()
	if err != nil {
		return &pb.RespBlockchain{Code: -1}, err
	}

	//request node's tip block same as tip block
	if bytes.Equal(tipblock.Hash, in.Hash) {
		return &pb.RespBlockchain{Code: 0, Issamechain: true}, nil
	}

	//local tip block height less than request peer tip block height
	if tipblock.Height < in.Height {
		hashs = append(hashs, tipblock.Hash)
		return &pb.RespBlockchain{Code: 0, Heigher: true, Height: tipblock.Height, Hashs: hashs}, err
	}

	hash, err := g.Bc.GetHash(in.Height)
	if err != nil && err != store.NotExist {
		return &pb.RespBlockchain{Code: -1}, err
	}

	if bytes.Equal(hash, in.Hash) {
		hashs, err = GetAfterHashs(g.Bc, in.Height)
		if err != nil {
			logger.SugarLogger.Error("get GetBlockByHash", err)
			return &pb.RespBlockchain{Code: -1}, err
		}
		return &pb.RespBlockchain{Hashs: hashs, Code: 0, Issamechain: true}, nil

	} else {

		//Find parent block of main chain and branch
		_, err := g.Bc.GetBlockByHash(in.Hash)
		if err == store.NotExist {
			return &pb.RespBlockchain{Hashs: hashs, Code: 0}, nil
		}
		mainchainblock, err := g.Bc.FindChainBranch(in.Hash)
		if err != nil {
			logger.SugarLogger.Error("get block chain___ hash:%s,error:%s\n", hex.EncodeToString(in.Hash), err)
			return &pb.RespBlockchain{Code: -1}, err
		}

		hashs = append(hashs, mainchainblock.Hash)
		afterhash, err := GetAfterHashs(g.Bc, mainchainblock.Height)
		if err != nil {
			return &pb.RespBlockchain{Code: -1}, err
		}
		hashs = append(hashs, afterhash...)
	}

	return &pb.RespBlockchain{Hashs: hashs, Code: 0, Isbranchchain: true}, nil
}

func (g *InsideGreeter) FindbranchPoint(cxt context.Context, in *pb.ReqBranch) (*pb.RespBranch, error) {

	hash, err := g.Bc.GetHash(in.Height)
	if err != nil && err != store.NotExist {
		return &pb.RespBranch{}, err
	}

	//request node's tip block same as tip block
	if !bytes.Equal(hash, in.Hash) {
		return &pb.RespBranch{Exist: false}, nil
	}

	return &pb.RespBranch{Height: in.Height, Exist: true}, nil

}

func (g *InsideGreeter) SyncBlockchain(cxt context.Context, in *pb.ReqBlockchain) (*pb.RespBlockchain, error) {

	hashs := make([][]byte, 0)
	tipblock, err := g.Bc.Tip()
	if err != nil {
		return &pb.RespBlockchain{Code: -1}, err
	}

	if in.Hash == nil && in.Height == 0 {
		hashs, err := GetAfterHashs(g.Bc, 0)
		if err != nil {
			logger.SugarLogger.Error("get GetBlockByHash", err)
			return &pb.RespBlockchain{Code: -1}, fmt.Errorf("GetAfterHashs err[%v]", err)
		}

		if len(hashs) > 1000 {
			hashs = hashs[:1000]
		}
		return &pb.RespBlockchain{Hashs: hashs, Code: 0, Issamechain: true, Height: tipblock.Height}, nil
	}

	if tipblock.Height < in.Height {
		return &pb.RespBlockchain{Code: 0}, nil
	}

	//request node's tip block same as tip block
	if bytes.Equal(tipblock.Hash, in.Hash) {
		return &pb.RespBlockchain{Code: 0, Issamechain: true, Height: tipblock.Height}, nil
	}

	hash, err := g.Bc.GetHash(in.Height)
	if err != nil && err != store.NotExist {
		return &pb.RespBlockchain{Code: -1}, fmt.Errorf("GetHash err[%v]", err)
	}

	if bytes.Equal(hash, in.Hash) {
		hashs, err = GetAfterHashs(g.Bc, in.Height)
		if err != nil {
			logger.SugarLogger.Error("get GetBlockByHash", err)
			return &pb.RespBlockchain{Code: -1}, fmt.Errorf("GetAfterHashs1 err[%v]", err)
		}
		if len(hashs) > 1000 {
			hashs = hashs[:1000]
		}
		return &pb.RespBlockchain{Hashs: hashs, Code: 0, Issamechain: true, Height: tipblock.Height}, nil

	} else {

		_, err := g.Bc.GetBlockByHash(in.Hash)

		if err != nil && err != store.NotExist {
			logger.SugarLogger.Error("get GetBlockByHash hash:%s,error:%s\n", hex.EncodeToString(in.Hash), err)
			return &pb.RespBlockchain{Code: -1}, fmt.Errorf("GetBlockByHash err[%v]", err)
		}

		if err == store.NotExist {
			return &pb.RespBlockchain{Code: 0}, nil
		}
		mainchainblock, err := g.Bc.FindChainBranch(in.Hash)
		if err != nil {
			logger.SugarLogger.Error("get block chain___ hash:%s,error:%s\n", hex.EncodeToString(in.Hash), err)

			return &pb.RespBlockchain{Code: -1}, fmt.Errorf("FindChainBranch err[%v]", err)
		}

		hashs = append(hashs, mainchainblock.Hash)
		afterhash, err := GetAfterHashs(g.Bc, mainchainblock.Height)
		if err != nil {
			return &pb.RespBlockchain{Code: -1}, fmt.Errorf("GetAfterHashs2 err[%v]", err)
		}
		hashs = append(hashs, afterhash...)
	}

	if len(hashs) > 1000 {
		hashs = hashs[:1000]
	}

	return &pb.RespBlockchain{Hashs: hashs, Code: 0, Isbranchchain: true, Height: tipblock.Height}, nil
}

//get hash  list from  blockchain if block  height
func GetAfterHashs(bc *blockchain.Blockchain, height uint64) ([][]byte, error) {
	re := make([][]byte, 0)
	for i := 1; ; i++ {

		hash, err := bc.GetHash(height + uint64(i))
		if err == store.NotExist {
			/* logger.Error("GetHash hash by height", zap.Error(err), zap.Uint64("height", height+uint64(i))) */
			break
		}
		if err != nil {
			logger.Error("GetHash hash by height", zap.Error(err), zap.Uint64("height", height+uint64(i)))
			return re, err
		}

		re = append(re, hash)
	}
	return re, nil

}
func (g *InsideGreeter) GetIPAddress(cxt context.Context, in *pb.Req_IPAddress) (*pb.Resp_IPAddress, error) {
	members := g.Node.Members()

	ret := &pb.Resp_IPAddress{}
	ret.Address = append(ret.Address, g.Cfg.P2PConfig.AdvertiseAddr)
	for _, member := range members {

		ret.Address = append(ret.Address, member.Addr.String())
	}
	return ret, nil
}

func (g *InsideGreeter) GetTransaction(cxt context.Context, in *pb.ReqTx) (*pb.RespTx, error) {
	return &pb.RespTx{}, nil
}

func (g *InsideGreeter) GetTransactions(cxt context.Context, in *pb.ReqTxhash) (*pb.RespTxhash, error) {
	poolTxs := g.Tp.CopySignedTransactions()
	minerTxs := g.m.MiningSignedTransaction()

	txs := append(poolTxs, minerTxs...)
	resp := &pb.RespTxhash{}
	for _, tx := range txs {
		data, err := tx.Serialize()
		if err != nil {
			continue
		}
		resp.Txs = append(resp.Txs, data)
	}

	return resp, nil
}

//chuli fencha
func (g *InsideGreeter) SendBlock(cxt context.Context, in *pb.ReqSendBlcok) (*pb.RespSendBlock, error) {

	if g.m.GenesisHash != in.Genesishash {
		//logger.SugarLogger.Error("Genesishash  not equal,local=[", g.m.GenesisHash, "],recv=[", in.Genesishash, "]")
		return &pb.RespSendBlock{}, nil
	}

	b, err := block.Deserialize(in.Block)
	if err != nil {
		return nil, err
	}

	ch := g.m.GetRPCBlockChan()
	ch <- b

	return &pb.RespSendBlock{}, nil

}

func (g *InsideGreeter) VerifyVersion(cxt context.Context, in *pb.ReqVersion) (*pb.RespVersion, error) {

	version := g.Cfg.MinerConfig.GenesisHash
	if len(version) < 0 {
		return nil, fmt.Errorf("version is nil")
	}
	return &pb.RespVersion{Versioninfo: version}, nil
}

func (g *InsideGreeter) AllStream(allStr pb.InsideGreeter_AllStreamServer) error {

	data, _ := allStr.Recv()
	log.Println(data)

	if data.Height == 0 {

	}

	re, err := g.GetBlockTip(context.Background(), &pb.ReqBlockTip{})
	if err != nil {
		return err
	}

	block, err := block.Deserialize(re.Data)
	if err != nil {
		return err
	}

	if data.Height == int64(block.Height) && bytes.Equal(data.Hash, block.Hash) {
		return nil
	}

	if data.Height < int64(block.Height) {
		hash, err := g.Bc.GetHash(uint64(data.Height))
		if err != nil {
			return err
		}
		if bytes.Equal(hash, data.Hash) {
			err := allStr.Send(&pb.StreamResData{})
			if err != nil {
				return err
			}
			return nil
		} else {
			return nil
		}

	}

	return nil
}
