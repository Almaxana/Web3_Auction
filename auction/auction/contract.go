package auction

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/lib/address"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

// Prefixes used for contract data storage.
const (
	initBetKey         = "initBet"
	currentBetKey      = "currentBet"
	potentialWinnerKey = "potentialWinner"
	lotKey             = "lot"
	ownerKey           = "ownerKey"
)

type AuctionItem struct {
	Owner      interop.Hash160
	InitialBet int
	CurrentBet int
	LotID      interop.Hash160
}

func _deploy(data interface{}, isUpdate bool) {
	if isUpdate {
		return
	}
}

func Update(script []byte, manifest []byte, data any) {
	management.UpdateWithData(script, manifest, data)
}

func Start(auctionOwner interop.Hash160, lotId []byte, initBet int) {
	ctx := storage.GetContext()

	currentOwner := storage.Get(ctx, ownerKey)
	if currentOwner != nil {
		panic("now current auction is processing, wait for finish")
	}
	if initBet < 0 {
		panic("initial bet must not be negative")
	}

	// 77a7c4e6f9307e5ce55136daa92ce5cb4621f8be - адрес nft контракта, чтобы получить из него nyanContractHashString, надо вручную в консоли
	// написать команду neo-go util convert 77a7c4e6f9307e5ce55136daa92ce5cb4621f8be и взять из нее LE ScriptHash to Address
	nyanContractHashString := "Nantu4ATNAbcqujTf8JFwtVpdikbFUkgc6"
	ownerOfLot := contract.Call(address.ToHash160(nyanContractHashString), "ownerOf", contract.All, lotId).(interop.Hash160)
	if !ownerOfLot.Equals(auctionOwner) {
		panic("\nyou can't start auction with lot " + string(lotId) + " because you're not its owner\n")
	}

	storage.Put(ctx, ownerKey, auctionOwner)
	storage.Put(ctx, lotKey, lotId)
	storage.Put(ctx, initBetKey, initBet)
	storage.Put(ctx, currentBetKey, initBet)
}

func ShowCurrentBet() string {
	data := storage.Get(storage.GetReadOnlyContext(), currentBetKey)
	if data == nil {
		return "0"
	}
	return string(data.([]byte))
}

func ShowLotId() string {
	data := storage.Get(storage.GetReadOnlyContext(), lotKey)
	if data == nil {
		return "nil"
	}

	return string(data.([]byte))
}

func GetPotentialWinner() interop.Hash160 {
	ctx := storage.GetReadOnlyContext()

	data := storage.Get(ctx, potentialWinnerKey)
	if data == nil {
		panic("no potential winner exists")
	}

	return data.(interop.Hash160)
}

func MakeBet(better interop.Hash160, bet int) {
	ctx := storage.GetContext()

	currentBetItem := storage.Get(ctx, currentBetKey)
	var currentBet int
	if currentBetItem == nil {
		currentBet = 0
	} else {
		currentBet = currentBetItem.(int)
	}

	auctionOwner := storage.Get(ctx, ownerKey).(interop.Hash160)

	if bet <= currentBet {
		panic("bet must be higher than the current bet")
	}

	if better.Equals(auctionOwner) {
		panic("auction owner cannot make bet")
	}

	if auctionOwner == nil {
		panic("auction has not started")
	}

	storage.Put(ctx, currentBetKey, bet)
	storage.Put(ctx, potentialWinnerKey, better)
}
