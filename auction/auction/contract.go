package auction

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/lib/address"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

const nyanContractHashString = "NdKipaSkcoXnwBLdBGCCb1exoi1HuiWj5x"

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

// Deploy function to initialize the contract
func _deploy(data interface{}, isUpdate bool) {
	if isUpdate {
		return
	}
}

func Update(script []byte, manifest []byte, data any) {
	management.UpdateWithData(script, manifest, data)
}

func Start(auctionOwnerHash160 interop.Hash160, lotId []byte, initBet int) {
	ctx := storage.GetContext()

	currentOwner := storage.Get(ctx, ownerKey)
	if currentOwner != nil {
		panic("Current auction is in progress, please wait until it finishes")
	}
	if initBet < 0 {
		panic("Initial bet must not be negative")
	}

	ownerOfLot := contract.Call(address.ToHash160(nyanContractHashString), "ownerOf", contract.All, lotId).(interop.Hash160)
	if !ownerOfLot.Equals(auctionOwnerHash160) {
		panic("\nYou can't start auction with lot " + string(lotId) + " because you're not its owner\n")
	}

	storage.Put(ctx, ownerKey, auctionOwnerHash160)
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

func FinishAuction() (winner interop.Hash160, lastBet int) {
	if !runtime.CheckWitness(runtime.GetScriptContainer().Sender) {
		panic("not witnessed")
	}

	ctx := storage.GetReadOnlyContext()

	winner, ok := storage.Get(ctx, potentialWinnerKey).(interop.Hash160)
	if !ok {
		panic("FinishAuction get the winner")
	}

	lastBet, ok = storage.Get(ctx, currentBetKey).(int)
	if !ok {
		panic("FinishAuction get the bet")
	}

	clearStorage()

	return
}

// clearStorage
// in this moment this func delete all values storage by hardcode prefix
func clearStorage() {
	ctx := storage.GetContext()

	storage.Delete(ctx, initBetKey)
	storage.Delete(ctx, currentBetKey)
	storage.Delete(ctx, potentialWinnerKey)
	storage.Delete(ctx, lotKey)
	storage.Delete(ctx, ownerKey)
}
