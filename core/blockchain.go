// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package core implements the Ethereum consensus protocol.
package core

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ledgerwatch/erigon-lib/chain"
	libcommon "github.com/ledgerwatch/erigon-lib/common"
	"golang.org/x/crypto/sha3"
	"golang.org/x/exp/slices"

	"github.com/ledgerwatch/erigon/core/systemcontracts"
	"github.com/ledgerwatch/erigon/core/vm/evmtypes"
	"github.com/ledgerwatch/erigon/firehose"
	"github.com/ledgerwatch/erigon/rlp"

	metrics2 "github.com/VictoriaMetrics/metrics"

	"github.com/ledgerwatch/erigon/common/math"
	"github.com/ledgerwatch/erigon/common/u256"
	"github.com/ledgerwatch/erigon/consensus"
	"github.com/ledgerwatch/erigon/consensus/misc"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/core/vm"
)

var (
	BlockExecutionTimer = metrics2.GetOrCreateSummary("chain_execution_seconds")
)

type SyncMode string

const (
	TriesInMemory = 128
)

type RejectedTx struct {
	Index int    `json:"index"    gencodec:"required"`
	Err   string `json:"error"    gencodec:"required"`
}

type RejectedTxs []*RejectedTx

type EphemeralExecResult struct {
	StateRoot        libcommon.Hash        `json:"stateRoot"`
	TxRoot           libcommon.Hash        `json:"txRoot"`
	ReceiptRoot      libcommon.Hash        `json:"receiptsRoot"`
	LogsHash         libcommon.Hash        `json:"logsHash"`
	Bloom            types.Bloom           `json:"logsBloom"        gencodec:"required"`
	Receipts         types.Receipts        `json:"receipts"`
	Rejected         RejectedTxs           `json:"rejected,omitempty"`
	Difficulty       *math.HexOrDecimal256 `json:"currentDifficulty" gencodec:"required"`
	GasUsed          math.HexOrDecimal64   `json:"gasUsed"`
	StateSyncReceipt *types.Receipt        `json:"-"`
}

