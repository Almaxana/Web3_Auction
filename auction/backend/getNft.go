package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"

	"git.frostfs.info/TrueCloudLab/frostfs-sdk-go/object"
	"git.frostfs.info/TrueCloudLab/frostfs-sdk-go/pool"
	"git.frostfs.info/TrueCloudLab/frostfs-sdk-go/user"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/notary"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"go.uber.org/zap"
)

func validateNotaryRequestGetNft(req *payload.P2PNotaryRequest) (util.Uint160, string, error) {
	var (
		opCode opcode.Opcode // мб = PUSH, CALL, RET и тп
		param  []byte        // параметры инструкции
	)

	ctx := vm.NewContext(req.MainTransaction.Script) // контекст vm, будем пошагаво разбирать байт код
	ops := make([]Op, 0, 10)                         // 10 is maximum num of opcodes for calling contracts with 4 args(no arrays of arrays)

	var err error
	for {
		opCode, param, err = ctx.Next()
		if err != nil {
			return util.Uint160{}, "", fmt.Errorf("could not get next opcode in script: %w", err)
		}

		if opCode == opcode.RET {
			break
		}

		ops = append(ops, Op{code: opCode, param: param})
	}

	opsLen := len(ops)

	contractSysCall := make([]byte, 4) // 4 байтам равен идентификатор системного вызова в neo
	binary.LittleEndian.PutUint32(contractSysCall, interopnames.ToID([]byte(interopnames.SystemContractCall)))
	// check if it is tx with contract call
	if !bytes.Equal(ops[opsLen-1].param, contractSysCall) { // смотрим последнюю инструкцию ops[opsLen-1]
		// потому что операция в скрипте tx, если tx соответствует вызову другого контракта,  в NeoVM всегда должна быть последней
		// проверяем, действительно последняя инструкция является вызовом контракта
		return util.Uint160{}, "", errors.New("not contract syscall")
	}

	// retrieve contract's script hash
	contractHash, err := util.Uint160DecodeBytesBE(ops[opsLen-2].param) // вызываемый контракт - 2ая с конца инструкция
	if err != nil {
		return util.Uint160{}, "", err
	}

	contractHashExpected, err := util.Uint160DecodeStringLE("77a7c4e6f9307e5ce55136daa92ce5cb4621f8be") // вызываемый контракт
	if err != nil {
		return util.Uint160{}, "", err
	}

	if !contractHash.Equals(contractHashExpected) {
		return util.Uint160{}, "", fmt.Errorf("unexpected contract hash: %s", contractHash)
	}

	// check if there is a call flag(must be in range [0:15))
	callFlag := callflag.CallFlag(ops[opsLen-4].code - opcode.PUSH0) // флаги - 4ая с конца инструкция
	if callFlag > callflag.All {
		return util.Uint160{}, "", fmt.Errorf("incorrect call flag: %s", callFlag)
	}

	args := ops[:opsLen-4] // аргументы - все инструкции до 4ой с конца

	if len(args) != 0 {
		err = validateParameterOpcodes(args)
		if err != nil {
			return util.Uint160{}, "", fmt.Errorf("could not validate arguments: %w", err)
		}

		// without args packing opcodes
		args = args[:len(args)-2]
	}

	// аргументы лежат в обратном порядке (как мы их передаем, только наоборот)
	if len(args) != 2 { // mint принимает ровно 2 аргумента
		return util.Uint160{}, "", fmt.Errorf("invalid param length: %d", len(args))
	}

	sh, err := util.Uint160DecodeBytesBE(args[1].Param())

	return sh, string(args[0].Param()), err
}

func (s *Server) proceedMainTxGetNft(ctx context.Context, nAct *notary.Actor, notaryEvent *result.NotaryRequestEvent, tokenName string) error {
	err := nAct.Sign(notaryEvent.NotaryRequest.MainTransaction)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	mainHash, fallbackHash, vub, err := nAct.Notarize(notaryEvent.NotaryRequest.MainTransaction, nil)
	s.log.Info("notarize sending",
		zap.String("hash", notaryEvent.NotaryRequest.Hash().String()),
		zap.String("main", mainHash.String()), zap.String("fb", fallbackHash.String()),
		zap.Uint32("vub", vub))

	_, err = nAct.Wait(mainHash, fallbackHash, vub, err) // ждем, пока какая-нибудь tx будет принята
	if err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	url := "https://www.nyan.cat/cats/" + tokenName

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("get url '%s' : %w", url, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			s.log.Error("close response bode", zap.Error(err))
		}
	}()

	var ownerID user.ID
	user.IDFromKey(&ownerID, s.acc.PrivateKey().PrivateKey.PublicKey)

	obj := object.New()
	obj.SetContainerID(s.cnrID)
	obj.SetOwnerID(ownerID)

	var prm pool.PrmObjectPut
	prm.SetPayload(resp.Body)
	prm.SetHeader(*obj)

	objID, err := s.p.PutObject(ctx, prm)
	if err != nil {
		return fmt.Errorf("put object '%s': %w", url, err)
	}

	addr := s.cnrID.EncodeToString() + "/" + objID.ObjectID.EncodeToString()
	s.log.Info("put object", zap.String("url", url), zap.String("address", addr))

	_, err = s.act.Wait(s.act.SendCall(s.nyanHash, "setAddress", tokenName, addr)) // добавляем адрес токену. После того, как произошел mint, заполнены у нового
	// nft будут поля, кроме address. Он будет добавляться отдельно здесь, после того, как токен создался.
	// Потому что пользователь должен знать, какую nft он хочет выписать
	if err != nil {
		return fmt.Errorf("wait setAddress: %w", err)
	}

	return nil
}
