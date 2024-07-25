// Copyright 2024 The go-ethereum Authors
// (original work)
// Copyright 2024 The Erigon Authors
// (modifications)
// This file is part of Erigon.
//
// Erigon is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// Erigon is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with Erigon. If not, see <http://www.gnu.org/licenses/>.

package tracing

import (
	"math/big"

	libcommon "github.com/erigontech/erigon-lib/common"

	"github.com/erigontech/erigon-lib/chain"
	"github.com/erigontech/erigon/core/types"

	"github.com/holiman/uint256"
)

// OpContext provides the context at which the opcode is being
// executed in, including the memory, stack and various contract-level information.
type OpContext interface {
	MemoryData() []byte
	StackData() []uint256.Int
	Caller() libcommon.Address
	Address() libcommon.Address
	CallValue() *uint256.Int
	CallInput() []byte
	Code() []byte
	CodeHash() libcommon.Hash
}

// IntraBlockState gives tracers access to the whole state.
type IntraBlockState interface {
	GetBalance(libcommon.Address) *uint256.Int
	GetNonce(libcommon.Address) uint64
	GetCode(libcommon.Address) []byte
	GetState(addr libcommon.Address, key *libcommon.Hash, value *uint256.Int)
	Exist(libcommon.Address) bool
	GetRefund() uint64
}

// VMContext provides the context for the EVM execution.
type VMContext struct {
	Coinbase    libcommon.Address
	BlockNumber uint64
	Time        uint64
	Random      *libcommon.Hash
	// Effective txn gas price
	GasPrice        *uint256.Int
	ChainConfig     *chain.Config
	IntraBlockState IntraBlockState

	TxHash libcommon.Hash
}

// BlockEvent is emitted upon tracing an incoming block.
// It contains the block as well as consensus related information.
type BlockEvent struct {
	Block     *types.Block
	TD        *big.Int
	Finalized *types.Header
	Safe      *types.Header
}

type (
	/*
		- VM events -
	*/

	// TxStartHook is called before the execution of a transaction starts.
	// Call simulations don't come with a valid signature. `from` field
	// to be used for address of the caller.
	TxStartHook = func(vm *VMContext, txn types.Transaction, from libcommon.Address)

	// TxEndHook is called after the execution of a transaction ends.
	TxEndHook = func(receipt *types.Receipt, err error)

	// EnterHook is invoked when the processing of a message starts.
	EnterHook = func(depth int, typ byte, from libcommon.Address, to libcommon.Address, precompile bool, input []byte, gas uint64, value *uint256.Int, code []byte)

	// ExitHook is invoked when the processing of a message ends.
	// `revert` is true when there was an error during the execution.
	// Exceptionally, before the homestead hardfork a contract creation that
	// ran out of gas when attempting to persist the code to database did not
	// count as a call failure and did not cause a revert of the call. This will
	// be indicated by `reverted == false` and `err == ErrCodeStoreOutOfGas`.
	ExitHook = func(depth int, output []byte, gasUsed uint64, err error, reverted bool)

	// OpcodeHook is invoked just prior to the execution of an opcode.
	OpcodeHook = func(pc uint64, op byte, gas, cost uint64, scope OpContext, rData []byte, depth int, err error)

	// FaultHook is invoked when an error occurs during the execution of an opcode.
	FaultHook = func(pc uint64, op byte, gas, cost uint64, scope OpContext, depth int, err error)

	// GasChangeHook is invoked when the gas changes.
	GasChangeHook = func(old, new uint64, reason GasChangeReason)

	/*
		- Chain events -
	*/

	// BlockchainInitHook is called when the blockchain is initialized.
	BlockchainInitHook = func(chainConfig *chain.Config)

	// BlockStartHook is called before executing `block`.
	// `td` is the total difficulty prior to `block`.
	BlockStartHook = func(event BlockEvent)

	// BlockEndHook is called after executing a block.
	BlockEndHook = func(err error)

	// GenesisBlockHook is called when the genesis block is being processed.
	GenesisBlockHook = func(genesis *types.Block, alloc types.GenesisAlloc)

	// OnSystemCallStartHook is called when a system call is about to be executed. Today,
	// this hook is invoked when the EIP-4788 system call is about to be executed to set the
	// beacon block root.
	//
	// After this hook, the EVM call tracing will happened as usual so you will receive a `OnEnter/OnExit`
	// as well as state hooks between this hook and the `OnSystemCallEndHook`.
	//
	// Note that system call happens outside normal transaction execution, so the `OnTxStart/OnTxEnd` hooks
	// will not be invoked.
	OnSystemCallStartHook = func()

	// OnSystemCallEndHook is called when a system call has finished executing. Today,
	// this hook is invoked when the EIP-4788 system call is about to be executed to set the
	// beacon block root.
	OnSystemCallEndHook = func()

	/*
		- State events -
	*/

	// BalanceChangeHook is called when the balance of an account changes.
	BalanceChangeHook = func(addr libcommon.Address, prev, new *uint256.Int, reason BalanceChangeReason)

	// NonceChangeHook is called when the nonce of an account changes.
	NonceChangeHook = func(addr libcommon.Address, prev, new uint64)

	// CodeChangeHook is called when the code of an account changes.
	CodeChangeHook = func(addr libcommon.Address, prevCodeHash libcommon.Hash, prevCode []byte, codeHash libcommon.Hash, code []byte)

	// StorageChangeHook is called when the storage of an account changes.
	StorageChangeHook = func(addr libcommon.Address, slot *libcommon.Hash, prev, new uint256.Int)

	// LogHook is called when a log is emitted.
	LogHook = func(log *types.Log)
)