func ExecuteBlockEphemerallyForBSC(
	chainConfig *chain.Config,
	vmConfig *vm.Config,
	blockHashFunc func(n uint64) libcommon.Hash,
	engine consensus.Engine,
	block *types.Block,
	stateReader state.StateReader,
	stateWriter state.WriterWithChangeSets,
	epochReader consensus.EpochReader,
	chainReader consensus.ChainHeaderReader,
	getTracer func(txIndex int, txHash libcommon.Hash) (vm.EVMLogger, error),
) (*EphemeralExecResult, error) {
	defer BlockExecutionTimer.UpdateDuration(time.Now())
	block.Uncles()
	ibs := state.New(stateReader)
	header := block.Header()
	usedGas := new(uint64)
	gp := new(GasPool)
	gp.AddGas(block.GasLimit())

	var (
		rejectedTxs     []*RejectedTx
		includedTxs     types.Transactions
		receipts        types.Receipts
		firehoseContext = firehose.NoOpContext
	)

	if !vmConfig.ReadOnly {
		firehoseContext = firehose.MaybeSyncContext()
	}

	if firehoseContext.Enabled() {
		firehoseContext.StartBlock(block)
	}

	if !vmConfig.ReadOnly {
		if err := InitializeBlockExecution(engine, chainReader, epochReader, block.Header(), block.Transactions(), block.Uncles(), chainConfig, ibs, firehoseContext); err != nil {
			if firehoseContext.Enabled() {
				firehoseContext.CancelBlock(block, err)
			}

			return nil, err
		}
	}

	if chainConfig.DAOForkSupport && chainConfig.DAOForkBlock != nil && chainConfig.DAOForkBlock.Cmp(block.Number()) == 0 {
		misc.ApplyDAOHardFork(ibs, firehoseContext)
	}
	systemcontracts.UpgradeBuildInSystemContract(chainConfig, header.Number, ibs)
	noop := state.NewNoopWriter()
	posa, isPoSA := engine.(consensus.PoSA)
	//fmt.Printf("====txs processing start: %d====\n", block.NumberU64())
	for i, tx := range block.Transactions() {
		if isPoSA {
			if isSystemTx, err := posa.IsSystemTransaction(tx, block.Header()); err != nil {
				if firehoseContext.Enabled() {
					firehoseContext.RecordFailedTransaction(err)
					firehoseContext.ExitBlock()
				}

				return nil, err
			} else if isSystemTx {
				continue
			}
		}

		// CS TODO: check if we need to include system transctions
		if firehoseContext.Enabled() {
			firehoseContext.StartTransaction(tx, header.BaseFee)
		}

		ibs.Prepare(tx.Hash(), block.Hash(), i)
		writeTrace := false
		if vmConfig.Debug && vmConfig.Tracer == nil {
			tracer, err := getTracer(i, tx.Hash())
			if err != nil {
				err := fmt.Errorf("could not obtain tracer: %w", err)
				if firehoseContext.Enabled() {
					firehoseContext.RecordFailedTransaction(err)
					firehoseContext.ExitBlock()
				}

				return nil, err
			}
			vmConfig.Tracer = tracer
			writeTrace = true
		}

		receipt, _, err := ApplyTransaction(chainConfig, blockHashFunc, engine, nil, gp, ibs, noop, header, tx, usedGas, *vmConfig, firehoseContext)
		if writeTrace {
			if ftracer, ok := vmConfig.Tracer.(vm.FlushableTracer); ok {
				ftracer.Flush(tx)
			}

			vmConfig.Tracer = nil
		}
		if err != nil {
			if !vmConfig.StatelessExec {
				err := fmt.Errorf("could not apply tx %d from block %d [%v]: %w", i, block.NumberU64(), tx.Hash().Hex(), err)
				if firehoseContext.Enabled() {
					firehoseContext.RecordFailedTransaction(err)
					firehoseContext.ExitBlock()
				}

				return nil, err
			}

			// There is nothing to do with rejected transactions for Firehose (actually rejected transactions are not used anymore within Erigon codebase)
			rejectedTxs = append(rejectedTxs, &RejectedTx{i, err.Error()})
		} else {
			includedTxs = append(includedTxs, tx)
			if !vmConfig.NoReceipts {
				receipts = append(receipts, receipt)
			}

			if firehoseContext.Enabled() {
				firehoseContext.EndTransaction(receipt)
			}
		}
	}

	var newBlock *types.Block
	var receiptSha libcommon.Hash
	if !vmConfig.ReadOnly {
		// We're doing this hack for BSC to avoid changing consensus interfaces a lot. BSC modifies txs and receipts by appending
		// system transactions, and they increase used gas and write cumulative gas to system receipts, that's why we need
		// to deduct system gas before. This line is equal to "blockGas-systemGas", but since we don't know how much gas is
		// used by system transactions we just override. Of course, we write used by block gas back. It also always true
		// that used gas by block is always equal to original's block header gas, and it's checked by receipts root verification
		// otherwise it causes block verification error.
		header.GasUsed = *usedGas
		syscall := func(contract libcommon.Address, data []byte) ([]byte, error) {
			return SysCallContract(contract, data, *chainConfig, ibs, header, engine, false /* constCall */, firehoseContext)
		}

		// Finalize block is a bit special since it can be enabled without the full firehose sync.
		// As such, if firehose is enabled, we log it and us the firehose context. Otherwise if
		// block progress is enabled.
		if firehoseContext.Enabled() {
			firehoseContext.FinalizeBlock(block)
		} else if firehose.BlockProgressEnabled {
			firehose.SyncContext().FinalizeBlock(block)
		}

		outTxs, outReceipts, err := engine.Finalize(chainConfig, header, ibs, block.Transactions(), block.Uncles(), receipts, block.Withdrawals(), epochReader, chainReader, syscall, firehoseContext)
		if err != nil {
			if firehoseContext.Enabled() {
				firehoseContext.CancelBlock(block, err)
			}

			return nil, err
		}
		*usedGas = header.GasUsed

		// We need repack this block because transactions and receipts might be changed by consensus, and
		// it won't pass receipts hash or bloom verification
		newBlock = types.NewBlock(block.Header(), outTxs, block.Uncles(), outReceipts, block.Withdrawals())
		// Update receipts
		if !vmConfig.NoReceipts {
			receipts = outReceipts
		}
		receiptSha = newBlock.ReceiptHash()
	} else {
		newBlock = block
		receiptSha = types.DeriveSha(receipts)
	}

	if chainConfig.IsByzantium(header.Number.Uint64()) && !vmConfig.NoReceipts {
		if !vmConfig.StatelessExec && receiptSha != block.ReceiptHash() {
			err := fmt.Errorf("mismatched receipt headers for block %d (%s != %s)", block.NumberU64(), receiptSha.Hex(), block.ReceiptHash().Hex())
			if firehoseContext.Enabled() {
				firehoseContext.CancelBlock(block, err)
			}

			return nil, err
		}
	}
	if !vmConfig.StatelessExec && newBlock.GasUsed() != header.GasUsed {
		err := fmt.Errorf("gas used by execution: %d, in header: %d, in new Block: %v", *usedGas, header.GasUsed, newBlock.GasUsed())
		if firehoseContext.Enabled() {
			firehoseContext.CancelBlock(newBlock, err)
		}

		return nil, err
	}

	var bloom types.Bloom
	if !vmConfig.NoReceipts {
		bloom = newBlock.Bloom()
		if !vmConfig.StatelessExec && bloom != header.Bloom {
			err := fmt.Errorf("bloom computed by execution: %x, in header: %x", bloom, header.Bloom)
			if firehoseContext.Enabled() {
				firehoseContext.CancelBlock(newBlock, err)
			}

			return nil, err
		}
	}

	if err := ibs.CommitBlock(chainConfig.Rules(header.Number.Uint64(), header.Time), stateWriter); err != nil {
		err := fmt.Errorf("committing block %d failed: %w", header.Number.Uint64(), err)
		if firehoseContext.Enabled() {
			firehoseContext.CancelBlock(newBlock, err)
		}

		return nil, err
	} else if err := stateWriter.WriteChangeSets(); err != nil {
		err := fmt.Errorf("writing changesets for block %d failed: %w", header.Number.Uint64(), err)
		if firehoseContext.Enabled() {
			firehoseContext.CancelBlock(newBlock, err)
		}

		return nil, err
	}

	if firehoseContext.Enabled() {
		// It seems BSC instrumentation repack the `block` into a `newBlock` variable because BSC
		// consensus might have tweaked the block, so we really need to use `newBlock` because it's
		// the one using the final correct value that will be broadcasted to the network

		// Calculate the total difficulty of the block
		ptd := chainReader.GetTd(newBlock.ParentHash(), newBlock.NumberU64()-1)
		difficulty := newBlock.Difficulty()
		if difficulty == nil {
			difficulty = big.NewInt(0)
		}

		td := ptd
		if ptd != nil {
			td = new(big.Int).Add(difficulty, ptd)
		}

		// The `finalizedBlock` argument **must** be `nil` on Bor because we still have an hard-coded -200 finalized block on Polygon
		firehoseContext.EndBlock(newBlock, nil, td)
	}

	execRs := &EphemeralExecResult{
		TxRoot:      types.DeriveSha(includedTxs),
		ReceiptRoot: receiptSha,
		Bloom:       bloom,
		Receipts:    receipts,
		Difficulty:  (*math.HexOrDecimal256)(block.Header().Difficulty),
		GasUsed:     math.HexOrDecimal64(*usedGas),
		Rejected:    rejectedTxs,
	}

	return execRs, nil
}

