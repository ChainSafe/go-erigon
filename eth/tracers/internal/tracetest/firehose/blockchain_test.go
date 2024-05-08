package firehose_test

import (
	"math/big"
	"testing"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/core"
	"github.com/ledgerwatch/erigon/core/state"
	"github.com/ledgerwatch/erigon/core/tracing"
	"github.com/ledgerwatch/erigon/core/types"
	"github.com/ledgerwatch/erigon/core/vm"
	"github.com/ledgerwatch/erigon/tests"
	"github.com/ledgerwatch/erigon/turbo/stages/mock"
	"github.com/ledgerwatch/log/v3"
	"github.com/stretchr/testify/require"
)

func runPrestateBlock(t *testing.T, prestatePath string, hooks *tracing.Hooks) {
	t.Helper()

	prestate := readPrestateData(t, prestatePath)
	tx, err := types.UnmarshalTransactionFromBinary(common.FromHex(prestate.Input))
	if err != nil {
		t.Fatalf("failed to parse testcase input: %v", err)
	}

	context := prestate.Context.toBlockContext(prestate.Genesis)
	rules := prestate.Genesis.Config.Rules(context.BlockNumber, context.Time)

	m := mock.Mock(t)
	dbTx, err := m.DB.BeginRw(m.Ctx)
	require.NoError(t, err)
	defer dbTx.Rollback()
	stateDB, _ := tests.MakePreState(rules, dbTx, prestate.Genesis.Alloc, context.BlockNumber)

	var logger = log.New("test")
	genesisBlock, _, err := core.GenesisToBlock(prestate.Genesis, "", logger, nil)
	require.NoError(t, err)

	block := types.NewBlock(&types.Header{
		ParentHash:            genesisBlock.Hash(),
		Number:                big.NewInt(int64(context.BlockNumber)),
		Difficulty:            context.Difficulty,
		Coinbase:              context.Coinbase,
		Time:                  context.Time,
		GasLimit:              context.GasLimit,
		BaseFee:               context.BaseFee.ToBig(),
		ParentBeaconBlockRoot: ptr(common.Hash{}),
	}, []types.Transaction{tx}, nil, nil, nil)

	stateDB.SetLogger(hooks)
	stateDB.SetTxContext(tx.Hash(), block.Hash(), 0)

	hooks.OnBlockchainInit(prestate.Genesis.Config)
	hooks.OnBlockStart(tracing.BlockEvent{
		Block: block,
		TD:    prestate.TotalDifficulty,
	})

	usedGas := uint64(0)
	usedBlobGas := uint64(0)
	_, _, err = core.ApplyTransaction(
		prestate.Genesis.Config,
		nil,
		nil,
		&context.Coinbase,
		new(core.GasPool).AddGas(block.GasLimit()),
		stateDB,
		state.NewNoopWriter(),
		block.Header(),
		tx,
		&usedGas,
		&usedBlobGas,
		vm.Config{Tracer: hooks},
	)
	require.NoError(t, err)

	hooks.OnBlockEnd(nil)
}
