// Copyright 2020 The go-binacoin Authors
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

package catalyst

import (
	"math/big"
	"testing"

	"github.com/binacoin-official/go-binacoin/consensus/ethash"
	"github.com/binacoin-official/go-binacoin/core"
	"github.com/binacoin-official/go-binacoin/core/rawdb"
	"github.com/binacoin-official/go-binacoin/core/types"
	"github.com/binacoin-official/go-binacoin/crypto"
	"github.com/binacoin-official/go-binacoin/bna"
	"github.com/binacoin-official/go-binacoin/bna/bnaconfig"
	"github.com/binacoin-official/go-binacoin/node"
	"github.com/binacoin-official/go-binacoin/params"
)

var (
	// testKey is a private key to use for funding a tester account.
	testKey, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")

	// testAddr is the Binacoin address of the tester account.
	testAddr = crypto.PubkeyToAddress(testKey.PublicKey)

	testBalance = big.NewInt(2e15)
)

func generateTestChain() (*core.Genesis, []*types.Block) {
	db := rawdb.NewMemoryDatabase()
	config := params.AllEthashProtocolChanges
	genesis := &core.Genesis{
		Config:    config,
		Alloc:     core.GenesisAlloc{testAddr: {Balance: testBalance}},
		ExtraData: []byte("test genesis"),
		Timestamp: 9000,
		BaseFee:   big.NewInt(params.InitialBaseFee),
	}
	generate := func(i int, g *core.BlockGen) {
		g.OffsetTime(5)
		g.SetExtra([]byte("test"))
	}
	gblock := genesis.ToBlock(db)
	engine := ethash.NewFaker()
	blocks, _ := core.GenerateChain(config, gblock, engine, db, 10, generate)
	blocks = append([]*types.Block{gblock}, blocks...)
	return genesis, blocks
}

func generateTestChainWithFork(n int, fork int) (*core.Genesis, []*types.Block, []*types.Block) {
	if fork >= n {
		fork = n - 1
	}
	db := rawdb.NewMemoryDatabase()
	config := &params.ChainConfig{
		ChainID:             big.NewInt(1337),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		MuirGlacierBlock:    big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
		CatalystBlock:       big.NewInt(0),
		Ethash:              new(params.EthashConfig),
	}
	genesis := &core.Genesis{
		Config:    config,
		Alloc:     core.GenesisAlloc{testAddr: {Balance: testBalance}},
		ExtraData: []byte("test genesis"),
		Timestamp: 9000,
		BaseFee:   big.NewInt(params.InitialBaseFee),
	}
	generate := func(i int, g *core.BlockGen) {
		g.OffsetTime(5)
		g.SetExtra([]byte("test"))
	}
	generateFork := func(i int, g *core.BlockGen) {
		g.OffsetTime(5)
		g.SetExtra([]byte("testF"))
	}
	gblock := genesis.ToBlock(db)
	engine := bnaash.NewFaker()
	blocks, _ := core.GenerateChain(config, gblock, engine, db, n, generate)
	blocks = append([]*types.Block{gblock}, blocks...)
	forkedBlocks, _ := core.GenerateChain(config, blocks[fork], engine, db, n-fork, generateFork)
	return genesis, blocks, forkedBlocks
}

func TestBna2AssembleBlock(t *testing.T) {
	genesis, blocks := generateTestChain()
	n, bnaservice := startBnaService(t, genesis, blocks[1:9])
	defer n.Close()

	api := newConsensusAPI(bnaservice)
	signer := types.NewEIP155Signer(bnaservice.BlockChain().Config().ChainID)
	tx, err := types.SignTx(types.NewTransaction(0, blocks[8].Coinbase(), big.NewInt(1000), params.TxGas, big.NewInt(params.InitialBaseFee), nil), signer, testKey)
	if err != nil {
		t.Fatalf("error signing transaction, err=%v", err)
	}
	bnaservice.TxPool().AddLocal(tx)
	blockParams := assembleBlockParams{
		ParentHash: blocks[8].ParentHash(),
		Timestamp:  blocks[8].Time(),
	}
	execData, err := api.AssembleBlock(blockParams)

	if err != nil {
		t.Fatalf("error producing block, err=%v", err)
	}

	if len(execData.Transactions) != 1 {
		t.Fatalf("invalid number of transactions %d != 1", len(execData.Transactions))
	}
}