type Hooks struct {
	// VM events
	OnTxStart   TxStartHook
	OnTxEnd     TxEndHook
	OnEnter     EnterHook
	OnExit      ExitHook
	OnOpcode    OpcodeHook
	OnFault     FaultHook
	OnGasChange GasChangeHook
	// Chain events
	OnBlockchainInit  BlockchainInitHook
	OnBlockStart      BlockStartHook
	OnBlockEnd        BlockEndHook
	OnGenesisBlock    GenesisBlockHook
	OnSystemCallStart OnSystemCallStartHook
	OnSystemCallEnd   OnSystemCallEndHook
	// State events
	OnBalanceChange BalanceChangeHook
	OnNonceChange   NonceChangeHook
	OnCodeChange    CodeChangeHook
	OnStorageChange StorageChangeHook
	OnLog           LogHook

	// Firehose backward compatibility
	// This hook exist because some current Firehose supported chains requires it
	// but this field is going to be deprecated and newer chains will not produced
	// those events anymore. The hook is registered conditionally based on the
	// tracer configuration.
	OnNewAccount func(address libcommon.Address, previousExisted bool)
}

// BalanceChangeReason is used to indicate the reason for a balance change, useful
// for tracing and reporting.
type BalanceChangeReason byte

//go:generate stringer -type=BalanceChangeReason -output gen_balance_change_reason_stringer.go

const (
	BalanceChangeUnspecified BalanceChangeReason = 0

	// Issuance
	// BalanceIncreaseRewardMineUncle is a reward for mining an uncle block.
	BalanceIncreaseRewardMineUncle BalanceChangeReason = 1
	// BalanceIncreaseRewardMineBlock is a reward for mining a block.
	BalanceIncreaseRewardMineBlock BalanceChangeReason = 2
	// BalanceIncreaseWithdrawal is ether withdrawn from the beacon chain.
	BalanceIncreaseWithdrawal BalanceChangeReason = 3
	// BalanceIncreaseGenesisBalance is ether allocated at the genesis block.
	BalanceIncreaseGenesisBalance BalanceChangeReason = 4

	// Transaction fees
	// BalanceIncreaseRewardTransactionFee is the transaction tip increasing block builder's balance.
	BalanceIncreaseRewardTransactionFee BalanceChangeReason = 5
	// BalanceDecreaseGasBuy is spent to purchase gas for execution a transaction.
	// Part of this gas will be burnt as per EIP-1559 rules.
	BalanceDecreaseGasBuy BalanceChangeReason = 6
	// BalanceIncreaseGasReturn is ether returned for unused gas at the end of execution.
	BalanceIncreaseGasReturn BalanceChangeReason = 7

	// DAO fork
	// BalanceIncreaseDaoContract is ether sent to the DAO refund contract.
	BalanceIncreaseDaoContract BalanceChangeReason = 8
	// BalanceDecreaseDaoAccount is ether taken from a DAO account to be moved to the refund contract.
	BalanceDecreaseDaoAccount BalanceChangeReason = 9

	// BalanceChangeTransfer is ether transferred via a call.
	// it is a decrease for the sender and an increase for the recipient.
	BalanceChangeTransfer BalanceChangeReason = 10
	// BalanceChangeTouchAccount is a transfer of zero value. It is only there to
	// touch-create an account.
	BalanceChangeTouchAccount BalanceChangeReason = 11

	// BalanceIncreaseSelfdestruct is added to the recipient as indicated by a selfdestructing account.
	BalanceIncreaseSelfdestruct BalanceChangeReason = 12
	// BalanceDecreaseSelfdestruct is deducted from a contract due to self-destruct.
	BalanceDecreaseSelfdestruct BalanceChangeReason = 13
	// BalanceDecreaseSelfdestructBurn is ether that is sent to an already self-destructed
	// account within the same txn (captured at end of tx).
	// Note it doesn't account for a self-destruct which appoints itself as recipient.
	BalanceDecreaseSelfdestructBurn BalanceChangeReason = 14
)

