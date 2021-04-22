package node

import (
	"context"
	"strings"
	"testing"

	"github.com/koinos/koinos-p2p/internal/options"

	types "github.com/koinos/koinos-types-golang"
)

type TestRPC struct {
	ChainID          types.UInt64
	Height           types.BlockHeightType
	HeadBlockIDDelta types.UInt64 // To ensure unique IDs within a "test chain", the multihash ID of each block is its height + this delta
	ApplyBlocks      int          // Number of blocks to apply before failure. < 0 = always apply
	BlocksApplied    []*types.Block
}

// GetHeadBlock rpc call
func (k *TestRPC) GetHeadBlock(ctx context.Context) (*types.GetHeadInfoResponse, error) {
	hi := types.NewGetHeadInfoResponse()
	hi.HeadTopology.Height = k.Height
	hi.HeadTopology.ID.ID = types.UInt64(k.Height) + k.HeadBlockIDDelta
	return hi, nil
}

// ApplyBlock rpc call
func (k *TestRPC) ApplyBlock(ctx context.Context, block *types.Block) (bool, error) {
	if k.ApplyBlocks >= 0 && len(k.BlocksApplied) >= k.ApplyBlocks {
		return false, nil
	}

	if k.BlocksApplied != nil {
		b := append(k.BlocksApplied, block)
		k.BlocksApplied = b
	}

	return true, nil
}

func (k *TestRPC) ApplyTransaction(ctx context.Context, txn *types.Transaction) (bool, error) {
	return true, nil
}

func (k *TestRPC) GetForkHeads(ctx context.Context) (*types.GetForkHeadsResponse, error) {
	return nil, nil
}

func (k *TestRPC) GetBlocksByID(ctx context.Context, blockID *types.VectorMultihash) (*types.GetBlocksByIDResponse, error) {
	return nil, nil
}

// GetBlocksByHeight rpc call
func (k *TestRPC) GetBlocksByHeight(ctx context.Context, blockID *types.Multihash, height types.BlockHeightType, numBlocks types.UInt32) (*types.GetBlocksByHeightResponse, error) {
	blocks := types.NewGetBlocksByHeightResponse()
	for i := types.UInt64(0); i < types.UInt64(numBlocks); i++ {
		blockItem := types.NewBlockItem()
		blockItem.BlockHeight = height + types.BlockHeightType(i)
		blockItem.BlockID = *types.NewMultihash()
		blockItem.BlockID.ID = types.UInt64(blockItem.BlockHeight) + k.HeadBlockIDDelta
		blockItem.Block = *types.NewOpaqueBlock()
		//vb := types.NewVariableBlob()
		//block := types.NewBlock()
		//blockItem.BlockBlob = *block.Serialize(vb)
		blocks.BlockItems = append(blocks.BlockItems, *blockItem)
	}

	return blocks, nil
}

// GetChainID rpc call
func (k *TestRPC) GetChainID(ctx context.Context) (*types.GetChainIDResponse, error) {
	mh := types.NewGetChainIDResponse()
	mh.ChainID.ID = k.ChainID
	return mh, nil
}

func (k *TestRPC) IsConnectedToBlockStore(ctx context.Context) (bool, error) {
	return true, nil
}

func (k *TestRPC) IsConnectedToChain(ctx context.Context) (bool, error) {
	return true, nil
}

func NewTestRPC(height types.BlockHeightType) *TestRPC {
	rpc := TestRPC{ChainID: 1, Height: height, HeadBlockIDDelta: 0, ApplyBlocks: -1}
	rpc.BlocksApplied = make([]*types.Block, 0)

	return &rpc
}

func TestBasicNode(t *testing.T) {
	ctx := context.Background()

	rpc := NewTestRPC(128)

	// With an explicit seed
	bn, err := NewKoinosP2PNode(ctx, "/ip4/127.0.0.1/tcp/8765", rpc, nil, "test1", options.NewConfig())
	if err != nil {
		t.Error(err)
	}

	addr := bn.GetPeerAddress()
	// Check peer address
	if !strings.HasPrefix(addr.String(), "/ip4/127.0.0.1/tcp/8765/p2p/Qm") {
		t.Errorf("Peer address returned by node is not correct")
	}

	bn.Close()

	// With blank seed
	bn, err = NewKoinosP2PNode(ctx, "/ip4/127.0.0.1/tcp/8765", rpc, nil, "", options.NewConfig())
	if err != nil {
		t.Error(err)
	}

	bn.Close()

	// Give an invalid listen address
	bn, err = NewKoinosP2PNode(ctx, "---", rpc, nil, "", options.NewConfig())
	if err == nil {
		bn.Close()
		t.Error("Starting a node with an invalid address should give an error, but it did not")
	}
}
