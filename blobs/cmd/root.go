// Package cmd wires the blobs binary: one image, three jobs.
//
//	blobs api         serve the Beacon blob endpoints from the store
//	blobs sink        stream map_blobs into the store
//	blobs migrate-kv  copy slots from a substreams-sink-kv serve into the store
package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/streamingfast/cli/sflags"
	"github.com/streamingfast/dmetrics"
	"github.com/streamingfast/logging"
	"go.uber.org/zap"

	"github.com/pinax-network/blob-service/blobs/flags"
	"github.com/pinax-network/blob-service/blobs/store"
)

var zlog, tracer = logging.PackageLogger("blobs", "github.com/pinax-network/blob-service/blobs")

var rootCmd = &cobra.Command{
	Use:   "blobs",
	Short: "Ethereum blobs: Substreams sink, Beacon-compatible API and migration tooling",
	Long: `One binary with three jobs:

  blobs api         serve /eth/v1/beacon/blob_sidecars and /eth/v1/beacon/blobs from the store
  blobs sink        stream the eth-blobs Substreams (map_blobs) into an S3-compatible store
  blobs migrate-kv  copy slots out of a substreams-sink-kv serve endpoint into the store

The store is any S3-compatible bucket (AWS, Ceph RGW, MinIO, R2). Credentials come from the
standard AWS environment (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY or a profile).`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       flags.GetVersion(),
}

func init() {
	rootCmd.PersistentFlags().String("log-format", "console", "Log format for the sink and migration commands: console or json")
	rootCmd.PersistentFlags().String("metrics-listen-addr", "", "[Operator] Serve Prometheus metrics on this address (sink and migrate-kv; the API has its own /metrics)")
	rootCmd.PersistentFlags().String("pprof-listen-addr", "", "[Operator] Serve pprof on this address")
	rootCmd.PersistentFlags().Duration("delay-before-start", 0, "[Operator] Wait this long before doing anything, to allow maintenance on a fresh pod")
}

// Execute runs the CLI and exits non-zero on error.
func Execute() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// operatorPreRun sets up logging, metrics and pprof for the long-running
// non-API commands. The API keeps the golang-base logger and gin exporter.
func operatorPreRun(cmd *cobra.Command, _ []string) error {
	options := []logging.InstantiateOption{
		logging.WithDefaultLevel(zap.InfoLevel),
		logging.WithConsoleToStderr(),
	}
	if sflags.MustGetString(cmd, "log-format") == "json" {
		options = append(options, logging.WithProductionLogger())
	}
	logging.InstantiateLoggers(options...)

	if delay := sflags.MustGetDuration(cmd, "delay-before-start"); delay > 0 {
		zlog.Info("sleeping before start", zap.Duration("delay", delay))
		time.Sleep(delay)
	}
	if addr := sflags.MustGetString(cmd, "metrics-listen-addr"); addr != "" {
		zlog.Info("serving prometheus metrics", zap.String("listen_addr", addr))
		go dmetrics.Serve(addr)
	}
	if addr := sflags.MustGetString(cmd, "pprof-listen-addr"); addr != "" {
		go func() {
			zlog.Info("serving pprof", zap.String("listen_addr", addr))
			if err := http.ListenAndServe(addr, nil); err != nil {
				zlog.Warn("pprof server stopped", zap.Error(err))
			}
		}()
	}
	return nil
}

func addStoreFlags(fs *pflag.FlagSet) {
	fs.String("s3-endpoint", "", "S3 endpoint URL, e.g. http://localhost:9000 (empty means AWS)")
	fs.String("s3-bucket", "", "Bucket holding the blobs objects (required)")
	fs.String("s3-prefix", "", "Key prefix inside the bucket, e.g. eth/ when several chains share one bucket")
	fs.String("s3-region", "", "Region to sign requests for (default us-east-1, which is what Ceph RGW and MinIO expect)")
	fs.Bool("s3-path-style", true, "Use path-style addressing (required by Ceph RGW and MinIO)")
	fs.Duration("s3-timeout", 30*time.Second, "Per-request timeout for store operations")
}

func storeFromFlags(ctx context.Context, cmd *cobra.Command) (store.Store, error) {
	cfg := store.S3Config{
		Endpoint:  sflags.MustGetString(cmd, "s3-endpoint"),
		Bucket:    sflags.MustGetString(cmd, "s3-bucket"),
		Prefix:    sflags.MustGetString(cmd, "s3-prefix"),
		Region:    sflags.MustGetString(cmd, "s3-region"),
		PathStyle: sflags.MustGetBool(cmd, "s3-path-style"),
		Timeout:   sflags.MustGetDuration(cmd, "s3-timeout"),
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("--s3-bucket is required")
	}
	zlog.Info("connecting to store", zap.String("endpoint", cfg.Endpoint), zap.String("bucket", cfg.Bucket), zap.String("prefix", cfg.Prefix))
	return store.NewS3(ctx, cfg)
}