// ExecuteBlockEphemerally runs a block from provided stateReader and
// writes the result to the provided stateWriter
func ExecuteBlockEphemerally(
	chainConfig *chain.Config,
	vmConfig *vm.Config,
	blockHashFunc func(n uint64) libcommon.Hash,
	engine consensus.Engine,
	block *types.Block,
	stateReader state.StateReader,
	stateWriter state.WriterWithChangeSets,
	epochReader consensus.EpochReader,
	chainReader consensus.ChainHeaderReader,
	getTracer func(txIndex int, txHash libcommon.Hash) (vm.EVMLogger, error),
) (*EphemeralExecResult, error) {

	defer BlockExecutionTimer.UpdateDuration(time.Now())
	block.Uncles()
	ibs := state.New(stateReader)
	header := block.Header()

	usedGas := new(uint64)
	gp := new(GasPool)
	gp.AddGas(block.GasLimit())

	var (
		rejectedTxs     []*RejectedTx
		includedTxs     types.Transactions
		receipts        types.Receipts
		firehoseContext = firehose.NoOpContext
	)

	if !vmConfig.ReadOnly {
		firehoseContext = firehose.MaybeSyncContext()
	}

	if firehoseContext.Enabled() {
		firehoseContext.StartBlock(block)
	}

	if !vmConfig.ReadOnly {
		if err := InitializeBlockExecution(engine, chainReader, epochReader, block.Header(), block.Transactions(), block.Uncles(), chainConfig, ibs, firehoseContext); err != nil {
			if firehoseContext.Enabled() {
				firehoseContext.CancelBlock(block, err)
			}
			return nil, err
		}
	}

	if chainConfig.DAOForkSupport && chainConfig.DAOForkBlock != nil && chainConfig.DAOForkBlock.Cmp(block.Number()) == 0 {
		misc.ApplyDAOHardFork(ibs, firehoseContext)
	}
	noop := state.NewNoopWriter()
	//fmt.Printf("====txs processing start: %d====\n", block.NumberU64())
	for i, tx := range block.Transactions() {
		if firehoseContext.Enabled() {
			firehoseContext.StartTransaction(tx, header.BaseFee)
		}

		ibs.Prepare(tx.Hash(), block.Hash(), i)
		writeTrace := false
		if vmConfig.Debug && vmConfig.Tracer == nil {
			tracer, err := getTracer(i, tx.Hash())
			if err != nil {
				err := fmt.Errorf("could not obtain tracer: %w", err)
				if firehoseContext.Enabled() {
					firehoseContext.RecordFailedTransaction(err)
					firehoseContext.ExitBlock()
				}

				return nil, err
			}
			vmConfig.Tracer = tracer
			writeTrace = true
		}

		receipt, _, err := ApplyTransaction(chainConfig, blockHashFunc, engine, nil, gp, ibs, noop, header, tx, usedGas, *vmConfig, firehoseContext)
		if writeTrace {
			if ftracer, ok := vmConfig.Tracer.(vm.FlushableTracer); ok {
				ftracer.Flush(tx)
			}

			vmConfig.Tracer = nil
		}
		if err != nil {
			if !vmConfig.StatelessExec {
				err := fmt.Errorf("could not apply tx %d from block %d [%v]: %w", i, block.NumberU64(), tx.Hash().Hex(), err)
				if firehoseContext.Enabled() {
					firehoseContext.RecordFailedTransaction(err)
					firehoseContext.ExitBlock()
				}

				return nil, err
			}

			// There is nothing to do with rejected transactions for Firehose (actually rejected transactions are not used anymore within Erigon codebase)
			rejectedTxs = append(rejectedTxs, &RejectedTx{i, err.Error()})
		} else {
			includedTxs = append(includedTxs, tx)
			if !vmConfig.NoReceipts {
				receipts = append(receipts, receipt)
			}

			if firehoseContext.Enabled() {
				firehoseContext.EndTransaction(receipt)
			}
		}
	}

	receiptSha := types.DeriveSha(receipts)
	if !vmConfig.StatelessExec && chainConfig.IsByzantium(header.Number.Uint64()) && !vmConfig.NoReceipts && receiptSha != block.ReceiptHash() {
		err := fmt.Errorf("mismatched receipt headers for block %d (%s != %s)", block.NumberU64(), receiptSha.Hex(), block.ReceiptHash().Hex())
		if firehoseContext.Enabled() {
			firehoseContext.CancelBlock(block, err)
		}

		return nil, err
	}

	if !vmConfig.StatelessExec && *usedGas != header.GasUsed {
		err := fmt.Errorf("gas used by execution: %d, in header: %d", *usedGas, header.GasUsed)
		if firehoseContext.Enabled() {
			firehoseContext.CancelBlock(block, err)
		}

		return nil, err
	}

	var bloom types.Bloom
	if !vmConfig.NoReceipts {
		bloom = types.CreateBloom(receipts)
		if !vmConfig.StatelessExec && bloom != header.Bloom {
			err := fmt.Errorf("bloom computed by execution: %x, in header: %x", bloom, header.Bloom)
			if firehoseContext.Enabled() {
				firehoseContext.CancelBlock(block, err)
			}

			return nil, err
		}
	}

	if !vmConfig.ReadOnly {
		// Finalize block is a bit special since it can be enabled without the full firehose sync.
		// As such, if firehose is enabled, we log it and us the firehose context. Otherwise if
		// block progress is enabled.
		if firehoseContext.Enabled() {
			firehoseContext.FinalizeBlock(block)
		} else if firehose.BlockProgressEnabled {
			firehose.SyncContext().FinalizeBlock(block)
		}

		txs := block.Transactions()
		if _, _, _, err := FinalizeBlockExecution(engine, stateReader, block.Header(), txs, block.Uncles(), stateWriter, chainConfig, ibs, receipts, block.Withdrawals(), epochReader, chainReader, false, firehoseContext); err != nil {
			if firehoseContext.Enabled() {
				firehoseContext.CancelBlock(block, err)
			}

			return nil, err
		}
	}

	if firehoseContext.Enabled() {
		// Calculate the total difficulty of the block
		ptd := chainReader.GetTd(block.ParentHash(), block.NumberU64()-1)
		difficulty := block.Difficulty()
		if difficulty == nil {
			difficulty = big.NewInt(0)
		}

		td := ptd
		if ptd != nil {
			td = new(big.Int).Add(difficulty, ptd)
		}

		finalizedBlock := chainReader.CurrentFinalizedHeader()
		if finalizedBlock != nil && firehose.SyncingBehindFinalized() {
			// if beaconFinalizedBlockNum is in the future, the 'finalizedBlock' will not progress until we reach it.
			// we don't want to advertise a super old finalizedBlock when reprocessing.
			finalizedBlock = nil
		}

		firehoseContext.EndBlock(block, finalizedBlock, td)
	}

	blockLogs := ibs.Logs()
	execRs := &EphemeralExecResult{
		TxRoot:      types.DeriveSha(includedTxs),
		ReceiptRoot: receiptSha,
		Bloom:       bloom,
		LogsHash:    rlpHash(blockLogs),
		Receipts:    receipts,
		Difficulty:  (*math.HexOrDecimal256)(header.Difficulty),
		GasUsed:     math.HexOrDecimal64(*usedGas),
		Rejected:    rejectedTxs,
	}

	return execRs, nil
}

