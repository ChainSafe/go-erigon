// Copyright 2016 The go-ethereum Authors
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

package debug

import (
	"encoding/json"
	"fmt"
	"net/http"
	_ "net/http/pprof" //nolint:gosec
	"os"

	metrics2 "github.com/VictoriaMetrics/metrics"
	"github.com/ledgerwatch/erigon-lib/common/metrics"
	"github.com/ledgerwatch/erigon/common/fdlimit"
	"github.com/ledgerwatch/erigon/core"
	"github.com/ledgerwatch/erigon/firehose"
	"github.com/ledgerwatch/erigon/metrics/exp"
	"github.com/ledgerwatch/erigon/params"
	"github.com/ledgerwatch/erigon/turbo/logging"
	"github.com/ledgerwatch/log/v3"
	"github.com/spf13/cobra"
	"github.com/urfave/cli/v2"
)

var (
	//nolint
	vmoduleFlag = cli.StringFlag{
		Name:  "vmodule",
		Usage: "Per-module verbosity: comma-separated list of <pattern>=<level> (e.g. eth/*=5,p2p=4)",
		Value: "",
	}
	metricsAddrFlag = cli.StringFlag{
		Name: "metrics.addr",
	}
	metricsPortFlag = cli.UintFlag{
		Name:  "metrics.port",
		Value: 6060,
	}
	pprofFlag = cli.BoolFlag{
		Name:  "pprof",
		Usage: "Enable the pprof HTTP server",
	}
	pprofPortFlag = cli.IntFlag{
		Name:  "pprof.port",
		Usage: "pprof HTTP server listening port",
		Value: 6060,
	}
	pprofAddrFlag = cli.StringFlag{
		Name:  "pprof.addr",
		Usage: "pprof HTTP server listening interface",
		Value: "127.0.0.1",
	}
	cpuprofileFlag = cli.StringFlag{
		Name:  "pprof.cpuprofile",
		Usage: "Write CPU profile to the given file",
	}
	traceFlag = cli.StringFlag{
		Name:  "trace",
		Usage: "Write execution trace to the given file",
	}

	// Firehose Flags
	firehoseEnabledFlag = &cli.BoolFlag{
		Name:  "firehose-enabled",
		Usage: "Activate/deactivate Firehose instrumentation, disabled by default",
	}
	firehoseSyncInstrumentationFlag = &cli.BoolFlag{
		Name:  "firehose-sync-instrumentation",
		Usage: "Activate/deactivate Firehose sync output instrumentation, enabled by default",
	}
	firehoseMiningEnabledFlag = &cli.BoolFlag{
		Name:  "firehose-mining-enabled",
		Usage: "Activate/deactivate mining code even if Firehose is active, required speculative execution on local miner node, disabled by default",
	}
	firehoseBlockProgressFlag = &cli.BoolFlag{
		Name:  "firehose-block-progress",
		Usage: "Activate/deactivate Firehose block progress output instrumentation, disabled by default",
	}
	// CS TODO: this may not be required
	firehoseCompactionDisabledFlag = &cli.BoolFlag{
		Name:  "firehose-compaction-disabled",
		Usage: "Disabled database compaction, enabled by default",
	}
	// CS TODO: this may not be required
	firehoseArchiveBlocksToKeepFlag = &cli.Uint64Flag{
		Name:  "firehose-archive-blocks-to-keep",
		Usage: "Controls how many archive blocks the node should keep, this tweaks the core/blockchain.go constant value TriesInMemory, the default value of 0 can be used to use Geth default value instead which is 128",
		Value: firehose.ArchiveBlocksToKeep,
	}
	firehoseGenesisFileFlag = &cli.StringFlag{
		Name:  "firehose-genesis-file",
		Usage: "On private chains where the genesis config is not known to Geth, you **must** provide the 'genesis.json' file path for proper instrumentation of genesis block",
		Value: "",
	}
)

// Flags holds all command-line flags required for debugging.
var Flags = []cli.Flag{
	&pprofFlag, &pprofAddrFlag, &pprofPortFlag,
	&cpuprofileFlag, &traceFlag,
}

