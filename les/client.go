// Copyright 2016 The go-binacoin Authors
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

// Package les implements the Light Binacoin Subprotocol.
package les

import (
	"fmt"
	"time"

	"github.com/binacoin-official/go-binacoin/accounts"
	"github.com/binacoin-official/go-binacoin/common"
	"github.com/binacoin-official/go-binacoin/common/hexutil"
	"github.com/binacoin-official/go-binacoin/common/mclock"
	"github.com/binacoin-official/go-binacoin/consensus"
	"github.com/binacoin-official/go-binacoin/core"
	"github.com/binacoin-official/go-binacoin/core/bloombits"
	"github.com/binacoin-official/go-binacoin/core/rawdb"
	"github.com/binacoin-official/go-binacoin/core/types"
	"github.com/binacoin-official/go-binacoin/bna/downloader"
	"github.com/binacoin-official/go-binacoin/bna/bnaconfig"
	"github.com/binacoin-official/go-binacoin/bna/filters"
	"github.com/binacoin-official/go-binacoin/bna/gasprice"
	"github.com/binacoin-official/go-binacoin/event"
	"github.com/binacoin-official/go-binacoin/internal/bnaapi"
	"github.com/binacoin-official/go-binacoin/les/vflux"
	vfc "github.com/binacoin-official/go-binacoin/les/vflux/client"
	"github.com/binacoin-official/go-binacoin/light"
	"github.com/binacoin-official/go-binacoin/log"
	"github.com/binacoin-official/go-binacoin/node"
	"github.com/binacoin-official/go-binacoin/p2p"
	"github.com/binacoin-official/go-binacoin/p2p/enode"
	"github.com/binacoin-official/go-binacoin/p2p/enr"
	"github.com/binacoin-official/go-binacoin/params"
	"github.com/binacoin-official/go-binacoin/rlp"
	"github.com/binacoin-official/go-binacoin/rpc"
)

type LightBinacoin struct {
	lesCommons

	peers              *serverPeerSet
	reqDist            *requestDistributor
	retriever          *retrieveManager
	odr                *LesOdr
	relay              *lesTxRelay
	handler            *clientHandler
	txPool             *light.TxPool
	blockchain         *light.LightChain
	serverPool         *vfc.ServerPool
	serverPoolIterator enode.Iterator
	pruner             *pruner

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	ApiBackend     *LesApiBackend
	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager
	netRPCService  *bnaapi.PublicNetAPI

	p2pServer  *p2p.Server
	p2pConfig  *p2p.Config
	udpEnabled bool
}

