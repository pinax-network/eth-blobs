package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/streamingfast/cli/sflags"
	"go.uber.org/zap"

	"github.com/pinax-network/blob-service/blobs/migrate"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate-kv <kv-serve-addr> [<from>:<to>]",
	Short: "Copy slots from a substreams-sink-kv serve endpoint into the store",
	Long: `Reads slot:<n> values from a running substreams-sink-kv serve gRPC endpoint and writes them
as slot objects, byte for byte, together with the root index. Existing slots are not copied
again, but their root index is checked and repaired, so an interrupted run can be rerun on the
same range. <to> defaults to the KV store's head.

With --writer the highest slot of the range present in the store (copied or already there) is
published as that writer's head, so the API can serve right away; start the live sink
afterwards with a start block of <to>+1.`,
	Args:    cobra.RangeArgs(1, 2),
	PreRunE: operatorPreRun,
	Example: `  blobs migrate-kv blobs-serve:8000 8626176: --workers=32 --writer=migration \
      --store-url="s3://localhost:9000/blobs-eth?region=none&insecure=true"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := migrate.Config{
			KVAddr:    args[0],
			Workers:   sflags.MustGetInt(cmd, "workers"),
			Writer:    sflags.MustGetString(cmd, "writer"),
			Overwrite: sflags.MustGetBool(cmd, "overwrite"),
			Retries:   sflags.MustGetInt(cmd, "retries"),
		}
		if len(args) == 2 {
			var err error
			cfg.From, cfg.To, err = parseRange(args[1])
			if err != nil {
				return err
			}
		}
		st, err := storeFromFlags(cmd)
		if err != nil {
			return err
		}
		report, err := migrate.Run(ctx, cfg, st, zlog)
		if err != nil {
			return err
		}
		zlog.Info("migration finished",
			zap.Uint64("copied", report.Copied), zap.Uint64("skipped", report.Skipped), zap.Uint64("repaired", report.Repaired), zap.Uint64("missing", report.Missing),
			zap.Uint64("blobs", report.Blobs), zap.Uint64("bytes", report.Bytes),
			zap.Uint64("high_slot", report.HighSlot), zap.Duration("took", report.Duration),
		)
		return nil
	},
}

func init() {
	fs := migrateCmd.Flags()
	addStoreFlags(fs)
	fs.Int("workers", 16, "Concurrent slot copies")
	fs.String("writer", "", "Publish head/<writer> for the highest copied slot")
	fs.Bool("overwrite", false, "Rewrite slots that already exist in the store")
	fs.Int("retries", migrate.DefaultRetries, "Attempts per KV read or store read/write before failing; with the default --store-timeout this rides out a ~5 minute store stall")
	rootCmd.AddCommand(migrateCmd)
}

// parseRange accepts "from:to", "from:" and ":to" with inclusive bounds.
func parseRange(s string) (from, to uint64, err error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range %q: want <from>:<to>", s)
	}
	if parts[0] != "" {
		if from, err = strconv.ParseUint(parts[0], 10, 64); err != nil {
			return 0, 0, fmt.Errorf("invalid from slot %q: %w", parts[0], err)
		}
	}
	if parts[1] != "" {
		if to, err = strconv.ParseUint(parts[1], 10, 64); err != nil {
			return 0, 0, fmt.Errorf("invalid to slot %q: %w", parts[1], err)
		}
	}
	return from, to, nil
}