// FirehoseFlags holds all StreamingFast Firehose related command-line flags.
var FirehoseFlags = []cli.Flag{
	firehoseEnabledFlag, firehoseSyncInstrumentationFlag, firehoseMiningEnabledFlag, firehoseBlockProgressFlag,
	firehoseCompactionDisabledFlag, firehoseArchiveBlocksToKeepFlag, firehoseGenesisFileFlag,
}

func SetupCobra(cmd *cobra.Command) error {
	RaiseFdLimit()
	flags := cmd.Flags()

	_ = logging.GetLoggerCmd("debug", cmd)

	traceFile, err := flags.GetString(traceFlag.Name)
	if err != nil {
		return err
	}
	cpuFile, err := flags.GetString(cpuprofileFlag.Name)
	if err != nil {
		return err
	}

	// profiling, tracing
	if traceFile != "" {
		if err2 := Handler.StartGoTrace(traceFile); err2 != nil {
			return err2
		}
	}
	if cpuFile != "" {
		if err2 := Handler.StartCPUProfile(cpuFile); err2 != nil {
			return err2
		}
	}

	go ListenSignals(nil)
	pprof, err := flags.GetBool(pprofFlag.Name)
	if err != nil {
		return err
	}
	pprofAddr, err := flags.GetString(pprofAddrFlag.Name)
	if err != nil {
		return err
	}
	pprofPort, err := flags.GetInt(pprofPortFlag.Name)
	if err != nil {
		return err
	}

	metricsAddr, err := flags.GetString(metricsAddrFlag.Name)
	if err != nil {
		return err
	}
	metricsPort, err := flags.GetInt(metricsPortFlag.Name)
	if err != nil {
		return err
	}

	if metrics.Enabled && metricsAddr != "" {
		address := fmt.Sprintf("%s:%d", metricsAddr, metricsPort)
		exp.Setup(address)
	}

	withMetrics := metrics.Enabled && metricsAddr == ""
	if pprof {
		// metrics and pprof server
		StartPProf(fmt.Sprintf("%s:%d", pprofAddr, pprofPort), withMetrics)
	}
	return nil
}

