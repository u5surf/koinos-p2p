package rpc

import (
	"context"
	"encoding/json"
	"errors"

	log "github.com/koinos/koinos-log-golang"
	koinosmq "github.com/koinos/koinos-mq-golang"
	types "github.com/koinos/koinos-types-golang"
)

// RPC service constants
const (
	ChainRPC      = "chain"
	BlockStoreRPC = "block_store"
)

// KoinosRPC Implementation of RPC Interface
type KoinosRPC struct {
	mq *koinosmq.Client
}

// NewKoinosRPC factory
func NewKoinosRPC(mq *koinosmq.Client) *KoinosRPC {
	rpc := new(KoinosRPC)
	rpc.mq = mq
	return rpc
}

// GetHeadBlock rpc call
func (k *KoinosRPC) GetHeadBlock(ctx context.Context) (*types.GetHeadInfoResponse, error) {
	args := types.ChainRPCRequest{
		Value: types.NewGetHeadInfoRequest(),
	}
	data, err := json.Marshal(args)

	if err != nil {
		return nil, err
	}

	var responseBytes []byte
	responseBytes, err = k.mq.RPCContext(ctx, "application/json", ChainRPC, data)

	if err != nil {
		return nil, err
	}

	responseVariant := types.NewChainRPCResponse()
	err = json.Unmarshal(responseBytes, responseVariant)
	if err != nil {
		return nil, err
	}

	var response *types.GetHeadInfoResponse

	switch t := responseVariant.Value.(type) {
	case *types.GetHeadInfoResponse:
		response = t
	case *types.ChainErrorResponse:
		err = errors.New("Chain returned error processing GetHeadInfoRequest: " + string(t.ErrorText) + "    req=" + string(data))
	default:
		err = errors.New("Chain returned unexpected type processing GetHeadInfoRequest")
	}

	return response, err
}

// ApplyBlock rpc call
func (k *KoinosRPC) ApplyBlock(ctx context.Context, block *types.Block) (*types.SubmitBlockResponse, error) {
	blockSub := types.NewSubmitBlockRequest()
	blockSub.Block = *block

	blockSub.VerifyPassiveData = true
	blockSub.VerifyBlockSignature = true
	blockSub.VerifyTransactionSignatures = true

	args := types.ChainRPCRequest{
		Value: blockSub,
	}
	data, err := json.Marshal(args)

	if err != nil {
		return nil, err
	}

	var responseBytes []byte
	responseBytes, err = k.mq.RPCContext(ctx, "application/json", ChainRPC, data)

	if err != nil {
		return nil, err
	}

	responseVariant := types.NewChainRPCResponse()
	err = json.Unmarshal(responseBytes, responseVariant)
	if err != nil {
		return nil, err
	}

	var response *types.SubmitBlockResponse

	switch t := responseVariant.Value.(type) {
	case *types.SubmitBlockResponse:
		response = (*types.SubmitBlockResponse)(t)
	case *types.ChainErrorResponse:
		err = errors.New("Chain returned error processing SubmitBlockRequest: " + string(t.ErrorText) + "    req=" + string(data))
	default:
		err = errors.New("Chain returned unexpected type processing SubmitBlockRequest" + "    req=" + string(data))
	}

	return response, err
}

// ApplyTransaction rpc call
func (k *KoinosRPC) ApplyTransaction(ctx context.Context, trx *types.Transaction) (*types.SubmitTransactionResponse, error) {
	trxSub := types.NewSubmitTransactionRequest()
	trxSub.Transaction = *trx

	trxSub.VerifyPassiveData = true
	trxSub.VerifyTransactionSignatures = true

	args := types.ChainRPCRequest{
		Value: trxSub,
	}
	data, err := json.Marshal(args)

	if err != nil {
		return nil, err
	}

	var responseBytes []byte
	responseBytes, err = k.mq.RPCContext(ctx, "application/json", ChainRPC, data)

	if err != nil {
		return nil, err
	}

	responseVariant := types.NewChainRPCResponse()
	err = json.Unmarshal(responseBytes, responseVariant)
	if err != nil {
		return nil, err
	}

	var response *types.SubmitTransactionResponse

	switch t := responseVariant.Value.(type) {
	case *types.SubmitTransactionResponse:
		response = (*types.SubmitTransactionResponse)(t)
	case *types.ChainErrorResponse:
		err = errors.New("Chain returned error processing SubmitTransactionRequest: " + string(t.ErrorText) + "    req=" + string(data))
	default:
		err = errors.New("Chain returned unexpected type processing SubmitTransactionRequest" + "    req=" + string(data))
	}

	return response, err
}

// GetBlocksByID rpc call
func (k *KoinosRPC) GetBlocksByID(ctx context.Context, blockID *types.VectorMultihash) (*types.GetBlocksByIDResponse, error) {
	args := types.BlockStoreRequest{
		Value: &types.GetBlocksByIDRequest{
			BlockID:           *blockID,
			ReturnBlockBlob:   true,
			ReturnReceiptBlob: false,
		},
	}
	data, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}

	var responseBytes []byte
	responseBytes, err = k.mq.RPCContext(ctx, "application/json", BlockStoreRPC, data)
	if err != nil {
		return nil, err
	}

	log.Debugf("GetBlocksByID() response: %s", responseBytes)

	responseVariant := types.NewBlockStoreResponse()
	err = json.Unmarshal(responseBytes, responseVariant)
	if err != nil {
		return nil, err
	}

	var response *types.GetBlocksByIDResponse

	switch t := responseVariant.Value.(type) {
	case *types.GetBlocksByIDResponse:
		response = (*types.GetBlocksByIDResponse)(t)
	case *types.BlockStoreErrorResponse:
		err = errors.New("Block store returned error processing GetBlocksByIDRequest: " + string(t.ErrorText) + "    req=" + string(data))
	default:
		err = errors.New("Block store returned unexpected type processing GetBlocksByIDRequest    req=" + string(data))
	}

	return response, err
}