// ExecuteBlockEphemerallyBor runs a block from provided stateReader and
// writes the result to the provided stateWriter
func ExecuteBlockEphemerallyBor(
	chainConfig *chain.Config,
	vmConfig *vm.Config,
	blockHashFunc func(n uint64) libcommon.Hash,
	engine consensus.Engine,
	block *types.Block,
	stateReader state.StateReader,
	stateWriter state.WriterWithChangeSets,
	epochReader consensus.EpochReader,
	chainReader consensus.ChainHeaderReader,
	getTracer func(txIndex int, txHash libcommon.Hash) (vm.EVMLogger, error),
) (*EphemeralExecResult, error) {

	defer BlockExecutionTimer.UpdateDuration(time.Now())
	block.Uncles()
	ibs := state.New(stateReader)
	header := block.Header()

	usedGas := new(uint64)
	gp := new(GasPool)
	gp.AddGas(block.GasLimit())

	var (
		rejectedTxs     []*RejectedTx
		includedTxs     types.Transactions
		receipts        types.Receipts
		firehoseContext = firehose.NoOpContext
	)

	if !vmConfig.ReadOnly {
		firehoseContext = firehose.MaybeSyncContext()
	}

	if firehoseContext.Enabled() {
		firehoseContext.StartBlock(block)
	}

	if !vmConfig.ReadOnly {
		if err := InitializeBlockExecution(engine, chainReader, epochReader, block.Header(), block.Transactions(), block.Uncles(), chainConfig, ibs, firehoseContext); err != nil {
			if firehoseContext.Enabled() {
				firehoseContext.CancelBlock(block, err)
			}

			return nil, err
		}
	}

	if chainConfig.DAOForkSupport && chainConfig.DAOForkBlock != nil && chainConfig.DAOForkBlock.Cmp(block.Number()) == 0 {
		misc.ApplyDAOHardFork(ibs, firehoseContext)
	}
	noop := state.NewNoopWriter()
	//fmt.Printf("====txs processing start: %d====\n", block.NumberU64())
	for i, tx := range block.Transactions() {
		if firehoseContext.Enabled() {
			firehoseContext.StartTransaction(tx, header.BaseFee)
		}

		ibs.Prepare(tx.Hash(), block.Hash(), i)
		writeTrace := false
		if vmConfig.Debug && vmConfig.Tracer == nil {
			tracer, err := getTracer(i, tx.Hash())
			if err != nil {
				err := fmt.Errorf("could not obtain tracer: %w", err)
				if firehoseContext.Enabled() {
					firehoseContext.RecordFailedTransaction(err)
					firehoseContext.ExitBlock()
				}

				return nil, err
			}
			vmConfig.Tracer = tracer
			writeTrace = true
		}

		receipt, _, err := ApplyTransaction(chainConfig, blockHashFunc, engine, nil, gp, ibs, noop, header, tx, usedGas, *vmConfig, firehoseContext)
		if writeTrace {
			if ftracer, ok := vmConfig.Tracer.(vm.FlushableTracer); ok {
				ftracer.Flush(tx)
			}

			vmConfig.Tracer = nil
		}
		if err != nil {
			if !vmConfig.StatelessExec {
				err := fmt.Errorf("could not apply tx %d from block %d [%v]: %w", i, block.NumberU64(), tx.Hash().Hex(), err)
				if firehoseContext.Enabled() {
					firehoseContext.RecordFailedTransaction(err)
					firehoseContext.ExitBlock()
				}

				return nil, err
			}

			// There is nothing to do with rejected transactions for Firehose (actually rejected transactions are not used anymore within Erigon codebase)
			rejectedTxs = append(rejectedTxs, &RejectedTx{i, err.Error()})
		} else {
			includedTxs = append(includedTxs, tx)
			if !vmConfig.NoReceipts {
				receipts = append(receipts, receipt)
			}

			if firehoseContext.Enabled() {
				firehoseContext.EndTransaction(receipt)
			}
		}
	}

	receiptSha := types.DeriveSha(receipts)
	if !vmConfig.StatelessExec && chainConfig.IsByzantium(header.Number.Uint64()) && !vmConfig.NoReceipts && receiptSha != block.ReceiptHash() {
		err := fmt.Errorf("mismatched receipt headers for block %d (%s != %s)", block.NumberU64(), receiptSha.Hex(), block.ReceiptHash().Hex())
		if firehoseContext.Enabled() {
			firehoseContext.CancelBlock(block, err)
		}

		return nil, err
	}

	if !vmConfig.StatelessExec && *usedGas != header.GasUsed {
		err := fmt.Errorf("gas used by execution: %d, in header: %d", *usedGas, header.GasUsed)
		if firehoseContext.Enabled() {
			firehoseContext.CancelBlock(block, err)
		}

		return nil, err
	}

	var bloom types.Bloom
	if !vmConfig.NoReceipts {
		bloom = types.CreateBloom(receipts)
		if !vmConfig.StatelessExec && bloom != header.Bloom {
			err := fmt.Errorf("bloom computed by execution: %x, in header: %x", bloom, header.Bloom)
			if firehoseContext.Enabled() {
				firehoseContext.CancelBlock(block, err)
			}

			return nil, err
		}
	}

	if !vmConfig.ReadOnly {
		// Finalize block is a bit special since it can be enabled without the full firehose sync.
		// As such, if firehose is enabled, we log it and us the firehose context. Otherwise if
		// block progress is enabled.
		if firehoseContext.Enabled() {
			firehoseContext.FinalizeBlock(block)
		} else if firehose.BlockProgressEnabled {
			firehose.SyncContext().FinalizeBlock(block)
		}

		txs := block.Transactions()
		if _, _, _, err := FinalizeBlockExecution(engine, stateReader, block.Header(), txs, block.Uncles(), stateWriter, chainConfig, ibs, receipts, block.Withdrawals(), epochReader, chainReader, false, firehoseContext); err != nil {
			if firehoseContext.Enabled() {
				firehoseContext.CancelBlock(block, err)
			}

			return nil, err
		}
	}

	var logs []*types.Log
	for _, receipt := range receipts {
		logs = append(logs, receipt.Logs...)
	}

	if firehoseContext.Enabled() {
		// Calculate the total difficulty of the block
		ptd := chainReader.GetTd(block.ParentHash(), block.NumberU64()-1)
		difficulty := block.Difficulty()
		if difficulty == nil {
			difficulty = big.NewInt(0)
		}

		td := ptd
		if ptd != nil {
			td = new(big.Int).Add(difficulty, ptd)
		}

		// The `finalizedBlock` argument **must** be `nil` on Bor because we still have an hard-coded -200 finalized block on Polygon
		firehoseContext.EndBlock(block, nil, td)
	}

	blockLogs := ibs.Logs()
	stateSyncReceipt := &types.Receipt{}
	if chainConfig.Consensus == chain.BorConsensus && len(blockLogs) > 0 {
		slices.SortStableFunc(blockLogs, func(i, j *types.Log) bool { return i.Index < j.Index })

		if len(blockLogs) > len(logs) {
			stateSyncReceipt.Logs = blockLogs[len(logs):] // get state-sync logs from `state.Logs()`

			// fill the state sync with the correct information
			types.DeriveFieldsForBorReceipt(stateSyncReceipt, block.Hash(), block.NumberU64(), receipts)
			stateSyncReceipt.Status = types.ReceiptStatusSuccessful
		}
	}

	execRs := &EphemeralExecResult{
		TxRoot:           types.DeriveSha(includedTxs),
		ReceiptRoot:      receiptSha,
		Bloom:            bloom,
		LogsHash:         rlpHash(blockLogs),
		Receipts:         receipts,
		Difficulty:       (*math.HexOrDecimal256)(header.Difficulty),
		GasUsed:          math.HexOrDecimal64(*usedGas),
		Rejected:         rejectedTxs,
		StateSyncReceipt: stateSyncReceipt,
	}

	return execRs, nil
}