// Setup initializes profiling and logging based on the CLI flags.
// It should be called as early as possible in the program.
func Setup(ctx *cli.Context, genesis *core.Genesis) error {
	RaiseFdLimit()

	_ = logging.GetLoggerCtx("debug", ctx)

	if traceFile := ctx.String(traceFlag.Name); traceFile != "" {
		if err := Handler.StartGoTrace(traceFile); err != nil {
			return err
		}
	}

	if cpuFile := ctx.String(cpuprofileFlag.Name); cpuFile != "" {
		if err := Handler.StartCPUProfile(cpuFile); err != nil {
			return err
		}
	}
	pprofEnabled := ctx.Bool(pprofFlag.Name)
	metricsAddr := ctx.String(metricsAddrFlag.Name)

	if metrics.Enabled && (!pprofEnabled || metricsAddr != "") {
		metricsPort := ctx.Int(metricsPortFlag.Name)
		address := fmt.Sprintf("%s:%d", metricsAddr, metricsPort)
		exp.Setup(address)
	}

	// pprof server
	if pprofEnabled {
		pprofHost := ctx.String(pprofAddrFlag.Name)
		pprofPort := ctx.Int(pprofPortFlag.Name)
		address := fmt.Sprintf("%s:%d", pprofHost, pprofPort)
		// This context value ("metrics.addr") represents the utils.MetricsHTTPFlag.Name.
		// It cannot be imported because it will cause a cyclical dependency.
		withMetrics := metrics.Enabled && metricsAddr == ""
		StartPProf(address, withMetrics)
	}

	// Firehose
	log.Info("Initializing firehose")
	if ctx.IsSet(firehoseEnabledFlag.Name) {
		firehose.Enabled = ctx.Bool(firehoseEnabledFlag.Name)
	}
	if ctx.IsSet(firehoseSyncInstrumentationFlag.Name) {
		firehose.SyncInstrumentationEnabled = ctx.Bool(firehoseSyncInstrumentationFlag.Name)
	}
	if ctx.IsSet(firehoseMiningEnabledFlag.Name) {
		firehose.MiningEnabled = ctx.Bool(firehoseMiningEnabledFlag.Name)
	}
	if ctx.IsSet(firehoseBlockProgressFlag.Name) {
		firehose.BlockProgressEnabled = ctx.Bool(firehoseBlockProgressFlag.Name)
	}
	if ctx.IsSet(firehoseCompactionDisabledFlag.Name) {
		firehose.CompactionDisabled = ctx.Bool(firehoseCompactionDisabledFlag.Name)
	}
	if ctx.IsSet(firehoseArchiveBlocksToKeepFlag.Name) {
		firehose.ArchiveBlocksToKeep = ctx.Uint64(firehoseArchiveBlocksToKeepFlag.Name)
	}

	genesisProvenance := "unset"

	if genesis != nil {
		firehose.GenesisConfig = genesis
		genesisProvenance = "Geth Specific Flag"
	} else {
		if genesisFilePath := ctx.String(firehoseGenesisFileFlag.Name); genesisFilePath != "" {
			file, err := os.Open(genesisFilePath)
			if err != nil {
				return fmt.Errorf("firehose open genesis file: %w", err)
			}
			defer file.Close()

			genesis := &core.Genesis{}
			if err := json.NewDecoder(file).Decode(genesis); err != nil {
				return fmt.Errorf("decode genesis file %q: %w", genesisFilePath, err)
			}

			firehose.GenesisConfig = genesis
			genesisProvenance = "Flag " + firehoseGenesisFileFlag.Name
		} else {
			firehose.GenesisConfig = core.DefaultGenesisBlock()
			genesisProvenance = "Geth Default"
		}
	}

	log.Info("Firehose initialized",
		"enabled", firehose.Enabled,
		"sync_instrumentation_enabled", firehose.SyncInstrumentationEnabled,
		"mining_enabled", firehose.MiningEnabled,
		"block_progress_enabled", firehose.BlockProgressEnabled,
		"compaction_disabled", firehose.CompactionDisabled,
		"archive_blocks_to_keep", firehose.ArchiveBlocksToKeep,
		"genesis_provenance", genesisProvenance,
		"firehose_version", params.FirehoseVersion(),
		"erigon_version", params.VersionWithMeta,
		"chain_variant", params.Variant,
	)

	return nil
}

func StartPProf(address string, withMetrics bool) {
	// Hook go-metrics into expvar on any /debug/metrics request, load all vars
	// from the registry into expvar, and execute regular expvar handler.
	if withMetrics {
		http.HandleFunc("/debug/metrics/prometheus", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			metrics2.WritePrometheus(w, true)
		})
	}
	cpuMsg := fmt.Sprintf("go tool pprof -lines -http=: http://%s/%s", address, "debug/pprof/profile?seconds=20")
	heapMsg := fmt.Sprintf("go tool pprof -lines -http=: http://%s/%s", address, "debug/pprof/heap")
	log.Info("Starting pprof server", "cpu", cpuMsg, "heap", heapMsg)
	go func() {
		if err := http.ListenAndServe(address, nil); err != nil { // nolint:gosec
			log.Error("Failure in running pprof server", "err", err)
		}
	}()
}

// Exit stops all running profiles, flushing their output to the
// respective file.
func Exit() {
	_ = Handler.StopCPUProfile()
	_ = Handler.StopGoTrace()
}

// RaiseFdLimit raises out the number of allowed file handles per process
func RaiseFdLimit() {
	limit, err := fdlimit.Maximum()
	if err != nil {
		log.Error("Failed to retrieve file descriptor allowance", "err", err)
		return
	}
	if _, err = fdlimit.Raise(uint64(limit)); err != nil {
		log.Error("Failed to raise file descriptor allowance", "err", err)
	}
}
