package auction

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/lib/address"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/management"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

// Prefixes used for contract data storage.
const (
	initBetKey         = "i"
	currentBetKey      = "c"
	lotKey             = "l" // nft id
	organizerKey       = "o" // organizer of the auction
	potentialWinnerKey = "w" // owner of the last bet
	ownerLotKey        = "n"
)

// b59cee8ea11307d6b911bf314ef01d63071448f3 - адрес nft контракта, чтобы получить из него nftContractHashString, надо вручную в консоли
// написать команду neo-go util convert b59cee8ea11307d6b911bf314ef01d63071448f3 и взять из нее LE ScriptHash to Address
const nftContractHashString = "NQBnDzrg6B6AconzXUwZkyLTnwkt9o9sBg"

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

	currentOwner := storage.Get(ctx, organizerKey)
	if currentOwner != nil {
		panic("now current auction is processing, wait for finish")
	}
	if initBet < 0 {
		panic("initial bet must not be negative")
	}

	ownerOfLot := contract.Call(address.ToHash160(nftContractHashString), "ownerOf", contract.All, lotId).(interop.Hash160)
	if !ownerOfLot.Equals(auctionOwner) {
		panic("you can't start auction with lot " + string(lotId) + " because you're not its owner")
	}

	storage.Put(ctx, organizerKey, auctionOwner)
	storage.Put(ctx, lotKey, lotId)
	storage.Put(ctx, initBetKey, initBet)
	storage.Put(ctx, currentBetKey, initBet)

	runtime.Notify("info", []byte("New auction started with initial bet = "+intToStr(initBet)))
}

func intToStr(value int) string {
	if value == 0 {
		return "0"
	}
	var chars = "0123456789"
	var result string
	for value > 0 {
		result = string(chars[value%10]) + result
		value = value / 10
	}

	return result
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

	ownerOfLot := contract.Call(address.ToHash160(nftContractHashString), "ownerOf", contract.All, lotID).(interop.Hash160)
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

	contract.Call(address.ToHash160(nftContractHashString), "transfer", contract.All, winner, lotID, nil)

	clearStorage()

	runtime.Notify("info", []byte("Auction has been finished. Winner is: "+ address.FromHash160(winner)))

	return winner
}

// clearStorage in this moment this func delete all values storage by hardcode prefix
func clearStorage() {
	ctx := storage.GetContext()

	storage.Delete(ctx, initBetKey)
	storage.Delete(ctx, currentBetKey)
	storage.Delete(ctx, potentialWinnerKey)
	storage.Delete(ctx, lotKey)
	storage.Delete(ctx, ownerLotKey)
	storage.Delete(ctx, organizerKey)
}
