package go_coin_fil

import (
	"context"
	"fmt"
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/chain/types"
	go_decimal "github.com/pefish/go-decimal"
	"github.com/pefish/go-test-assert"
	"testing"
	"time"
)

func TestWallet_NewSecp256k1Address(t *testing.T) {
	w := NewWallet()
	result, err := w.NewSecp256k1AddressFromSeed("aaa", address.Mainnet)
	test.Equal(t, nil, err)
	test.Equal(t, "f1fd5fdxw4padjxgufduxzy32n5zgx4cymjocqrfq", result.Address)
	test.Equal(t, "3030303030303030abefadbc0f1ce050fd35f4fc99d6ff45136d5c56bdc7f431", result.PrivateKey)
}

func TestWallet_GetAllFromPrivateKey(t *testing.T) {
	w := NewWallet()
	result, err := w.GetSecp256k1AddressFromPrivateKey("3030303030303030abefadbc0f1ce050fd35f4fc99d6ff45136d5c56bdc7f431", address.Mainnet)
	test.Equal(t, nil, err)
	test.Equal(t, "f1fd5fdxw4padjxgufduxzy32n5zgx4cymjocqrfq", result)
}

func TestWallet_InitRemote(t *testing.T) {
	w := NewWallet()
	err := w.InitRemote("192.168.50.248:12345", "")
	test.Equal(t, nil, err)
	defer w.Close()

	ctx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	ver, err := w.Remote.Version(ctx)
	test.Equal(t, nil, err)
	fmt.Println(ver.String())

	//ad, err := address.NewFromString("t1lo5nyuxekg5n545g2vyhu7evg5crj4eyhugzroq")
	//test.Equal(t, nil, err)
	//addr, err := w.Remote.StateAccountKey(ctx, ad, types.EmptyTSK)
	//addr.
}

func TestWallet_BuildTransferTx(t *testing.T) {
	w := NewWallet()
	err := w.InitRemote("192.168.50.248:12345", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIiwiYWRtaW4iXX0.D1YBeJdNFoW_ewLEFohg2rZvZjHjrVJExZv8AyPZlyE")
	test.Equal(t, nil, err)
	defer w.Close()

	msg, err := w.BuildTransferTx("30303030303030306eb482d265ace22c3cf9530a3e499de695afe6caf647f431", "t3voesoy2vnuwchb3fgupgvmejagrhk3yph5xv7sfus5e342arktvwcypstefad4qwxvpe6cup3xa3fymvxdta", go_decimal.Decimal.Start("0.1").MustShiftedBy(18).EndForString())
	test.Equal(t, nil, err)
	fmt.Println(msg)

	//ctx, _ := context.WithTimeout(context.Background(), 10 * time.Second)
	//cid, err := w.Remote.MpoolPush(ctx, msg)
	//test.Equal(t, nil, err)
	//fmt.Println(cid)
}

func TestWallet_ExportPrivateKey(t *testing.T) {
	w := NewWallet()
	str, err := w.ExportPrivateKey(types.KTSecp256k1, "30303030303030306eb482d265ace22c3cf9530a3e499de695afe6caf647f431")
	test.Equal(t, nil, err)
	fmt.Println(str)
}

func TestWallet_DecodeSubmitWindowedPoStParams(t *testing.T) {
	w := NewWallet()
	result, err := w.DecodeSubmitWindowedPoStParams("hROBggFAgYIFWMCLYdoi5iSm2hnzy6F4sJNUplhxdKCxK7FHUbtr725GlnLcWRyuWNJv1sr1RUqDt5y4m1WBZjL9I9AO+qXuH6ueu0FR7U9o4nkA8D/eIWA0eZTIgBTBjiDIiySyNbKX4jIMHQ+a73Ai/wQieSThvSNwFZdwqZNH1rVc4zSM+Xa5t5jzs6lBOGXmSBuQ9sLPVomXpiGQlUH/kb+wqA4AQ8O/JxtKZQy9ycZ2G+yLjsU/8rAcLytY4bWY7WiqJJd+pIIaAAI5clgg/gf5oZDOEQpmvSWNXr5X+vXKUnPjBW2MgQJxpakmyzw=")
	test.Equal(t, nil, err)
	test.Equal(t, 1, len(result.Partitions))
	test.Equal(t, uint64(1), result.Partitions[0].Index)
}