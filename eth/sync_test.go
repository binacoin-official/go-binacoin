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
	"sync/atomic"
	"testing"
	"time"

	"github.com/binacoin-official/go-binacoin/bna/downloader"
	"github.com/binacoin-official/go-binacoin/bna/protocols/bna"
	"github.com/binacoin-official/go-binacoin/p2p"
	"github.com/binacoin-official/go-binacoin/p2p/enode"
)

// Tests that fast sync is disabled after a successful sync cycle.
func TestFastSyncDisabling65(t *testing.T) { testFastSyncDisabling(t, bna.BNA65) }
func TestFastSyncDisabling66(t *testing.T) { testFastSyncDisabling(t, bna.BNA66) }

// Tests that fast sync gets disabled as soon as a real block is successfully
// imported into the blockchain.
func testFastSyncDisabling(t *testing.T, protocol uint) {
	t.Parallel()

	// Create an empty handler and ensure it's in fast sync mode
	empty := newTestHandler()
	if atomic.LoadUint32(&empty.handler.fastSync) == 0 {
		t.Fatalf("fast sync disabled on pristine blockchain")
	}
	defer empty.close()

	// Create a full handler and ensure fast sync ends up disabled
	full := newTestHandlerWithBlocks(1024)
	if atomic.LoadUint32(&full.handler.fastSync) == 1 {
		t.Fatalf("fast sync not disabled on non-empty blockchain")
	}
	defer full.close()

	// Sync up the two handlers
	emptyPipe, fullPipe := p2p.MsgPipe()
	defer emptyPipe.Close()
	defer fullPipe.Close()

	emptyPeer := bna.NewPeer(protocol, p2p.NewPeer(enode.ID{1}, "", nil), emptyPipe, empty.txpool)
	fullPeer := bna.NewPeer(protocol, p2p.NewPeer(enode.ID{2}, "", nil), fullPipe, full.txpool)
	defer emptyPeer.Close()
	defer fullPeer.Close()

	go empty.handler.runBnaPeer(emptyPeer, func(peer *bna.Peer) error {
		return bna.Handle((*bnaHandler)(empty.handler), peer)
	})
	go full.handler.runBnaPeer(fullPeer, func(peer *bna.Peer) error {
		return bna.Handle((*bnaHandler)(full.handler), peer)
	})
	// Wait a bit for the above handlers to start
	time.Sleep(250 * time.Millisecond)

	// Check that fast sync was disabled
	op := peerToSyncOp(downloader.FastSync, empty.handler.peers.peerWithHighestTD())
	if err := empty.handler.doSync(op); err != nil {
		t.Fatal("sync failed:", err)
	}
	if atomic.LoadUint32(&empty.handler.fastSync) == 1 {
		t.Fatalf("fast sync not disabled after successful synchronisation")
	}
}
