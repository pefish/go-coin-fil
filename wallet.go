package go_coin_fil

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-crypto"
	"github.com/filecoin-project/go-jsonrpc"
	crypto2 "github.com/filecoin-project/go-state-types/crypto"
	lotusapi "github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"
	"github.com/filecoin-project/lotus/chain/types"
	miner2 "github.com/filecoin-project/specs-actors/actors/builtin/miner"
	"github.com/minio/blake2b-simd"
	go_error "github.com/pefish/go-error"
	"github.com/pkg/errors"
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

	if !strings.HasPrefix(addr, "ws") && !strings.HasPrefix(addr, "http") {
		addr = "ws://"+addr+"/rpc/v0"
	}

	closer, err := jsonrpc.NewMergeClient(context.Background(), addr, "Filecoin", []interface{}{&w.Remote.Internal, &w.Remote.CommonStruct.Internal}, headers)
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

func (w *Wallet) GetAddressFromPrivateKey(pkey string, pkeyType types.KeyType, network address.Network) (string, error) {
	secp256k1Addr, _, err := w.getAddressFromPrivateKey(pkey, pkeyType)
	if err != nil {
		return "", go_error.Wrap(err)
	}

	address.CurrentNetwork = network

	return secp256k1Addr.String(), nil
}

func (w *Wallet) ExportPrivateKey(keyType types.KeyType, privateKey string) (string, error) {
	priv, err := hex.DecodeString(privateKey)
	if err != nil {
		return "", go_error.Wrap(err)
	}
	by, err := json.Marshal(types.KeyInfo{
		Type:       keyType,
		PrivateKey: priv,
	})
	if err != nil {
		return "", go_error.Wrap(err)
	}
	return hex.EncodeToString(by), nil
}

func (w *Wallet) ImportPrivateKey(keyInfoStr string) (string, types.KeyType, error) {
	keyInfoByte, err := hex.DecodeString(keyInfoStr)
	if err != nil {
		return "", "", go_error.WithStack(errors.WithMessage(err, "decode key info error"))
	}
	var keyInfo types.KeyInfo
	err = json.Unmarshal(keyInfoByte, &keyInfo)
	if err != nil {
		return "", "", go_error.WithStack(errors.WithMessage(err, "unmarshal key info error"))
	}
	return hex.EncodeToString(keyInfo.PrivateKey), keyInfo.Type, nil
}

func (w *Wallet) getAddressFromPrivateKey(pkey string, pkeyType types.KeyType) (*address.Address, []byte, error) {
	pkeyBytes, err := hex.DecodeString(pkey)
	if err != nil {
		return nil, nil, go_error.WithStack(err)
	}
	var addr address.Address
	if pkeyType == types.KTBLS {
		return nil, nil, go_error.WithStack(errors.New("not be supported"))
	} else if pkeyType == types.KTSecp256k1 {
		secp256k1Addr, err := address.NewSecp256k1Address(crypto.PublicKey(pkeyBytes))
		if err != nil {
			return nil, nil, go_error.Wrap(err)
		}
		addr = secp256k1Addr
	} else {
		return nil, nil, go_error.WithStack(errors.New("key type error"))
	}

	return &addr, pkeyBytes, nil
}

func (w *Wallet) BuildTransferTx(pkey string, pkeyType types.KeyType, to string, amount string) (*types.SignedMessage, error) {
	fromAddress, pkeyBytes, err := w.getAddressFromPrivateKey(pkey, pkeyType)
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

	return w.SignMsg(pkeyBytes, pkeyType, msg)

}

func (w *Wallet) DecodeSubmitWindowedPoStParams(data string) (*miner.SubmitWindowedPoStParams, error) {
	var params = new(miner.SubmitWindowedPoStParams)
	bytes_, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	err = params.UnmarshalCBOR(bytes.NewReader(bytes_))
	return params, err
}

func (w *Wallet) SignMsg(pkeyBytes []byte, pkeyType types.KeyType, msg *types.Message) (*types.SignedMessage, error) {
	ctx, _ := context.WithTimeout(context.Background(), w.timeout)
	msg, err := w.Remote.GasEstimateMessageGas(ctx, msg, nil, types.EmptyTSK)
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

	var sig []byte
	if pkeyType == types.KTBLS {
		return nil, go_error.WithStack(errors.New("not be supported"))
	} else if pkeyType == types.KTSecp256k1 {
		b2sum := blake2b.Sum256(mb.Cid().Bytes())
		sig_, err := crypto.Sign(pkeyBytes, b2sum[:])
		if err != nil {
			return nil, go_error.WithStack(err)
		}
		sig = sig_
	} else {
		return nil, go_error.WithStack(errors.New("key type error"))
	}


	return &types.SignedMessage{
		Message:   *msg,
		Signature: crypto2.Signature{
			Type: crypto2.SigTypeSecp256k1,
			Data: sig,
		},
	}, nil
}

func (w *Wallet) BuildWithdrawFromMinerTx(pkey string, pkeyType types.KeyType, minerAddress string, amount string) (*types.SignedMessage, error) {
	fromAddress, pkeyBytes, err := w.getAddressFromPrivateKey(pkey, pkeyType)
	if err != nil {
		return nil, go_error.WithStack(err)
	}

	toAddr, err := address.NewFromString(minerAddress)
	if err != nil {
		return nil, go_error.WithStack(err)
	}

	amountBigInt, err := types.BigFromString(amount)
	if err != nil {
		return nil, go_error.WithStack(err)
	}

	//ctx, _ := context.WithTimeout(context.Background(), w.timeout)
	//available, err := w.Remote.StateMinerAvailableBalance(ctx, toAddr, types.EmptyTSK)
	//if err != nil {
	//	return nil, err
	//}
	//fmt.Println(available.String())

	params, err := actors.SerializeParams(&miner2.WithdrawBalanceParams{
		AmountRequested: amountBigInt,
	})
	if err != nil {
		return nil, go_error.WithStack(err)
	}

	msg := &types.Message{
		To:     toAddr,
		From:   *fromAddress,
		Value:  types.NewInt(0),
		Method: miner.Methods.WithdrawBalance,
		Params: params,
	}

	return w.SignMsg(pkeyBytes, pkeyType, msg)
}