// GasChangeReason is used to indicate the reason for a gas change, useful
// for tracing and reporting.
//
// There is essentially two types of gas changes, those that can be emitted once per transaction
// and those that can be emitted on a call basis, so possibly multiple times per transaction.
//
// They can be recognized easily by their name, those that start with `GasChangeTx` are emitted
// once per transaction, while those that start with `GasChangeCall` are emitted on a call basis.
type GasChangeReason byte

//go:generate stringer -type=GasChangeReason -output gen_gas_change_reason_stringer.go

const (
	GasChangeUnspecified GasChangeReason = 0

	// GasChangeTxInitialBalance is the initial balance for the call which will be equal to the gasLimit of the call. There is only
	// one such gas change per transaction.
	GasChangeTxInitialBalance GasChangeReason = 1
	// GasChangeTxIntrinsicGas is the amount of gas that will be charged for the intrinsic cost of the transaction, there is
	// always exactly one of those per transaction.
	GasChangeTxIntrinsicGas GasChangeReason = 2
	// GasChangeTxRefunds is the sum of all refunds which happened during the txn execution (e.g. storage slot being cleared)
	// this generates an increase in gas. There is at most one of such gas change per transaction.
	GasChangeTxRefunds GasChangeReason = 3
	// GasChangeTxLeftOverReturned is the amount of gas left over at the end of transaction's execution that will be returned
	// to the chain. This change will always be a negative change as we "drain" left over gas towards 0. If there was no gas
	// left at the end of execution, no such even will be emitted. The returned gas's value in Wei is returned to caller.
	// There is at most one of such gas change per transaction.
	GasChangeTxLeftOverReturned GasChangeReason = 4

	// GasChangeCallInitialBalance is the initial balance for the call which will be equal to the gasLimit of the call. There is only
	// one such gas change per call.
	GasChangeCallInitialBalance GasChangeReason = 5
	// GasChangeCallLeftOverReturned is the amount of gas left over that will be returned to the caller, this change will always
	// be a negative change as we "drain" left over gas towards 0. If there was no gas left at the end of execution, no such even
	// will be emitted.
	GasChangeCallLeftOverReturned GasChangeReason = 6
	// GasChangeCallLeftOverRefunded is the amount of gas that will be refunded to the call after the child call execution it
	// executed completed. This value is always positive as we are giving gas back to the you, the left over gas of the child.
	// If there was no gas left to be refunded, no such even will be emitted.
	GasChangeCallLeftOverRefunded GasChangeReason = 7
	// GasChangeCallContractCreation is the amount of gas that will be burned for a CREATE.
	GasChangeCallContractCreation GasChangeReason = 8
	// GasChangeContractCreation is the amount of gas that will be burned for a CREATE2.
	GasChangeCallContractCreation2 GasChangeReason = 9
	// GasChangeCallCodeStorage is the amount of gas that will be charged for code storage.
	GasChangeCallCodeStorage GasChangeReason = 10
	// GasChangeCallOpCode is the amount of gas that will be charged for an opcode executed by the EVM, exact opcode that was
	// performed can be check by `OnOpcode` handling.
	GasChangeCallOpCode GasChangeReason = 11
	// GasChangeCallPrecompiledContract is the amount of gas that will be charged for a precompiled contract execution.
	GasChangeCallPrecompiledContract GasChangeReason = 12
	// GasChangeCallStorageColdAccess is the amount of gas that will be charged for a cold storage access as controlled by EIP2929 rules.
	GasChangeCallStorageColdAccess GasChangeReason = 13
	// GasChangeCallFailedExecution is the burning of the remaining gas when the execution failed without a revert.
	GasChangeCallFailedExecution GasChangeReason = 14

	// GasChangeIgnored is a special value that can be used to indicate that the gas change should be ignored as
	// it will be "manually" tracked by a direct emit of the gas change event.
	GasChangeIgnored GasChangeReason = 0xFF
)
