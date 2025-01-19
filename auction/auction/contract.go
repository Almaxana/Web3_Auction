package auction

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/lib/address"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
)

// 77a7c4e6f9307e5ce55136daa92ce5cb4621f8be - адрес nft контракта, чтобы получить из него nyanContractHashString, надо вручную в консоли
// написать команду neo-go util convert 77a7c4e6f9307e5ce55136daa92ce5cb4621f8be и взять из нее LE ScriptHash to Address
const nyanContractHashString = "Nantu4ATNAbcqujTf8JFwtVpdikbFUkgc6"

// Prefixes used for contract data storage.
const (
	initBetKey         = "initBet"
	currentBetKey      = "currentBet"
	potentialWinnerKey = "potentialWinner"
	lotKey             = "lot"
	ownerKey           = "ownerKey"
)

type Item struct {
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

func Start(Owner interop.Hash160, lotId []byte, initBet int) {
	ctx := storage.GetContext()

	currentOwner := storage.Get(ctx, ownerKey)
	if currentOwner != nil {
		panic("now current  is processing, wait for finish")
	}
	if initBet < 0 {
		panic("initial bet must not be negative")
	}

	ownerOfLot := contract.Call(address.ToHash160(nyanContractHashString), "ownerOf", contract.All, lotId).(interop.Hash160)
	if !ownerOfLot.Equals(Owner) {
		panic("\nyou can't start  with lot " + string(lotId) + " because you're not its owner\n")
	}

	storage.Put(ctx, ownerKey, Owner)
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

func Finish(Owner interop.Hash160) interop.Hash160 {
	ctx := storage.GetReadOnlyContext()

	lotData := storage.Get(ctx, lotKey)
	if lotData == nil {
		panic("LotID is not set in storage")
	}
	lotID := lotData.([]byte)

	ownerOfLot := contract.Call(address.ToHash160(nyanContractHashString), "ownerOf", contract.All, lotID).(interop.Hash160)
	if !ownerOfLot.Equals(Owner) {
		panic("\nyou can't finish  with lot because you're not its owner\n")
	}

	var winner interop.Hash160
	winnerData := storage.Get(ctx, potentialWinnerKey)
	if winnerData == nil {
		winner = ownerOfLot
	} else {
		winner = winnerData.(interop.Hash160)
	}

	contract.Call(address.ToHash160(nyanContractHashString), "transfer", contract.All, winner, lotID, nil)

	clearStorage()

	return winner
}

// clearStorage in this moment this func delete all values storage by hardcode prefix
func clearStorage() {
	ctx := storage.GetContext()

	storage.Delete(ctx, initBetKey)
	storage.Delete(ctx, currentBetKey)
	storage.Delete(ctx, potentialWinnerKey)
	storage.Delete(ctx, lotKey)
	storage.Delete(ctx, ownerKey)
}
