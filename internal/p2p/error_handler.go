package p2p

import (
	"context"
	"errors"
	"math"
	"time"

	log "github.com/koinos/koinos-log-golang"
	"github.com/koinos/koinos-p2p/internal/options"
	"github.com/koinos/koinos-p2p/internal/p2perrors"
	"github.com/libp2p/go-libp2p-core/control"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	multiaddr "github.com/multiformats/go-multiaddr"
)

// PeerError represents an error originating from a peer
type PeerError struct {
	id  peer.ID
	err error
}

type errorScoreRecord struct {
	lastUpdate time.Time
	score      uint64
}

type canConnectRequest struct {
	id         peer.ID
	resultChan chan<- bool
}

// PeerErrorHandler handles PeerErrors and tracks errors over time
// to determine if a peer should be disconnected from
type PeerErrorHandler struct {
	errorScores        map[peer.ID]*errorScoreRecord
	disconnectPeerChan chan<- peer.ID
	peerErrorChan      <-chan PeerError
	canConnectChan     chan canConnectRequest

	opts options.PeerErrorHandlerOptions
}

// CanConnect to peer if the peer's error score is below the error score threshold
func (p *PeerErrorHandler) CanConnect(ctx context.Context, id peer.ID) bool {
	resultChan := make(chan bool, 1)
	p.canConnectChan <- canConnectRequest{
		id:         id,
		resultChan: resultChan,
	}

	select {
	case res := <-resultChan:
		return res
	case <-ctx.Done():
		return false
	}
}

func (p *PeerErrorHandler) handleCanConnect(id peer.ID) bool {
	if record, ok := p.errorScores[id]; ok {
		p.decayErrorScore(record)
		return record.score < p.opts.ErrorScoreThreshold
	}

	return true
}

func (p *PeerErrorHandler) handleError(ctx context.Context, peerErr PeerError) {
	if record, ok := p.errorScores[peerErr.id]; ok {
		p.decayErrorScore(record)
		record.score += p.getScoreForError(peerErr.err)
	} else {
		p.errorScores[peerErr.id] = &errorScoreRecord{
			lastUpdate: time.Now(),
			score:      p.getScoreForError(peerErr.err),
		}
	}

	log.Infof("Encountered peer error: %s, %s. Current error score: %v", peerErr.id, peerErr.err.Error(), p.errorScores[peerErr.id].score)

	if p.errorScores[peerErr.id].score >= p.opts.ErrorScoreThreshold {
		go func() {
			select {
			case p.disconnectPeerChan <- peerErr.id:
			case <-ctx.Done():
			}
		}()
	}
}

func (p *PeerErrorHandler) getScoreForError(err error) uint64 {
	// These should be ordered from most common error to least
	switch {

	// Errors that are commonly expected during normal use or potential attack vectors
	case errors.Is(err, p2perrors.ErrTransactionApplication):
		return p.opts.TransactionApplicationErrorScore
	case errors.Is(err, p2perrors.ErrBlockApplication):
		return p.opts.BlockApplicationErrorScore
	case errors.Is(err, p2perrors.ErrDeserialization):
		return p.opts.DeserializationErrorScore
	case errors.Is(err, p2perrors.ErrBlockIrreversibility):
		return p.opts.BlockIrreversibilityErrorScore
	case errors.Is(err, p2perrors.ErrPeerRPC):
		return p.opts.PeerRPCErrorScore
	case errors.Is(err, p2perrors.ErrPeerRPCTimeout):
		return p.opts.PeerRPCTimeoutErrorScore

	// These errors are expected, but result in instant disconnection
	case errors.Is(err, p2perrors.ErrChainIDMismatch):
		return p.opts.ChainIDMismatchErrorScore
	case errors.Is(err, p2perrors.ErrChainNotConnected):
		return p.opts.ChainNotConnectedErrorScore
	case errors.Is(err, p2perrors.ErrCheckpointMismatch):
		return p.opts.CheckpointMismatchErrorScore

	// Errors that should only originate from the local process or local node
	case errors.Is(err, p2perrors.ErrLocalRPC):
		return p.opts.LocalRPCErrorScore
	case errors.Is(err, p2perrors.ErrLocalRPCTimeout):
		return p.opts.LocalRPCTimeoutErrorScore
	case errors.Is(err, p2perrors.ErrSerialization):
		return p.opts.SerializationErrorScore
	case errors.Is(err, p2perrors.ErrProcessRequestTimeout):
		return p.opts.ProcessRequestTimeoutErrorScore

	default:
		return p.opts.UnknownErrorScore
	}
}

func (p *PeerErrorHandler) decayErrorScore(record *errorScoreRecord) {
	decayConstant := math.Log(2) / float64(p.opts.ErrorScoreDecayHalflife)
	now := time.Now()
	record.score = uint64(float64(record.score) * math.Exp(-1*decayConstant*float64(now.Sub(record.lastUpdate))))
	record.lastUpdate = now
}

// InterceptPeerDial implements the libp2p ConnectionGater interface
func (p *PeerErrorHandler) InterceptPeerDial(pid peer.ID) bool {
	return p.CanConnect(context.Background(), pid)
}

// InterceptAddrDial implements the libp2p ConnectionGater interface
func (p *PeerErrorHandler) InterceptAddrDial(peer.ID, multiaddr.Multiaddr) bool {
	return true
}

// InterceptAccept implements the libp2p ConnectionGater interface
func (p *PeerErrorHandler) InterceptAccept(network.ConnMultiaddrs) bool {
	return true
}

// InterceptSecured implements the libp2p ConnectionGater interface
func (p *PeerErrorHandler) InterceptSecured(_ network.Direction, pid peer.ID, _ network.ConnMultiaddrs) bool {
	return p.CanConnect(context.Background(), pid)
}

// InterceptUpgraded implements the libp2p ConnectionGater interface
func (p *PeerErrorHandler) InterceptUpgraded(network.Conn) (bool, control.DisconnectReason) {
	return true, 0
}

// Start processing peer errors
func (p *PeerErrorHandler) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case perr := <-p.peerErrorChan:
				p.handleError(ctx, perr)
			case req := <-p.canConnectChan:
				req.resultChan <- p.handleCanConnect(req.id)

			case <-ctx.Done():
				return
			}
		}
	}()
}

// NewPeerErrorHandler creates a new PeerErrorHandler
func NewPeerErrorHandler(disconnectPeerChan chan<- peer.ID, peerErrorChan <-chan PeerError, opts options.PeerErrorHandlerOptions) *PeerErrorHandler {
	return &PeerErrorHandler{
		errorScores:        make(map[peer.ID]*errorScoreRecord),
		disconnectPeerChan: disconnectPeerChan,
		peerErrorChan:      peerErrorChan,
		canConnectChan:     make(chan canConnectRequest),
		opts:               opts,
	}
}
