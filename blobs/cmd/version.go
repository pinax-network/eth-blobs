package cmd

import (
	"github.com/spf13/cobra"

	"github.com/pinax-network/blob-service/blobs/flags"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the build version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		cmd.Println(flags.GetVersion())
	},
}

func init() { rootCmd.AddCommand(versionCmd) }
