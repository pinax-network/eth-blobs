package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/streamingfast/cli"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/shutter"
	sink "github.com/streamingfast/substreams/sink"
	"go.uber.org/zap"

	"github.com/pinax-network/blob-service/blobs/flags"
	blobsink "github.com/pinax-network/blob-service/blobs/sink"
)

var sinkCmd = &cobra.Command{
	Use:   "sink <endpoint> <manifest>",
	Short: "Stream the eth-blobs Substreams (map_blobs) into the store",
	Long: `Streams map_blobs from a Substreams endpoint and writes one object per beacon block to the
store, plus the block-root index and this writer's head and cursor objects.

  <endpoint>  Substreams endpoint, e.g. eth-cl.substreams.pinax.network:443 (or a plaintext one with --plaintext)
  <manifest>  URL or path of the eth-blobs .spkg

--start-block and --stop-block (exclusive) bound the stream; without them the sink follows the
chain from the module's first block. --start-block applies only while this writer has no cursor
object: once it has one, the sink resumes from it. A negative --start-block counts back from the
chain head the server sees (-1 is the block below it), so a fresh writer skips history.

Several sinks (one per DC) may write the same bucket as long as each uses its own --writer:
slot and root objects are identical for a given block, and head and cursor are per writer.
Authentication uses SUBSTREAMS_API_KEY or SUBSTREAMS_API_TOKEN when the endpoint needs it.

Reorgs are handled here, not by the library: there is no --undo-buffer-size. Every block is
written as it arrives; on an undo the sink deletes the orphans it wrote and republishes its
head, and slots the stream skips (no block on the chain) have their objects deleted.`,
	Args:    sinkArgs,
	PreRunE: operatorPreRun,
	RunE:    runSink,
	Example: `  blobs sink eth-cl.substreams.pinax.network:443 \
      https://github.com/pinax-network/eth-blobs/releases/download/v0.10.0/eth-blobs-v0.10.0.spkg \
      --writer=dc1 --start-block=-1 \
      --store-url="s3://localhost:9000/blobs-eth?region=none&insecure=true" --metrics-listen-addr=:9102`,
}

func init() {
	addSinkFlags(sinkCmd.Flags())
	rootCmd.AddCommand(sinkCmd)
}

func addSinkFlags(fs *pflag.FlagSet) {
	sink.AddFlagsToSet(fs, sink.FlagExcludeDefault(
		// No undo buffer: with one, the library swallows undo signals (and a deeper fork is
		// swallowed again on restart), so orphans already written would never be cleaned up.
		sink.FlagUndoBufferSize,
		// The library would serve its own metrics endpoint (localhost:9102 by default), which
		// collides with --metrics-listen-addr; that one serves the library's metrics too.
		sink.FlagPrometheusAddr,
		// Redefined below with a longer default.
		sink.FlagMaxRetries,
	))
	// The endpoint is the first argument; the library reads it from this flag.
	_ = fs.MarkHidden(sink.FlagEndpoint)
	fs.Int(sink.FlagMaxRetries, 15, "Consecutive failed Substreams connection attempts before the sink exits (exponential backoff, about 5 minutes for 15; -1 retries forever). The count resets when data arrives")
	addStoreFlags(fs)
	fs.String("writer", "", "Writer id naming this sink's head and cursor objects, e.g. riv or kan (required)")
	fs.String("module", blobsink.OutputModuleName, "Output module to stream")
	fs.Int("batch-size", 200, "Blocks per flush while catching up; live blocks flush one by one")
	fs.Int("parallelism", 16, "Concurrent object writes within a flush")
	fs.Int("flush-retries", 5, "Attempts for each failed store write (batch, head, cursor) before the sink exits")
}

// sinkArgs takes the endpoint and the manifest. The block range used to be an
// optional third argument; say where it went instead of a bare arity error.
func sinkArgs(cmd *cobra.Command, args []string) error {
	if len(args) == 3 && strings.Contains(args[2], ":") {
		return fmt.Errorf("block range %q: the range argument was replaced by --start-block and --stop-block", args[2])
	}
	return cobra.ExactArgs(2)(cmd, args)
}

func runSink(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	app := shutter.New()

	endpoint, manifest := args[0], args[1]
	if err := cmd.Flags().Set(sink.FlagEndpoint, endpoint); err != nil {
		return fmt.Errorf("endpoint: %w", err)
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

	// The library registers its metrics (head_block_time_drift and friends, which
	// the drift alert reads) only when it serves them itself on --prometheus-addr,
	// which blobs does not register; --metrics-listen-addr serves them instead.
	sink.Metrics.Register()
	blobsink.RegisterMetrics()

	st, err := storeFromFlags(cmd)
	if err != nil {
		return err
	}

	baseSinker, err := sink.NewFromViper(cmd, blobsink.OutputModuleType,
		manifest, sflags.MustGetString(cmd, "module"), "blobs/"+flags.GetVersion(),
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