func rlpHash(x interface{}) (h libcommon.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, x) //nolint:errcheck
	hw.Sum(h[:0])
	return h
}

func SysCallContract(contract libcommon.Address, data []byte, chainConfig chain.Config, ibs *state.IntraBlockState, header *types.Header, engine consensus.EngineReader, constCall bool, firehoseContext *firehose.Context) (result []byte, err error) {
	if chainConfig.DAOForkSupport && chainConfig.DAOForkBlock != nil && chainConfig.DAOForkBlock.Cmp(header.Number) == 0 {
		misc.ApplyDAOHardFork(ibs, firehoseContext)
	}

	msg := types.NewMessage(
		state.SystemAddress,
		&contract,
		0, u256.Num0,
		math.MaxUint64, u256.Num0,
		nil, nil,
		data, nil, false,
		true, // isFree
	)
	vmConfig := vm.Config{NoReceipts: true, RestoreState: constCall}
	// Create a new context to be used in the EVM environment
	isBor := chainConfig.Bor != nil
	var txContext evmtypes.TxContext
	var author *libcommon.Address
	if isBor {
		author = &header.Coinbase
		txContext = evmtypes.TxContext{}
	} else {
		author = &state.SystemAddress
		txContext = NewEVMTxContext(msg)
	}
	blockContext := NewEVMBlockContext(header, GetHashFn(header, nil), engine, author)
	evm := vm.NewEVM(blockContext, txContext, ibs, &chainConfig, vmConfig, firehoseContext)

	ret, _, err := evm.Call(
		vm.AccountRef(msg.From()),
		*msg.To(),
		msg.Data(),
		msg.Gas(),
		msg.Value(),
		false,
	)
	if isBor && err != nil {
		return nil, nil
	}
	return ret, err
}

