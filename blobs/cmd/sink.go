package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/shutter"
	sink "github.com/streamingfast/substreams-sink"
	"go.uber.org/zap"

	blobsink "github.com/pinax-network/blob-service/blobs/sink"
)

var sinkCmd = &cobra.Command{
	Use:   "sink <endpoint> <manifest> [<start>:<stop>]",
	Short: "Stream the eth-blobs Substreams (map_blobs) into the store",
	Long: `Streams map_blobs from a Substreams endpoint and writes one object per beacon block to the
store, plus the block-root index and this writer's head and cursor objects.

  <endpoint>      Substreams endpoint, e.g. eth-cl.substreams.pinax.network:443 (or a plaintext one with --plaintext)
  <manifest>      URL or path of the eth-blobs .spkg
  <start>:<stop>  Optional block range (absolute slots, <stop> exclusive); omit to follow the chain.

Several sinks (one per DC) may write the same bucket as long as each uses its own --writer:
slot and root objects are identical for a given block, and head and cursor are per writer.
Authentication uses SUBSTREAMS_API_TOKEN when the endpoint needs it.

Reorgs are handled here, not by the library: there is no --undo-buffer-size. Every block is
written as it arrives; on an undo the sink deletes the orphans it wrote and republishes its
head, and slots the stream skips (no block on the chain) have their objects deleted.`,
	Args:    cobra.RangeArgs(2, 3),
	PreRunE: operatorPreRun,
	RunE:    runSink,
	Example: `  blobs sink eth-cl.substreams.pinax.network:443 \
      https://github.com/pinax-network/eth-blobs/releases/download/v0.10.0/eth-blobs-v0.10.0.spkg \
      --writer=dc1 \
      --store-url="s3://localhost:9000/blobs-eth?region=none&insecure=true" --metrics-listen-addr=:9102`,
}

func init() {
	fs := sinkCmd.Flags()
	// No undo buffer: with one, the library swallows undo signals (and a deeper fork is
	// swallowed again on restart), so orphans already written would never be cleaned up.
	sink.AddFlagsToSet(fs, sink.FlagIgnore(sink.FlagUndoBufferSize))
	addStoreFlags(fs)
	fs.String("writer", "", "Writer id naming this sink's head and cursor objects, e.g. riv or kan (required)")
	fs.String("module", blobsink.OutputModuleName, "Output module to stream")
	fs.Int("batch-size", 200, "Blocks per flush while catching up; live blocks flush one by one")
	fs.Int("parallelism", 16, "Concurrent object writes within a flush")
	fs.Int("flush-retries", 5, "Attempts for each failed store write (batch, head, cursor) before the sink exits")
	rootCmd.AddCommand(sinkCmd)
}

func runSink(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	app := shutter.New()

	endpoint, manifest := args[0], args[1]
	blockRange := ""
	if len(args) == 3 {
		blockRange = args[2]
	}

	cfg := blobsink.Config{
		Writer:       sflags.MustGetString(cmd, "writer"),
		BatchSize:    sflags.MustGetInt(cmd, "batch-size"),
		Parallelism:  sflags.MustGetInt(cmd, "parallelism"),
		FlushRetries: sflags.MustGetInt(cmd, "flush-retries"),
	}
	if cfg.Writer == "" {
		return fmt.Errorf("--writer is required")
	}

	sink.RegisterMetrics()
	blobsink.RegisterMetrics()

	st, err := storeFromFlags(cmd)
	if err != nil {
		return err
	}

	baseSinker, err := sink.NewFromViper(cmd, blobsink.OutputModuleType,
		endpoint, manifest, sflags.MustGetString(cmd, "module"), blockRange,
		zlog, tracer,
	)
	if err != nil {
		return fmt.Errorf("setting up substreams sink: %w", err)
	}

	sinker, err := blobsink.New(baseSinker, st, cfg, zlog)
	if err != nil {
		return err
	}

	sinker.OnTerminating(app.Shutdown)
	app.OnTerminating(sinker.Shutdown)
	go sinker.Run(ctx)

	signalHandler, isSignaled, _ := cli.SetupSignalHandler(0*time.Second, zlog)
	select {
	case <-signalHandler:
		go app.Shutdown(nil)
	case <-app.Terminating():
		zlog.Info("sink terminating", zap.Bool("from_signal", isSignaled.Load()), zap.Bool("with_error", app.Err() != nil))
	}

	select {
	case <-app.Terminated():
	case <-time.After(30 * time.Second):
		zlog.Warn("sink did not terminate within 30s")
	}
	if err := app.Err(); err != nil {
		return err
	}
	zlog.Info("sink terminated gracefully")
	return nil
}
