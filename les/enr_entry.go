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

package les

import (
	"github.com/binacoin-official/go-binacoin/core/forkid"
	"github.com/binacoin-official/go-binacoin/p2p/dnsdisc"
	"github.com/binacoin-official/go-binacoin/p2p/enode"
	"github.com/binacoin-official/go-binacoin/rlp"
)

// lesEntry is the "les" ENR entry. This is set for LES servers only.
type lesEntry struct {
	// Ignore additional fields (for forward compatibility).
	VfxVersion uint
	Rest       []rlp.RawValue `rlp:"tail"`
}

func (lesEntry) ENRKey() string { return "les" }

// bnaEntry is the "bna" ENR entry. This is redeclared here to avoid depending on package bna.
type bnaEntry struct {
	ForkID forkid.ID
	Tail   []rlp.RawValue `rlp:"tail"`
}

func (bnaEntry) ENRKey() string { return "bna" }

// setupDiscovery creates the node discovery source for the bna protocol.
func (bna *LightBinacoin) setupDiscovery() (enode.Iterator, error) {
	it := enode.NewFairMix(0)

	// Enable DNS discovery.
	if len(bna.config.BnaDiscoveryURLs) != 0 {
		client := dnsdisc.NewClient(dnsdisc.Config{})
		dns, err := client.NewIterator(bna.config.BnaDiscoveryURLs...)
		if err != nil {
			return nil, err
		}
		it.AddSource(dns)
	}

	// Enable DHT.
	if bna.udpEnabled {
		it.AddSource(bna.p2pServer.DiscV5.RandomNodes())
	}

	forkFilter := forkid.NewFilter(bna.blockchain)
	iterator := enode.Filter(it, func(n *enode.Node) bool { return nodeIsServer(forkFilter, n) })
	return iterator, nil
}

// nodeIsServer checks whether n is an LES server node.
func nodeIsServer(forkFilter forkid.Filter, n *enode.Node) bool {
	var les lesEntry
	var bna bnaEntry
	return n.Load(&les) == nil && n.Load(&bna) == nil && forkFilter(bna.ForkID) == nil
}