// SysCreate is a special (system) contract creation methods for genesis constructors.
func SysCreate(contract libcommon.Address, data []byte, chainConfig chain.Config, ibs *state.IntraBlockState, header *types.Header, firehoseContext *firehose.Context) (result []byte, err error) {
	if chainConfig.DAOForkSupport && chainConfig.DAOForkBlock != nil && chainConfig.DAOForkBlock.Cmp(header.Number) == 0 {
		misc.ApplyDAOHardFork(ibs, firehoseContext)
	}

	msg := types.NewMessage(
		contract,
		nil, // to
		0, u256.Num0,
		math.MaxUint64, u256.Num0,
		nil, nil,
		data, nil, false,
		true, // isFree
	)
	vmConfig := vm.Config{NoReceipts: true}
	// Create a new context to be used in the EVM environment
	author := &contract
	txContext := NewEVMTxContext(msg)
	blockContext := NewEVMBlockContext(header, GetHashFn(header, nil), nil, author)
	evm := vm.NewEVM(blockContext, txContext, ibs, &chainConfig, vmConfig, firehoseContext)

	ret, _, err := evm.SysCreate(
		vm.AccountRef(msg.From()),
		msg.Data(),
		msg.Gas(),
		msg.Value(),
		contract,
	)
	return ret, err
}

