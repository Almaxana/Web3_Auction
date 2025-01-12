package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"

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

func (s *Server) proceedMainTxStartAuction(ctx context.Context, nAct *notary.Actor, notaryEvent *result.NotaryRequestEvent, nftIdBytes []byte, initBet int) error {
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

	var ownerID user.ID
	user.IDFromKey(&ownerID, s.acc.PrivateKey().PrivateKey.PublicKey)

	obj := object.New()
	obj.SetContainerID(s.cnrID)
	obj.SetOwnerID(ownerID)

	var prm pool.PrmObjectPut
	prm.SetHeader(*obj)

	objID, err := s.p.PutObject(ctx, prm)
	if err != nil {
		return fmt.Errorf("put object '%s': %w", nftIdBytes, err)
	}

	addr := s.cnrID.EncodeToString() + "/" + objID.ObjectID.EncodeToString()
	s.log.Info("put object", zap.String("nftId", string(nftIdBytes)), zap.String("address", addr))

	return nil
}

func validateNotaryRequestStartAuction(req *payload.P2PNotaryRequest) (util.Uint160, []byte, int, error) {
	var (
		opCode opcode.Opcode
		param  []byte
	)

	ctx := vm.NewContext(req.MainTransaction.Script) // контекст vm, будем пошагаво разбирать байт код
	ops := make([]Op, 0, 20)

	var err error
	for {
		opCode, param, err = ctx.Next()
		if err != nil {
			return util.Uint160{}, nil, 0, fmt.Errorf("could not get next opcode in script: %w", err)
		}

		if opCode == opcode.RET {
			break
		}

		ops = append(ops, Op{code: opCode, param: param})
	}

	opsLen := len(ops)

	contractSysCall := make([]byte, 4)
	binary.LittleEndian.PutUint32(contractSysCall, interopnames.ToID([]byte(interopnames.SystemContractCall)))
	// check if it is tx with contract call
	if !bytes.Equal(ops[opsLen-1].param, contractSysCall) {
		return util.Uint160{}, nil, 0, errors.New("not contract syscall")
	}

	// retrieve contract's script hash
	contractHash, err := util.Uint160DecodeBytesBE(ops[opsLen-2].param) // вызываемый контракт - 2ая с конца инструкция
	if err != nil {
		return util.Uint160{}, nil, 0, err
	}

	contractHashExpected, err := util.Uint160DecodeStringLE("29c1332ede5f2ac639fa2c72b9e29babf110faaf") // вызываемый контракт
	if err != nil {
		return util.Uint160{}, nil, 0, err
	}

	if !contractHash.Equals(contractHashExpected) {
		return util.Uint160{}, nil, 0, fmt.Errorf("unexpected contract hash: %s", contractHash)
	}

	// check if there is a call flag(must be in range [0:15))
	callFlag := callflag.CallFlag(ops[opsLen-4].code - opcode.PUSH0)
	if callFlag > callflag.All {
		return util.Uint160{}, nil, 0, fmt.Errorf("incorrect call flag: %s", callFlag)
	}

	args := ops[:opsLen-4]

	if len(args) != 0 {
		err = validateParameterOpcodes(args)
		if err != nil {
			return util.Uint160{}, nil, 0, fmt.Errorf("could not validate arguments: %w", err)
		}

		// without args packing opcodes
		args = args[:len(args)-2]
	}

	if len(args) != 3 { // start принимает ровно 3 аргумента
		return util.Uint160{}, nil, 0, fmt.Errorf("invalid param length: %d", len(args))
	}

	nftIdBytes := args[1].Param()

	initBet := int(binary.LittleEndian.Uint16(args[0].Param()))

	sh, err := util.Uint160DecodeBytesBE(args[2].Param())
	if err != nil {
		return util.Uint160{}, nil, 0, fmt.Errorf("could not decode script hash: %w", err)
	}

	return sh, nftIdBytes, initBet, err
}
