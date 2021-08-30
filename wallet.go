package go_coin_fil

import (
	"context"
	"encoding/hex"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-crypto"
	"github.com/filecoin-project/go-jsonrpc"
	crypto2 "github.com/filecoin-project/go-state-types/crypto"
	lotusapi "github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/minio/blake2b-simd"
	go_error "github.com/pefish/go-error"
	"net/http"
	"strings"
	"time"
)

const (
	Method_Send = iota
)

type Wallet struct {
	Remote lotusapi.FullNodeStruct
	remoteCloser jsonrpc.ClientCloser
	timeout time.Duration
}

func NewWallet() *Wallet {
	return &Wallet{
		timeout: 30 * time.Second,
	}
}

func (w *Wallet) InitRemote(addr string, authToken string) error {
	headers := http.Header{}
	if authToken != "" {
		headers["Authorization"] = []string{"Bearer " + authToken}
	}

	closer, err := jsonrpc.NewMergeClient(context.Background(), "ws://"+addr+"/rpc/v0", "Filecoin", []interface{}{&w.Remote.Internal, &w.Remote.CommonStruct.Internal}, headers)
	if err != nil {
		return go_error.WithStack(err)
	}
	w.remoteCloser = closer

	return nil
}

func (w *Wallet) Close() {
	w.remoteCloser()

}

type NewAddressResult struct {
	PrivateKey string
	Address string
}

func (w *Wallet) NewSecp256k1AddressFromSeed(seed string, network address.Network) (*NewAddressResult, error) {
	if len(seed) < 40 {
		seed = seed + strings.Repeat("0", 40 - len(seed))
	}
	pkey, err := crypto.GenerateKeyFromSeed(strings.NewReader(seed))
	if err != nil {
		return nil, go_error.Wrap(err)
	}
	addr, err := address.NewSecp256k1Address(crypto.PublicKey(pkey))
	if err != nil {
		return nil, go_error.Wrap(err)
	}
	address.CurrentNetwork = network

	return &NewAddressResult{
		PrivateKey: hex.EncodeToString(pkey),
		Address: addr.String(),
	}, nil
}

func (w *Wallet) GetSecp256k1AddressFromPrivateKey(pkey string, network address.Network) (string, error) {
	secp256k1Addr, _, err := w.getSecp256k1AddressFromPrivateKey(pkey)
	if err != nil {
		return "", go_error.Wrap(err)
	}

	address.CurrentNetwork = network

	return secp256k1Addr.String(), nil
}

func (w *Wallet) getSecp256k1AddressFromPrivateKey(pkey string) (*address.Address, []byte, error) {
	pkeyBytes, err := hex.DecodeString(pkey)
	if err != nil {
		return nil, nil, go_error.WithStack(err)
	}
	secp256k1Addr, err := address.NewSecp256k1Address(crypto.PublicKey(pkeyBytes))
	if err != nil {
		return nil, nil, go_error.Wrap(err)
	}


	return &secp256k1Addr, pkeyBytes, nil
}

func (w *Wallet) BuildTransferTx(pkey string, to string, amount string) (*types.SignedMessage, error) {
	fromAddress, pkeyBytes, err := w.getSecp256k1AddressFromPrivateKey(pkey)
	if err != nil {
		return nil, go_error.WithStack(err)
	}
	toAddr, err := address.NewFromString(to)
	if err != nil {
		return nil, go_error.WithStack(err)
	}
	val, err := types.BigFromString(amount)
	if err != nil {
		return nil, go_error.WithStack(err)
	}
	msg := &types.Message{  // 构造交易信息
		To:     toAddr,
		From:   *fromAddress,
		Method: Method_Send,
		Params: make([]byte, 0),
		Value:  val,
	}

	ctx, _ := context.WithTimeout(context.Background(), w.timeout)
	msg, err = w.Remote.GasEstimateMessageGas(ctx, msg, nil, types.EmptyTSK)
	if err != nil {
		return nil, go_error.WithStack(err)
	}

	ctx, _ = context.WithTimeout(context.Background(), w.timeout)
	nonce, err := w.Remote.MpoolGetNonce(ctx, msg.From)
	if err != nil {
		return nil, go_error.WithStack(err)
	}
	msg.Nonce = nonce


	mb, err := msg.ToStorageBlock()
	if err != nil {
		return nil, go_error.WithStack(err)
	}


	b2sum := blake2b.Sum256(mb.Cid().Bytes())
	sig, err := crypto.Sign(pkeyBytes, b2sum[:])
	if err != nil {
		return nil, go_error.WithStack(err)
	}

	return &types.SignedMessage{
		Message:   *msg,
		Signature: crypto2.Signature{
			Type: crypto2.SigTypeSecp256k1,
			Data: sig,
		},
	}, nil

}