func CallContract(contract libcommon.Address, data []byte, chainConfig chain.Config, ibs *state.IntraBlockState, header *types.Header, engine consensus.Engine, firehoseContext *firehose.Context) (result []byte, err error) {
	gp := new(GasPool)
	gp.AddGas(50_000_000)
	var gasUsed uint64

	if chainConfig.DAOForkSupport && chainConfig.DAOForkBlock != nil && chainConfig.DAOForkBlock.Cmp(header.Number) == 0 {
		misc.ApplyDAOHardFork(ibs, firehoseContext)
	}
	noop := state.NewNoopWriter()
	tx, err := CallContractTx(contract, data, ibs)
	if err != nil {
		return nil, fmt.Errorf("SysCallContract: %w ", err)
	}
	vmConfig := vm.Config{NoReceipts: true}
	_, result, err = ApplyTransaction(&chainConfig, GetHashFn(header, nil), engine, &state.SystemAddress, gp, ibs, noop, header, tx, &gasUsed, vmConfig, firehoseContext)
	if err != nil {
		return result, fmt.Errorf("SysCallContract: %w ", err)
	}
	return result, nil
}

// from the null sender, with 50M gas.
func CallContractTx(contract libcommon.Address, data []byte, ibs *state.IntraBlockState) (tx types.Transaction, err error) {
	from := libcommon.Address{}
	nonce := ibs.GetNonce(from)
	tx = types.NewTransaction(nonce, contract, u256.Num0, 50_000_000, u256.Num0, data)
	return tx.FakeSign(from)
}

