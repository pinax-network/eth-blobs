package cmd

import (
	"sync"

	"github.com/spf13/cobra"
	"github.com/streamingfast/cli/sflags"

	"github.com/pinax-network/blob-service/blobs/server"
	_ "github.com/pinax-network/blob-service/blobs/swagger"
)

//go:generate swag init -d ../ -g cmd/api.go --output ../swagger --parseDependency --parseDepth 3

//	@title			Ethereum Blobs REST API
//	@version		1.0
//	@description	Use this API to get EIP-4844 blobs as a drop-in replacement for Consensus Layer clients API.

//	@host		localhost:8080
//	@schemes	https http

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Serve the Beacon-compatible blobs REST API from the store",
	Long: `Serves GET /eth/v1/beacon/blob_sidecars/{block_id}, GET /eth/v1/beacon/blobs/{block_id},
/health, /metrics and /swagger. Configuration is a YAML file (see config.example.yaml); the
store section points at the bucket the sink writes, credentials come from the AWS environment.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		app, err := server.Initialize(cmd.Context(), sflags.MustGetString(cmd, "config"), sflags.MustGetBool(cmd, "debug"))
		if err != nil {
			return err
		}
		httpServer := server.SetupHttpServer(app)
		wg := &sync.WaitGroup{}
		wg.Add(1)
		go httpServer.Run(wg)
		wg.Wait()
		return nil
	},
}

func init() {
	apiCmd.Flags().String("config", "./config.yaml", "Config file path")
	apiCmd.Flags().Bool("debug", false, "Debug logging and gin debug mode")
	rootCmd.AddCommand(apiCmd)
}