// New creates an instance of the light client.
func New(stack *node.Node, config *bnaconfig.Config) (*LightBinacoin, error) {
	chainDb, err := stack.OpenDatabase("lightchaindata", config.DatabaseCache, config.DatabaseHandles, "bna/db/chaindata/", false)
	if err != nil {
		return nil, err
	}
	lesDb, err := stack.OpenDatabase("les.client", 0, 0, "bna/db/lesclient/", false)
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlockWithOverride(chainDb, config.Genesis, config.OverrideLondon)
	if _, isCompat := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !isCompat {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	peers := newServerPeerSet()
	lbna := &LightBinacoin{
		lesCommons: lesCommons{
			genesis:     genesisHash,
			config:      config,
			chainConfig: chainConfig,
			iConfig:     light.DefaultClientIndexerConfig,
			chainDb:     chainDb,
			lesDb:       lesDb,
			closeCh:     make(chan struct{}),
		},
		peers:          peers,
		eventMux:       stack.EventMux(),
		reqDist:        newRequestDistributor(peers, &mclock.System{}),
		accountManager: stack.AccountManager(),
		engine:         bnaconfig.CreateConsensusEngine(stack, chainConfig, &config.Ethash, nil, false, chainDb),
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   core.NewBloomIndexer(chainDb, params.BloomBitsBlocksClient, params.HelperTrieConfirmations),
		p2pServer:      stack.Server(),
		p2pConfig:      &stack.Config().P2P,
		udpEnabled:     stack.Config().P2P.DiscoveryV5,
	}

	var prenegQuery vfc.QueryFunc
	if lbna.udpEnabled {
		prenegQuery = lbna.prenegQuery
	}
	lbna.serverPool, lbna.serverPoolIterator = vfc.NewServerPool(lesDb, []byte("serverpool:"), time.Second, prenegQuery, &mclock.System{}, config.UltraLightServers, requestList)
	lbna.serverPool.AddMetrics(suggestedTimeoutGauge, totalValueGauge, serverSelectableGauge, serverConnectedGauge, sessionValueMeter, serverDialedMeter)

	lbna.retriever = newRetrieveManager(peers, lbna.reqDist, lbna.serverPool.GetTimeout)
	lbna.relay = newLesTxRelay(peers, lbna.retriever)

	lbna.odr = NewLesOdr(chainDb, light.DefaultClientIndexerConfig, lbna.peers, lbna.retriever)
	lbna.chtIndexer = light.NewChtIndexer(chainDb, lbna.odr, params.CHTFrequency, params.HelperTrieConfirmations, config.LightNoPrune)
	lbna.bloomTrieIndexer = light.NewBloomTrieIndexer(chainDb, lbna.odr, params.BloomBitsBlocksClient, params.BloomTrieFrequency, config.LightNoPrune)
	lbna.odr.SetIndexers(lbna.chtIndexer, lbna.bloomTrieIndexer, lbna.bloomIndexer)

	checkpoint := config.Checkpoint
	if checkpoint == nil {
		checkpoint = params.TrustedCheckpoints[genesisHash]
	}
	// Note: NewLightChain adds the trusted checkpoint so it needs an ODR with
	// indexers already set but not started yet
	if lbna.blockchain, err = light.NewLightChain(lbna.odr, lbna.chainConfig, lbna.engine, checkpoint); err != nil {
		return nil, err
	}
	lbna.chainReader = lbna.blockchain
	lbna.txPool = light.NewTxPool(lbna.chainConfig, lbna.blockchain, lbna.relay)

	// Set up checkpoint oracle.
	lbna.oracle = lbna.setupOracle(stack, genesisHash, config)

	// Note: AddChildIndexer starts the update process for the child
	lbna.bloomIndexer.AddChildIndexer(lbna.bloomTrieIndexer)
	lbna.chtIndexer.Start(lbna.blockchain)
	lbna.bloomIndexer.Start(lbna.blockchain)

	// Start a light chain pruner to delete useless historical data.
	lbna.pruner = newPruner(chainDb, lbna.chtIndexer, leth.bloomTrieIndexer)

	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		lbna.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}

	lbna.ApiBackend = &LesApiBackend{stack.Config().ExtRPCEnabled(), stack.Config().AllowUnprotectedTxs, lbna, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.Miner.GasPrice
	}
	lbna.ApiBackend.gpo = gasprice.NewOracle(lbna.ApiBackend, gpoParams)

	lbna.handler = newClientHandler(config.UltraLightServers, config.UltraLightFraction, checkpoint, lbna)
	if lbna.handler.ulc != nil {
		log.Warn("Ultra light client is enabled", "trustedNodes", len(lbna.handler.ulc.keys), "minTrustedFraction", lbna.handler.ulc.fraction)
		lbna.blockchain.DisableCheckFreq()
	}

	lbna.netRPCService = bnaapi.NewPublicNetAPI(lbna.p2pServer, lbna.config.NetworkId)

	// Register the backend on the node
	stack.RegisterAPIs(lbna.APIs())
	stack.RegisterProtocols(lbna.Protocols())
	stack.RegisterLifecycle(lbna)

	// Check for unclean shutdown
	if uncleanShutdowns, discards, err := rawdb.PushUncleanShutdownMarker(chainDb); err != nil {
		log.Error("Could not update unclean-shutdown-marker list", "error", err)
	} else {
		if discards > 0 {
			log.Warn("Old unclean shutdowns found", "count", discards)
		}
		for _, tstamp := range uncleanShutdowns {
			t := time.Unix(int64(tstamp), 0)
			log.Warn("Unclean shutdown detected", "booted", t,
				"age", common.PrettyAge(t))
		}
	}
	return lbna, nil
}

// VfluxRequest sends a batch of requests to the given node through discv5 UDP TalkRequest and returns the responses
func (s *LightBinacoin) VfluxRequest(n *enode.Node, reqs vflux.Requests) vflux.Replies {
	if !s.udpEnabled {
		return nil
	}
	reqsEnc, _ := rlp.EncodeToBytes(&reqs)
	repliesEnc, _ := s.p2pServer.DiscV5.TalkRequest(s.serverPool.DialNode(n), "vfx", reqsEnc)
	var replies vflux.Replies
	if len(repliesEnc) == 0 || rlp.DecodeBytes(repliesEnc, &replies) != nil {
		return nil
	}
	return replies
}

// vfxVersion returns the version number of the "les" service subdomain of the vflux UDP
// service, as advertised in the ENR record
func (s *LightBinacoin) vfxVersion(n *enode.Node) uint {
	if n.Seq() == 0 {
		var err error
		if !s.udpEnabled {
			return 0
		}
		if n, err = s.p2pServer.DiscV5.RequestENR(n); n != nil && err == nil && n.Seq() != 0 {
			s.serverPool.Persist(n)
		} else {
			return 0
		}
	}

	var les []rlp.RawValue
	if err := n.Load(enr.WithEntry("les", &les)); err != nil || len(les) < 1 {
		return 0
	}
	var version uint
	rlp.DecodeBytes(les[0], &version) // Ignore additional fields (for forward compatibility).
	return version
}

