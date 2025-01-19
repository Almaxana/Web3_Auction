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
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"go.uber.org/zap"
)

func (s *Server) proceedMainTxGetPotentialWinner(ctx context.Context, nAct *notary.Actor, notaryEvent *result.NotaryRequestEvent) error {
	err := nAct.Sign(notaryEvent.NotaryRequest.MainTransaction)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}

	mainHash, fallbackHash, vub, err := nAct.Notarize(notaryEvent.NotaryRequest.MainTransaction, nil)
	s.log.Info("notarize sending",
		zap.String("hash", notaryEvent.NotaryRequest.Hash().String()),
		zap.String("main", mainHash.String()), zap.String("fb", fallbackHash.String()),
		zap.Uint32("vub", vub))

	_, err = nAct.Wait(mainHash, fallbackHash, vub, err) // Wait for transaction acceptance
	if err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	return nil
}

func validateNotaryRequestGetPotentialWinner(req *payload.P2PNotaryRequest) (util.Uint160, error) {
	var (
		opCode opcode.Opcode
		param  []byte
	)

	ctx := vm.NewContext(req.MainTransaction.Script) // Create a VM context to analyze the bytecode
	ops := make([]Op, 0, 20)

	var err error
	for {
		opCode, param, err = ctx.Next()
		if err != nil {
			return util.Uint160{}, fmt.Errorf("could not get next opcode in script: %w", err)
		}

		if opCode == opcode.RET {
			break
		}

		ops = append(ops, Op{code: opCode, param: param})
	}

	opsLen := len(ops)

	contractSysCall := make([]byte, 4)
	binary.LittleEndian.PutUint32(contractSysCall, interopnames.ToID([]byte(interopnames.SystemContractCall)))
	if !bytes.Equal(ops[opsLen-1].param, contractSysCall) {
		return util.Uint160{}, errors.New("not contract syscall")
	}

	// Retrieve contract's script hash
	contractHash, err := util.Uint160DecodeBytesBE(ops[opsLen-2].param) // Called contract - 2nd from the end
	if err != nil {
		return util.Uint160{}, err
	}

	contractHashExpected, err := util.Uint160DecodeStringLE("29c1332ede5f2ac639fa2c72b9e29babf110faaf") // contract hash
	if err != nil {
		return util.Uint160{}, err
	}

	if !contractHash.Equals(contractHashExpected) {
		return util.Uint160{}, fmt.Errorf("unexpected contract hash: %s", contractHash)
	}

	return contractHash, nil
}
