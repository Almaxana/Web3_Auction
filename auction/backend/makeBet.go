package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"

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

func (s *Server) proceedMainTxMakeBet(ctx context.Context, nAct *notary.Actor, notaryEvent *result.NotaryRequestEvent, args []string) error {
	// Декодируем адрес участника (better)????
	better, err := util.Uint160DecodeStringLE(args[3])
	if err != nil {
		return fmt.Errorf("error decoding better address: %w", err)
	}

	betStr := args[4]
	bet, err := strconv.Atoi(betStr) // bet amount to int
	if err != nil {
		return fmt.Errorf("error converting bet amount to integer: %w", err)
	}

	err = nAct.Sign(notaryEvent.NotaryRequest.MainTransaction) // sign transaction
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	// send notary query
	mainHash, fallbackHash, vub, err := nAct.Notarize(notaryEvent.NotaryRequest.MainTransaction, nil)
	if err != nil {
		return fmt.Errorf("notarize: %w", err)
	}

	s.log.Info("notarize sending",
		zap.String("hash", notaryEvent.NotaryRequest.MainTransaction.Hash().String()),
		zap.String("main", mainHash.String()),
		zap.String("fallback", fallbackHash.String()),
		zap.Uint32("vub", vub))

	_, err = nAct.Wait(mainHash, fallbackHash, vub, err)
	if err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	s.log.Info("bet placed successfully",
		zap.String("better", better.String()),
		zap.Int("bet", bet))
	return nil
}

func validateNotaryRequestMakeBet(req *payload.P2PNotaryRequest) (util.Uint160, int, error) {
	var (
		opCode opcode.Opcode
		param  []byte
	)

	ctx := vm.NewContext(req.MainTransaction.Script)
	ops := make([]Op, 0, 20)

	var err error
	for {
		opCode, param, err = ctx.Next()
		if err != nil {
			return util.Uint160{}, 0, fmt.Errorf("could not get next opcode in script: %w", err)
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
		return util.Uint160{}, 0, errors.New("not a contract syscall")
	}

	// адрес вызываемого контракта
	contractHash, err := util.Uint160DecodeBytesBE(ops[opsLen-2].param)
	if err != nil {
		return util.Uint160{}, 0, err
	}

	contractHashExpected, err := util.Uint160DecodeStringLE("29c1332ede5f2ac639fa2c72b9e29babf110faaf") // Скрипт-хэш контракта
	if err != nil {
		return util.Uint160{}, 0, err
	}

	if !contractHash.Equals(contractHashExpected) {
		return util.Uint160{}, 0, fmt.Errorf("unexpected contract hash: %s", contractHash)
	}

	// check if there is a call flag(must be in range [0:15))
	callFlag := callflag.CallFlag(ops[opsLen-4].code - opcode.PUSH0)
	if callFlag > callflag.All {
		return util.Uint160{}, 0, fmt.Errorf("incorrect call flag: %s", callFlag)
	}

	args := ops[:opsLen-4]

	if len(args) != 0 {
		err = validateParameterOpcodes(args)
		if err != nil {
			return util.Uint160{}, 0, fmt.Errorf("could not validate arguments: %w", err)
		}

		// without args packing opcodes
		args = args[:len(args)-2]
	}

	// проверяем, что makeBet принимает 2 аргумента
	if len(args) != 2 {
		return util.Uint160{}, 0, fmt.Errorf("invalid param length: %d", len(args))
	}

	bet := int(binary.LittleEndian.Uint16(args[0].Param()))

	better, err := util.Uint160DecodeBytesBE(args[1].Param())
	if err != nil {
		return util.Uint160{}, 0, fmt.Errorf("could not decode script hash: %w", err)
	}

	return better, bet, nil
}
