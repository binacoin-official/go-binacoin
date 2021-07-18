// Copyright 2015 The go-binacoin Authors
// This file is part of the go-binacoin library.
//
// The go-binacoin library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-binacoin library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-binacoin library. If not, see <http://www.gnu.org/licenses/>.

package bna

import (
	"context"
	"errors"
	"math/big"

	"github.com/binacoin-official/go-binacoin/accounts"
	"github.com/binacoin-official/go-binacoin/common"
	"github.com/binacoin-official/go-binacoin/consensus"
	"github.com/binacoin-official/go-binacoin/core"
	"github.com/binacoin-official/go-binacoin/core/bloombits"
	"github.com/binacoin-official/go-binacoin/core/rawdb"
	"github.com/binacoin-official/go-binacoin/core/state"
	"github.com/binacoin-official/go-binacoin/core/types"
	"github.com/binacoin-official/go-binacoin/core/vm"
	"github.com/binacoin-official/go-binacoin/bna/downloader"
	"github.com/binacoin-official/go-binacoin/bna/gasprice"
	"github.com/binacoin-official/go-binacoin/bnadb"
	"github.com/binacoin-official/go-binacoin/event"
	"github.com/binacoin-official/go-binacoin/miner"
	"github.com/binacoin-official/go-binacoin/params"
	"github.com/binacoin-official/go-binacoin/rpc"
)

// BnaAPIBackend implements bnaapi.Backend for full nodes
type BnaAPIBackend struct {
	extRPCEnabled       bool
	allowUnprotectedTxs bool
	bna                 *Binacoin
	gpo                 *gasprice.Oracle
}

// ChainConfig returns the active chain configuration.
func (b *BnaAPIBackend) ChainConfig() *params.ChainConfig {
	return b.bna.blockchain.Config()
}

func (b *BnaAPIBackend) CurrentBlock() *types.Block {
	return b.bna.blockchain.CurrentBlock()
}

func (b *BnaAPIBackend) SetHead(number uint64) {
	b.bna.handler.downloader.Cancel()
	b.bna.blockchain.SetHead(number)
}

func (b *BnaAPIBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		block := b.bna.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.bna.blockchain.CurrentBlock().Header(), nil
	}
	return b.bna.blockchain.GetHeaderByNumber(uint64(number)), nil
}

func (b *BnaAPIBackend) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.HeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.bna.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.bna.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		return header, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *BnaAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.bna.blockchain.GetHeaderByHash(hash), nil
}

func (b *BnaAPIBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		block := b.bna.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.bna.blockchain.CurrentBlock(), nil
	}
	return b.bna.blockchain.GetBlockByNumber(uint64(number)), nil
}

func (b *BnaAPIBackend) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.bna.blockchain.GetBlockByHash(hash), nil
}

func (b *BnaAPIBackend) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.BlockByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.bna.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.bna.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		block := b.bna.blockchain.GetBlock(hash, header.Number.Uint64())
		if block == nil {
			return nil, errors.New("header found, but block body is missing")
		}
		return block, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *BnaAPIBackend) PendingBlockAndReceipts() (*types.Block, types.Receipts) {
	return b.bna.miner.PendingBlockAndReceipts()
}

func (b *BnaAPIBackend) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if number == rpc.PendingBlockNumber {
		block, state := b.bna.miner.Pending()
		return state, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, number)
	if err != nil {
		return nil, nil, err
	}
	if header == nil {
		return nil, nil, errors.New("header not found")
	}
	stateDb, err := b.bna.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}

func (b *BnaAPIBackend) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.StateAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header, err := b.HeaderByHash(ctx, hash)
		if err != nil {
			return nil, nil, err
		}
		if header == nil {
			return nil, nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.bna.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, nil, errors.New("hash is not currently canonical")
		}
		stateDb, err := b.bna.BlockChain().StateAt(header.Root)
		return stateDb, header, err
	}
	return nil, nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *BnaAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.bna.blockchain.GetReceiptsByHash(hash), nil
}

