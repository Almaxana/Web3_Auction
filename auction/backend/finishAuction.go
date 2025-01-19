package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"

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

func (s *Server) proceedMainTxFinishAuction(ctx context.Context, nAct *notary.Actor, notaryEvent *result.NotaryRequestEvent) error {
	err := nAct.Sign(notaryEvent.NotaryRequest.MainTransaction)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	mainHash, fallbackHash, vub, err := nAct.Notarize(notaryEvent.NotaryRequest.MainTransaction, nil)
	s.log.Info("notarize sending",
		zap.String("hash", notaryEvent.NotaryRequest.Hash().String()),
		zap.String("main", mainHash.String()), zap.String("fb", fallbackHash.String()),
		zap.Uint32("vub", vub))

	_, err = nAct.Wait(mainHash, fallbackHash, vub, err)
	if err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	return nil
}

func validateNotaryRequestFinishAuction(req *payload.P2PNotaryRequest) error {
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
			return fmt.Errorf("could not get next opcode in script: %w", err)
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
		return errors.New("not contract syscall")
	}

	// retrieve contract's script hash
	contractHash, err := util.Uint160DecodeBytesBE(ops[opsLen-2].param) // вызываемый контракт - 2ая с конца инструкция
	if err != nil {
		return err
	}

	contractHashExpected, err := util.Uint160DecodeStringLE("c1c0a967c8edc4158f605098b51e8e794b9cb2af") // вызываемый контракт
	if err != nil {
		return err
	}

	if !contractHash.Equals(contractHashExpected) {
		return fmt.Errorf("unexpected contract hash: %s", contractHash)
	}

	// check if there is a call flag(must be in range [0:15))
	callFlag := callflag.CallFlag(ops[opsLen-4].code - opcode.PUSH0)
	if callFlag > callflag.All {
		return fmt.Errorf("incorrect call flag: %s", callFlag)
	}

	args := ops[:opsLen-4]

	if len(args) != 0 {
		err = validateParameterOpcodes(args)
		if err != nil {
			return fmt.Errorf("could not validate arguments: %w", err)
		}

		// without args packing opcodes
		args = args[:len(args)-2]
	}

	if len(args) != 1 {
		return fmt.Errorf("invalid param length: %d", len(args))
	}

	_, err = util.Uint160DecodeBytesBE(args[0].Param())
	if err != nil {
		return fmt.Errorf("could not decode script hash: %w", err)
	}

	return nil
}