// prenegQuery sends a capacity query to the given server node to determine whether
// a connection slot is immediately available
func (s *LightBinacoin) prenegQuery(n *enode.Node) int {
	if s.vfxVersion(n) < 1 {
		// UDP query not supported, always try TCP connection
		return 1
	}

	var requests vflux.Requests
	requests.Add("les", vflux.CapacityQueryName, vflux.CapacityQueryReq{
		Bias:      180,
		AddTokens: []vflux.IntOrInf{{}},
	})
	replies := s.VfluxRequest(n, requests)
	var cqr vflux.CapacityQueryReply
	if replies.Get(0, &cqr) != nil || len(cqr) != 1 { // Note: Get returns an error if replies is nil
		return -1
	}
	if cqr[0] > 0 {
		return 1
	}
	return 0
}

type LightDummyAPI struct{}

// Bitherbase is the address that mining rewards will be send to
func (s *LightDummyAPI) Bitherbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("mining is not supported in light mode")
}

// Coinbase is the address that mining rewards will be send to (alias for Bitherbase)
func (s *LightDummyAPI) Coinbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("mining is not supported in light mode")
}

// Hashrate returns the POW hashrate
func (s *LightDummyAPI) Hashrate() hexutil.Uint {
	return 0
}

// Mining returns an indication if this node is currently mining.
func (s *LightDummyAPI) Mining() bool {
	return false
}

// APIs returns the collection of RPC services the binacoin package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *LightBinacoin) APIs() []rpc.API {
	apis := bnaapi.GetAPIs(s.ApiBackend)
	apis = append(apis, s.engine.APIs(s.BlockChain().HeaderChain())...)
	return append(apis, []rpc.API{
		{
			Namespace: "bna",
			Version:   "1.0",
			Service:   &LightDummyAPI{},
			Public:    true,
		}, {
			Namespace: "bna",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.handler.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "bna",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.ApiBackend, true, 5*time.Minute),
			Public:    true,
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		}, {
			Namespace: "les",
			Version:   "1.0",
			Service:   NewPrivateLightAPI(&s.lesCommons),
			Public:    false,
		}, {
			Namespace: "vflux",
			Version:   "1.0",
			Service:   s.serverPool.API(),
			Public:    false,
		},
	}...)
}

func (s *LightBinacoin) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *LightBinacoin) BlockChain() *light.LightChain      { return s.blockchain }
func (s *LightBinacoin) TxPool() *light.TxPool              { return s.txPool }
func (s *LightBinacoin) Engine() consensus.Engine           { return s.engine }
func (s *LightBinacoin) LesVersion() int                    { return int(ClientProtocolVersions[0]) }
func (s *LightBinacoin) Downloader() *downloader.Downloader { return s.handler.downloader }
func (s *LightBinacoin) EventMux() *event.TypeMux           { return s.eventMux }

// Protocols returns all the currently configured network protocols to start.
func (s *LightBinacoin) Protocols() []p2p.Protocol {
	return s.makeProtocols(ClientProtocolVersions, s.handler.runPeer, func(id enode.ID) interface{} {
		if p := s.peers.peer(id.String()); p != nil {
			return p.Info()
		}
		return nil
	}, s.serverPoolIterator)
}

// Start implements node.Lifecycle, starting all internal goroutines needed by the
// light binacoin protocol implementation.
func (s *LightBinacoin) Start() error {
	log.Warn("Light client mode is an experimental feature")

	if s.udpEnabled && s.p2pServer.DiscV5 == nil {
		s.udpEnabled = false
		log.Error("Discovery v5 is not initialized")
	}
	discovery, err := s.setupDiscovery()
	if err != nil {
		return err
	}
	s.serverPool.AddSource(discovery)
	s.serverPool.Start()
	// Start bloom request workers.
	s.wg.Add(bloomServiceThreads)
	s.startBloomHandlers(params.BloomBitsBlocksClient)
	s.handler.start()

	return nil
}

// Stop implements node.Lifecycle, terminating all internal goroutines used by the
// Binacoin protocol.
func (s *LightBinacoin) Stop() error {
	close(s.closeCh)
	s.serverPool.Stop()
	s.peers.close()
	s.reqDist.close()
	s.odr.Stop()
	s.relay.Stop()
	s.bloomIndexer.Close()
	s.chtIndexer.Close()
	s.blockchain.Stop()
	s.handler.stop()
	s.txPool.Stop()
	s.engine.Close()
	s.pruner.close()
	s.eventMux.Stop()
	rawdb.PopUncleanShutdownMarker(s.chainDb)
	s.chainDb.Close()
	s.lesDb.Close()
	s.wg.Wait()
	log.Info("Light binacoin stopped")
	return nil
}
