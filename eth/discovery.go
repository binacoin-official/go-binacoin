// Copyright 2019 The go-binacoin Authors
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
	"github.com/binacoin-official/go-binacoin/core"
	"github.com/binacoin-official/go-binacoin/core/forkid"
	"github.com/binacoin-official/go-binacoin/p2p/enode"
	"github.com/binacoin-official/go-binacoin/rlp"
)

// bnaEntry is the "bna" ENR entry which advertises bna protocol
// on the discovery network.
type bnaEntry struct {
	ForkID forkid.ID // Fork identifier per EIP-2124

	// Ignore additional fields (for forward compatibility).
	Rest []rlp.RawValue `rlp:"tail"`
}

// ENRKey implements enr.Entry.
func (e bnaEntry) ENRKey() string {
	return "bna"
}

// startBnaEntryUpdate starts the ENR updater loop.
func (bna *Binacoin) startBnaEntryUpdate(ln *enode.LocalNode) {
	var newHead = make(chan core.ChainHeadEvent, 10)
	sub := bna.blockchain.SubscribeChainHeadEvent(newHead)

	go func() {
		defer sub.Unsubscribe()
		for {
			select {
			case <-newHead:
				ln.Set(bna.currentBnaEntry())
			case <-sub.Err():
				// Would be nice to sync with bna.Stop, but there is no
				// good way to do that.
				return
			}
		}
	}()
}

func (bna *Binacoin) currentBnaEntry() *bnaEntry {
	return &bnaEntry{ForkID: forkid.NewID(bna.blockchain.Config(), bna.blockchain.Genesis().Hash(),
		bna.blockchain.CurrentHeader().Number.Uint64())}
}