func FinalizeBlockExecution(engine consensus.Engine, stateReader state.StateReader, header *types.Header,
	txs types.Transactions, uncles []*types.Header, stateWriter state.WriterWithChangeSets, cc *chain.Config, ibs *state.IntraBlockState,
	receipts types.Receipts, withdrawals []*types.Withdrawal, e consensus.EpochReader, headerReader consensus.ChainHeaderReader, isMining bool,
	firehoseContext *firehose.Context,
) (newBlock *types.Block, newTxs types.Transactions, newReceipt types.Receipts, err error) {
	syscall := func(contract libcommon.Address, data []byte) ([]byte, error) {
		return SysCallContract(contract, data, *cc, ibs, header, engine, false /* constCall */, firehoseContext)
	}
	if isMining {
		newBlock, newTxs, newReceipt, err = engine.FinalizeAndAssemble(cc, header, ibs, txs, uncles, receipts, withdrawals, e, headerReader, syscall, nil, firehoseContext)
	} else {
		_, _, err = engine.Finalize(cc, header, ibs, txs, uncles, receipts, withdrawals, e, headerReader, syscall, firehoseContext)
	}
	if err != nil {
		return nil, nil, nil, err
	}

	if err := ibs.CommitBlock(cc.Rules(header.Number.Uint64(), header.Time), stateWriter); err != nil {
		return nil, nil, nil, fmt.Errorf("committing block %d failed: %w", header.Number.Uint64(), err)
	}

	if err := stateWriter.WriteChangeSets(); err != nil {
		return nil, nil, nil, fmt.Errorf("writing changesets for block %d failed: %w", header.Number.Uint64(), err)
	}
	return newBlock, newTxs, newReceipt, nil
}

func InitializeBlockExecution(engine consensus.Engine, chain consensus.ChainHeaderReader, epochReader consensus.EpochReader, header *types.Header, txs types.Transactions, uncles []*types.Header, cc *chain.Config, ibs *state.IntraBlockState, firehoseContext *firehose.Context) error {
	engine.Initialize(cc, chain, epochReader, header, ibs, txs, uncles, func(contract libcommon.Address, data []byte) ([]byte, error) {
		return SysCallContract(contract, data, *cc, ibs, header, engine, false /* constCall */, firehoseContext)
	}, firehoseContext)
	noop := state.NewNoopWriter()
	ibs.FinalizeTx(cc.Rules(header.Number.Uint64(), header.Time), noop)
	return nil
}