func TestBna2AssembleBlockWithAnotherBlocksTxs(t *testing.T) {
	genesis, blocks := generateTestChain()
	n, bnaservice := startBnaService(t, genesis, blocks[1:9])
	defer n.Close()

	api := newConsensusAPI(bnaservice)

	// Put the 10th block's tx in the pool and produce a new block
	api.addBlockTxs(blocks[9])
	blockParams := assembleBlockParams{
		ParentHash: blocks[9].ParentHash(),
		Timestamp:  blocks[9].Time(),
	}
	execData, err := api.AssembleBlock(blockParams)
	if err != nil {
		t.Fatalf("error producing block, err=%v", err)
	}

	if len(execData.Transactions) != blocks[9].Transactions().Len() {
		t.Fatalf("invalid number of transactions %d != 1", len(execData.Transactions))
	}
}

func TestBna2NewBlock(t *testing.T) {
	genesis, blocks, forkedBlocks := generateTestChainWithFork(10, 4)
	n, bnaservice := startBnaService(t, genesis, blocks[1:5])
	defer n.Close()

	api := newConsensusAPI(bnaservice)
	for i := 5; i < 10; i++ {
		p := executableData{
			ParentHash:   bnaservice.BlockChain().CurrentBlock().Hash(),
			Miner:        blocks[i].Coinbase(),
			StateRoot:    blocks[i].Root(),
			GasLimit:     blocks[i].GasLimit(),
			GasUsed:      blocks[i].GasUsed(),
			Transactions: encodeTransactions(blocks[i].Transactions()),
			ReceiptRoot:  blocks[i].ReceiptHash(),
			LogsBloom:    blocks[i].Bloom().Bytes(),
			BlockHash:    blocks[i].Hash(),
			Timestamp:    blocks[i].Time(),
			Number:       uint64(i),
		}
		success, err := api.NewBlock(p)
		if err != nil || !success.Valid {
			t.Fatalf("Failed to insert block: %v", err)
		}
	}

	exp := bnaservice.BlockChain().CurrentBlock().Hash()

	// Introduce the fork point.
	lastBlockNum := blocks[4].Number()
	lastBlock := blocks[4]
	for i := 0; i < 4; i++ {
		lastBlockNum.Add(lastBlockNum, big.NewInt(1))
		p := executableData{
			ParentHash:   lastBlock.Hash(),
			Miner:        forkedBlocks[i].Coinbase(),
			StateRoot:    forkedBlocks[i].Root(),
			Number:       lastBlockNum.Uint64(),
			GasLimit:     forkedBlocks[i].GasLimit(),
			GasUsed:      forkedBlocks[i].GasUsed(),
			Transactions: encodeTransactions(blocks[i].Transactions()),
			ReceiptRoot:  forkedBlocks[i].ReceiptHash(),
			LogsBloom:    forkedBlocks[i].Bloom().Bytes(),
			BlockHash:    forkedBlocks[i].Hash(),
			Timestamp:    forkedBlocks[i].Time(),
		}
		success, err := api.NewBlock(p)
		if err != nil || !success.Valid {
			t.Fatalf("Failed to insert forked block #%d: %v", i, err)
		}
		lastBlock, err = insertBlockParamsToBlock(bnaservice.BlockChain().Config(), lastBlock.Header(), p)
		if err != nil {
			t.Fatal(err)
		}
	}

	if bnaservice.BlockChain().CurrentBlock().Hash() != exp {
		t.Fatalf("Wrong head after inserting fork %x != %x", exp, bnaservice.BlockChain().CurrentBlock().Hash())
	}
}

// startBnaService creates a full node instance for testing.
func startBnaService(t *testing.T, genesis *core.Genesis, blocks []*types.Block) (*node.Node, *bna.Binacoin) {
	t.Helper()

	n, err := node.New(&node.Config{})
	if err != nil {
		t.Fatal("can't create node:", err)
	}

	bnacfg := &bnaconfig.Config{Genesis: genesis, Ethash: bnaash.Config{PowMode: bnaash.ModeFake}}
	bnaservice, err := bna.New(n, bnacfg)
	if err != nil {
		t.Fatal("can't create bna service:", err)
	}
	if err := n.Start(); err != nil {
		t.Fatal("can't start node:", err)
	}
	if _, err := bnaservice.BlockChain().InsertChain(blocks); err != nil {
		n.Close()
		t.Fatal("can't import test blocks:", err)
	}
	bnaservice.SetBitherbase(testAddr)

	return n, bnaservice
}
