package rpc

import (
	"context"
	"encoding/hex"
	"errors"

	log "github.com/koinos/koinos-log-golang"
	"github.com/multiformats/go-multihash"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// PeerRPCID Identifies the peer rpc service
const PeerRPCID = "/koinos/peerrpc/1.0.0"

type GetChainIDRequest struct {
}

type GetChainIDResponse struct {
	ID multihash.Multihash
}

type GetHeadBlockRequest struct {
}

type GetHeadBlockResponse struct {
	ID     multihash.Multihash
	Height uint64
}

type GetAncestorBlockIDRequest struct {
	ParentID    *multihash.Multihash
	ChildHeight uint64
}

type GetAncestorBlockIDResponse struct {
	ID multihash.Multihash
}

type GetBlocksRequest struct {
	HeadBlockID      *multihash.Multihash
	StartBlockHeight uint64
	NumBlocks        uint32
}

type GetBlocksResponse struct {
	Blocks [][]byte
}

type PeerRPCService struct {
	local LocalRPC
}

func NewPeerRPCService(local LocalRPC) *PeerRPCService {
	return &PeerRPCService{
		local: local,
	}
}

func (p *PeerRPCService) GetChainID(ctx context.Context, request *GetChainIDRequest, response *GetChainIDResponse) error {
	rpcResult, err := p.local.GetChainID(ctx)
	if err != nil {
		return err
	}

	response.ID = rpcResult.ChainId
	return nil
}

func (p *PeerRPCService) GetHeadBlock(ctx context.Context, request *GetHeadBlockRequest, response *GetHeadBlockResponse) error {
	rpcResult, err := p.local.GetHeadBlock(ctx)
	if err != nil {
		return err
	}

	response.ID = rpcResult.HeadTopology.Id
	response.Height = rpcResult.HeadTopology.Height
	return nil
}

func (p *PeerRPCService) GetAncestorBlockID(ctx context.Context, request *GetAncestorBlockIDRequest, response *GetAncestorBlockIDResponse) error {
	log.Infof("Getting ancestor block parent: %s child_height: %s", request.ParentID.HexString(), request.ChildHeight)
	rpcResult, err := p.local.GetBlocksByHeight(ctx, request.ParentID, request.ChildHeight, 1)
	if err != nil {
		return err
	}

	log.Infof("Result: %s", rpcResult.String())

	if len(rpcResult.BlockItems) != 1 {
		return errors.New("unexpected number of blocks returned")
	}

	response.ID = rpcResult.BlockItems[0].BlockId
	return nil
}

func (p *PeerRPCService) GetBlocks(ctx context.Context, request *GetBlocksRequest, response *GetBlocksResponse) error {
	rpcResult, err := p.local.GetBlocksByHeight(ctx, request.HeadBlockID, request.StartBlockHeight, request.NumBlocks)
	if err != nil {
		return err
	}

	response.Blocks = make([][]byte, len(rpcResult.BlockItems))
	for i, block := range rpcResult.BlockItems {
		json, _ := protojson.Marshal(block)
		log.Info(string(json))
		response.Blocks[i], err = proto.Marshal(block)
		log.Info(hex.EncodeToString(response.Blocks[i]))
		if err != nil {
			return err
		}
	}

	return nil
}
