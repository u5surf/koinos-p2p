package node

import (
	"context"
	crand "crypto/rand"
	"fmt"
	"io"
	"log"
	mrand "math/rand"
	"time"

	koinosmq "github.com/koinos/koinos-mq-golang"
	"github.com/koinos/koinos-p2p/internal/protocol"
	"github.com/koinos/koinos-p2p/internal/rpc"
	types "github.com/koinos/koinos-types-golang"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	gorpc "github.com/libp2p/go-libp2p-gorpc"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	multiaddr "github.com/multiformats/go-multiaddr"
)

type KoinosP2POptions struct {
	// Set to true if peer exchange enabled
	EnablePeerExchange bool

	// Peers to initially connect
	InitialPeers []string

	// Peers to directly connect
	DirectPeers []string
}

// KoinosP2PNode is the core object representing
type KoinosP2PNode struct {
	Host        host.Host
	RPC         rpc.RPC
	Gossip      *protocol.KoinosGossip
	SyncServer  *gorpc.Server
	SyncManager *protocol.SyncManager

	Options KoinosP2POptions
}

// NewKoinosP2POptions creates a KoinosP2POptions object which controls how p2p works
func NewKoinosP2POptions() *KoinosP2POptions {
	return &KoinosP2POptions{
		EnablePeerExchange: true,
		InitialPeers:       make([]string, 0),
		DirectPeers:        make([]string, 0),
	}
}

// NewKoinosP2PNode creates a libp2p node object listening on the given multiaddress
// uses secio encryption on the wire
// listenAddr is a multiaddress string on which to listen
// seed is the random seed to use for key generation. Use a negative number for a random seed.
func NewKoinosP2PNode(ctx context.Context, listenAddr string, rpc rpc.RPC, seed int64, koptions KoinosP2POptions) (*KoinosP2PNode, error) {
	var r io.Reader
	if seed == 0 {
		r = crand.Reader
	} else {
		r = mrand.New(mrand.NewSource(seed))
	}

	privateKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return nil, err
	}

	options := []libp2p.Option{
		libp2p.ListenAddrStrings(listenAddr),
		libp2p.Identity(privateKey),
	}

	host, err := libp2p.New(ctx, options...)
	if err != nil {
		return nil, err
	}

	node := new(KoinosP2PNode)
	node.Host = host
	node.RPC = rpc
	node.SyncServer = gorpc.NewServer(host, protocol.SyncID)
	err = node.SyncServer.Register(&protocol.SyncService{})
	if err != nil {
		return nil, err
	}

	node.SyncManager = protocol.NewSyncManager(node.Host, node.RPC)
	node.SyncManager.Start()
	node.Options = koptions

	// Create the pubsub gossip
	ps, err := pubsub.NewGossipSub(ctx, node.Host)
	if err != nil {
		return nil, err
	}
	node.Gossip = protocol.NewKoinosGossip(ctx, rpc, ps)

	// Setup handler for RabbitMQ messages
	mq := koinosmq.GetKoinosMQ()
	mq.SetBroadcastHandler("koinos.block.accept", node.mqBroadcastHandler)
	mq.SetBroadcastHandler("koinos.transaction.accept", node.mqBroadcastHandler)

	err = node.connectInitialPeers()
	if err != nil {
		return nil, err
	}

	return node, nil
}

func (n *KoinosP2PNode) mqBroadcastHandler(topic string, data []byte) {
	vb := types.VariableBlob(data)
	switch topic {
	case "koinos.block.accept":
		n.Gossip.Block.PublishMessage(context.Background(), &vb)

	case "koinos.transaction.accept":
		n.Gossip.Transaction.PublishMessage(context.Background(), &vb)
	}
}

func getChannelError(errs chan error) error {
	select {
	case err := <-errs:
		return err
	default:
		return nil
	}
}

func (n *KoinosP2PNode) connectInitialPeers() error {
	// TODO: Return errors via channel instead of error
	// TODO: Connect to peers simultaneously instead of sequentially
	// TODO: Instead of calling InitiateProtocol() here, register a notify handler using host.Network().Notify(n),
	//       then initiate the sync protocol in the notify handler's Connected() message.
	//       See e.g. go-libp2p-pubsub newPeers for the way, it also contains a manager-like processLoop() function.
	//
	// Connect to a peer
	for _, pid := range n.Options.InitialPeers {
		if pid != "" {
			log.Printf("Connecting to peer %s and sending broadcast\n", pid)
			_, err := n.ConnectToPeer(pid)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ConnectToPeer connects the node to the given peer
func (n *KoinosP2PNode) ConnectToPeer(peerAddr string) (*peer.AddrInfo, error) {
	addr, err := multiaddr.NewMultiaddr(peerAddr)
	if err != nil {
		return nil, err
	}
	peer, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := n.Host.Connect(ctx, *peer); err != nil {
		return nil, err
	}

	err = n.SyncManager.AddPeer(peer.ID)
	if err != nil {
		return nil, err
	}

	return peer, nil
}

// MakeContext creates and returns the canonical context which should be used for peer connections
// TODO: create this from configuration
func (n *KoinosP2PNode) MakeContext() (ctx context.Context, cancel context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// GetListenAddress returns the multiaddress on which the node is listening
func (n *KoinosP2PNode) GetListenAddress() multiaddr.Multiaddr {
	return n.Host.Addrs()[0]
}

// GetPeerAddress returns the ipfs multiaddress to which other peers should connect
func (n *KoinosP2PNode) GetPeerAddress() multiaddr.Multiaddr {
	hostAddr, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/ipfs/%s", n.Host.ID().Pretty()))
	return n.GetListenAddress().Encapsulate(hostAddr)
}

// Close closes the node
func (n *KoinosP2PNode) Close() error {
	if err := n.Host.Close(); err != nil {
		return err
	}

	return nil
}
