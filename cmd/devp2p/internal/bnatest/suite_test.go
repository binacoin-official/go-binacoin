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

package bnatest

import (
	"os"
	"testing"
	"time"

	"github.com/binacoin-official/go-binacoin/bna"
	"github.com/binacoin-official/go-binacoin/bna/bnaconfig"
	"github.com/binacoin-official/go-binacoin/internal/utesting"
	"github.com/binacoin-official/go-binacoin/node"
	"github.com/binacoin-official/go-binacoin/p2p"
)

var (
	genesisFile   = "./testdata/genesis.json"
	halfchainFile = "./testdata/halfchain.rlp"
	fullchainFile = "./testdata/chain.rlp"
)

func TestBnaSuite(t *testing.T) {
	gbina, err := runGbina()
	if err != nil {
		t.Fatalf("could not run gbina: %v", err)
	}
	defer gbina.Close()

	suite, err := NewSuite(gbina.Server().Self(), fullchainFile, genesisFile)
	if err != nil {
		t.Fatalf("could not create new test suite: %v", err)
	}
	for _, test := range suite.AllBnaTests() {
		t.Run(test.Name, func(t *testing.T) {
			result := utesting.RunTAP([]utesting.Test{{Name: test.Name, Fn: test.Fn}}, os.Stdout)
			if result[0].Failed {
				t.Fatal()
			}
		})
	}
}

// runGbina creates and starts a gbina node
func runGbina() (*node.Node, error) {
	stack, err := node.New(&node.Config{
		P2P: p2p.Config{
			ListenAddr:  "127.0.0.1:0",
			NoDiscovery: true,
			MaxPeers:    10, // in case a test requires multiple connections, can be changed in the future
			NoDial:      true,
		},
	})
	if err != nil {
		return nil, err
	}

	err = setupGbina(stack)
	if err != nil {
		stack.Close()
		return nil, err
	}
	if err = stack.Start(); err != nil {
		stack.Close()
		return nil, err
	}
	return stack, nil
}

func setupGbina(stack *node.Node) error {
	chain, err := loadChain(halfchainFile, genesisFile)
	if err != nil {
		return err
	}

	backend, err := bna.New(stack, &bnaconfig.Config{
		Genesis:                 &chain.genesis,
		NetworkId:               chain.genesis.Config.ChainID.Uint64(), // 19763
		DatabaseCache:           10,
		TrieCleanCache:          10,
		TrieCleanCacheJournal:   "",
		TrieCleanCacheRejournal: 60 * time.Minute,
		TrieDirtyCache:          16,
		TrieTimeout:             60 * time.Minute,
		SnapshotCache:           10,
	})
	if err != nil {
		return err
	}

	_, err = backend.BlockChain().InsertChain(chain.blocks[1:])
	return err
}