func (b *BnaAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	receipts := b.bna.blockchain.GetReceiptsByHash(hash)
	if receipts == nil {
		return nil, nil
	}
	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (b *BnaAPIBackend) GetTd(ctx context.Context, hash common.Hash) *big.Int {
	return b.bna.blockchain.GetTdByHash(hash)
}

func (b *BnaAPIBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header, vmConfig *vm.Config) (*vm.EVM, func() error, error) {
	vmError := func() error { return nil }
	if vmConfig == nil {
		vmConfig = b.bna.blockchain.GetVMConfig()
	}
	txContext := core.NewEVMTxContext(msg)
	context := core.NewEVMBlockContext(header, b.bna.BlockChain(), nil)
	return vm.NewEVM(context, txContext, state, b.bna.blockchain.Config(), *vmConfig), vmError, nil
}

func (b *BnaAPIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.bna.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *BnaAPIBackend) SubscribePendingLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.bna.miner.SubscribePendingLogs(ch)
}

func (b *BnaAPIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.bna.BlockChain().SubscribeChainEvent(ch)
}

func (b *BnaAPIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.bna.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *BnaAPIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.bna.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *BnaAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.bna.BlockChain().SubscribeLogsEvent(ch)
}

func (b *BnaAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.bna.txPool.AddLocal(signedTx)
}

func (b *BnaAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.bna.txPool.Pending(false)
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *BnaAPIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.bna.txPool.Get(hash)
}

func (b *BnaAPIBackend) GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(b.bna.ChainDb(), txHash)
	return tx, blockHash, blockNumber, index, nil
}

func (b *BnaAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.bna.txPool.Nonce(addr), nil
}

func (b *BnaAPIBackend) Stats() (pending int, queued int) {
	return b.bna.txPool.Stats()
}

func (b *BnaAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.bna.TxPool().Content()
}

func (b *BnaAPIBackend) TxPoolContentFrom(addr common.Address) (types.Transactions, types.Transactions) {
	return b.bna.TxPool().ContentFrom(addr)
}

func (b *BnaAPIBackend) TxPool() *core.TxPool {
	return b.bna.TxPool()
}

func (b *BnaAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.bna.TxPool().SubscribeNewTxsEvent(ch)
}

func (b *BnaAPIBackend) Downloader() *downloader.Downloader {
	return b.bna.Downloader()
}

func (b *BnaAPIBackend) SuggestGasTipCap(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestTipCap(ctx)
}

func (b *BnaAPIBackend) FeeHistory(ctx context.Context, blockCount int, lastBlock rpc.BlockNumber, rewardPercentiles []float64) (firstBlock rpc.BlockNumber, reward [][]*big.Int, baseFee []*big.Int, gasUsedRatio []float64, err error) {
	return b.gpo.FeeHistory(ctx, blockCount, lastBlock, rewardPercentiles)
}

func (b *BnaAPIBackend) ChainDb() bnadb.Database {
	return b.bna.ChainDb()
}

func (b *BnaAPIBackend) EventMux() *event.TypeMux {
	return b.bna.EventMux()
}

func (b *BnaAPIBackend) AccountManager() *accounts.Manager {
	return b.bna.AccountManager()
}

func (b *BnaAPIBackend) ExtRPCEnabled() bool {
	return b.extRPCEnabled
}

func (b *BnaAPIBackend) UnprotectedAllowed() bool {
	return b.allowUnprotectedTxs
}

func (b *BnaAPIBackend) RPCGasCap() uint64 {
	return b.bna.config.RPCGasCap
}

func (b *BnaAPIBackend) RPCTxFeeCap() float64 {
	return b.bna.config.RPCTxFeeCap
}

func (b *BnaAPIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.bna.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}

func (b *BnaAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.bna.bloomRequests)
	}
}

func (b *BnaAPIBackend) Engine() consensus.Engine {
	return b.bna.engine
}

func (b *BnaAPIBackend) CurrentHeader() *types.Header {
	return b.bna.blockchain.CurrentHeader()
}

func (b *BnaAPIBackend) Miner() *miner.Miner {
	return b.bna.Miner()
}

func (b *BnaAPIBackend) StartMining(threads int) error {
	return b.bna.StartMining(threads)
}

func (b *BnaAPIBackend) StateAtBlock(ctx context.Context, block *types.Block, reexec uint64, base *state.StateDB, checkLive bool) (*state.StateDB, error) {
	return b.bna.stateAtBlock(block, reexec, base, checkLive)
}

func (b *BnaAPIBackend) StateAtTransaction(ctx context.Context, block *types.Block, txIndex int, reexec uint64) (core.Message, vm.BlockContext, *state.StateDB, error) {
	return b.bna.stateAtTransaction(block, txIndex, reexec)
}
