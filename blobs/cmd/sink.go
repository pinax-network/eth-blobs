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

  <endpoint>      Substreams tier1, e.g. eth-cl.substreams.riv-prod-firehose-a.pinax.io:80 (use --plaintext)
  <manifest>      URL or path of the eth-blobs .spkg
  <start>:<stop>  Optional block range; omit to follow the chain. Negative start is relative to head.

Several sinks (one per DC) may write the same bucket as long as each uses its own --writer:
slot and root objects are identical for a given block, and head and cursor are per writer.
Authentication uses SUBSTREAMS_API_TOKEN when the endpoint needs it.`,
	Args:    cobra.RangeArgs(2, 3),
	PreRunE: operatorPreRun,
	RunE:    runSink,
	Example: `  blobs sink eth-cl.substreams.riv-prod-firehose-a.pinax.io:80 \
      https://github.com/pinax-network/eth-blobs/releases/download/v0.10.0/eth-blobs-v0.10.0.spkg \
      --plaintext --undo-buffer-size=1 --writer=riv \
      --s3-endpoint=http://rgw.internal --s3-bucket=blobs-eth --metrics-listen-addr=:9102`,
}

func init() {
	fs := sinkCmd.Flags()
	sink.AddFlagsToSet(fs)
	addStoreFlags(fs)
	fs.String("writer", "", "Writer id naming this sink's head and cursor objects, e.g. riv or kan (required)")
	fs.String("module", blobsink.OutputModuleName, "Output module to stream")
	fs.Int("batch-size", 200, "Blocks per flush while catching up; live blocks flush one by one")
	fs.Int("parallelism", 16, "Concurrent object writes within a flush")
	fs.Uint64("cleanup-window", 64, "Slots above an undo's last valid block (or the cursor on startup) whose slot objects are deleted")
	fs.Int("flush-retries", 5, "Attempts for a failed batch write before the sink exits")
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
		Writer:        sflags.MustGetString(cmd, "writer"),
		BatchSize:     sflags.MustGetInt(cmd, "batch-size"),
		Parallelism:   sflags.MustGetInt(cmd, "parallelism"),
		CleanupWindow: sflags.MustGetUint64(cmd, "cleanup-window"),
		FlushRetries:  sflags.MustGetInt(cmd, "flush-retries"),
	}
	if cfg.Writer == "" {
		return fmt.Errorf("--writer is required")
	}

	sink.RegisterMetrics()
	blobsink.RegisterMetrics()

	st, err := storeFromFlags(ctx, cmd)
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