// GetBlocksByHeight rpc call
func (k *KoinosRPC) GetBlocksByHeight(ctx context.Context, blockID *types.Multihash, height types.BlockHeightType, numBlocks types.UInt32) (*types.GetBlocksByHeightResponse, error) {
	args := types.BlockStoreRequest{
		Value: &types.GetBlocksByHeightRequest{
			HeadBlockID:         *blockID,
			AncestorStartHeight: height,
			NumBlocks:           numBlocks,
			ReturnBlock:         true,
			ReturnReceipt:       false,
		},
	}
	data, err := json.Marshal(args)

	if err != nil {
		return nil, err
	}

	var responseBytes []byte
	responseBytes, err = k.mq.RPCContext(ctx, "application/json", BlockStoreRPC, data)

	if err != nil {
		return nil, err
	}

	responseVariant := types.NewBlockStoreResponse()
	err = json.Unmarshal(responseBytes, responseVariant)
	if err != nil {
		return nil, err
	}

	var response *types.GetBlocksByHeightResponse

	switch t := responseVariant.Value.(type) {
	case *types.GetBlocksByHeightResponse:
		response = (*types.GetBlocksByHeightResponse)(t)
	case *types.BlockStoreErrorResponse:
		err = errors.New("Block store returned error processing GetBlocksByHeightRequest: " + string(t.ErrorText) + "    req=" + string(data))
	default:
		err = errors.New("Block store returned unexpected type processing GetBlocksByHeightRequest    req=" + string(data))
	}

	return response, err
}

// GetChainID rpc call
func (k *KoinosRPC) GetChainID(ctx context.Context) (*types.GetChainIDResponse, error) {
	args := types.ChainRPCRequest{
		Value: types.NewGetChainIDRequest(),
	}
	data, err := json.Marshal(args)

	if err != nil {
		return nil, err
	}

	var responseBytes []byte
	responseBytes, err = k.mq.RPCContext(ctx, "application/json", ChainRPC, data)
	log.Debugf("GetChainID() response was %s", responseBytes)

	if err != nil {
		return nil, err
	}

	responseVariant := types.NewChainRPCResponse()
	err = json.Unmarshal(responseBytes, responseVariant)
	if err != nil {
		return nil, err
	}

	var response *types.GetChainIDResponse

	switch t := responseVariant.Value.(type) {
	case *types.GetChainIDResponse:
		response = (*types.GetChainIDResponse)(t)
	case *types.ChainErrorResponse:
		err = errors.New("Chain returned error processing GetChainIDRequest: " + string(t.ErrorText) + "    req=" + string(data))
	default:
		err = errors.New("Chain returned unexpected type processing GetChainIDRequest:    req=" + string(data))
	}

	return response, err
}

// GetForkHeads rpc call
func (k *KoinosRPC) GetForkHeads(ctx context.Context) (*types.GetForkHeadsResponse, error) {
	args := types.ChainRPCRequest{
		Value: types.NewGetForkHeadsRequest(),
	}

	data, err := json.Marshal(args)

	if err != nil {
		return nil, err
	}

	var responseBytes []byte
	responseBytes, err = k.mq.RPCContext(ctx, "application/json", ChainRPC, data)

	if err != nil {
		return nil, err
	}

	responseVariant := types.NewChainRPCResponse()
	err = json.Unmarshal(responseBytes, responseVariant)
	if err != nil {
		return nil, err
	}

	var response *types.GetForkHeadsResponse

	switch t := responseVariant.Value.(type) {
	case *types.GetForkHeadsResponse:
		response = (*types.GetForkHeadsResponse)(t)
	case *types.ChainErrorResponse:
		err = errors.New("Chain returned error processing GetForkHeadsRequest: " + string(t.ErrorText) + "    req=" + string(data))
	default:
		err = errors.New("Chain returned unexpected type processing GetForkHeadsRequest    req=" + string(data))
	}

	return response, err
}

// IsConnectedToBlockStore returns if the AMQP connection can currently communicate
// with the block store microservice.
func (k *KoinosRPC) IsConnectedToBlockStore(ctx context.Context) (bool, error) {
	args := types.BlockStoreRequest{
		Value: &types.BlockStoreReservedRequest{},
	}

	data, err := json.Marshal(args)

	if err != nil {
		return false, err
	}

	var responseBytes []byte
	responseBytes, err = k.mq.RPCContext(ctx, "application/json", BlockStoreRPC, data)

	if err != nil {
		return false, err
	}

	responseVariant := types.NewBlockStoreResponse()
	err = json.Unmarshal(responseBytes, responseVariant)
	if err != nil {
		return false, err
	}

	return true, nil
}

// IsConnectedToChain returns if the AMQP connection can currently communicate
// with the chain microservice.
func (k *KoinosRPC) IsConnectedToChain(ctx context.Context) (bool, error) {
	args := types.ChainRPCRequest{
		Value: &types.ChainReservedRequest{},
	}

	data, err := json.Marshal(args)

	if err != nil {
		return false, err
	}

	var responseBytes []byte
	responseBytes, err = k.mq.RPCContext(ctx, "application/json", ChainRPC, data)

	if err != nil {
		return false, err
	}

	responseVariant := types.NewChainRPCResponse()
	err = json.Unmarshal(responseBytes, responseVariant)
	if err != nil {
		return false, err
	}

	return true, nil
}
