package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/streamingfast/cli/sflags"
)

// The sink handles reorgs itself and must see every undo signal. With the
// library's undo buffer the signals are swallowed (and a deep fork is
// swallowed again on restart), so the flag must stay unregistered.
func TestSinkHasNoUndoBuffer(t *testing.T) {
	if f := sinkCmd.Flags().Lookup("undo-buffer-size"); f != nil {
		t.Fatalf("--undo-buffer-size must not be registered on blobs sink (default %q)", f.DefValue)
	}
	if f := sinkCmd.Flags().Lookup("cleanup-window"); f != nil {
		t.Fatal("--cleanup-window was removed with the startup cleanup")
	}
}

// The library serves its own metrics when --prometheus-addr is registered
// (default localhost:9102), colliding with --metrics-listen-addr.
func TestSinkLibraryFlags(t *testing.T) {
	if f := sinkCmd.Flags().Lookup("prometheus-addr"); f != nil {
		t.Fatalf("--prometheus-addr must not be registered on blobs sink (default %q)", f.DefValue)
	}
	if f := sinkCmd.Flags().Lookup("max-retries"); f == nil || f.DefValue != "15" {
		t.Fatalf("--max-retries must default to 15, got %v", f)
	}
	for _, name := range []string{"endpoint", "start-block", "stop-block"} {
		if sinkCmd.Flags().Lookup(name) == nil {
			t.Fatalf("--%s must be registered: the library reads it", name)
		}
	}
}

// A negative start block is sent as is; the server resolves it against the
// chain head. Every spelling must reach the flag rather than be taken for
// shorthand flags.
func TestSinkNegativeStartBlock(t *testing.T) {
	for _, argv := range [][]string{
		{"--start-block=-1"},
		{"--start-block", "-1"},
		{"-s", "-1"},
		{"eth-cl.example:443", "blobs.spkg", "-s", "-1", "--writer", "dc1"},
	} {
		cmd := &cobra.Command{Use: "sink"}
		addSinkFlags(cmd.Flags())
		if err := cmd.ParseFlags(argv); err != nil {
			t.Fatalf("%q: %v", argv, err)
		}
		if got := sflags.MustGetString(cmd, "start-block"); got != "-1" {
			t.Fatalf("%q: start-block = %q, want -1", argv, got)
		}
	}
}

func TestSinkRangeArgumentPointsToFlags(t *testing.T) {
	err := sinkArgs(sinkCmd, []string{"eth-cl.example:443", "blobs.spkg", "15600000:"})
	if err == nil || !strings.Contains(err.Error(), "--start-block") {
		t.Fatalf("a range argument must point to --start-block, got %v", err)
	}
	if err := sinkArgs(sinkCmd, []string{"eth-cl.example:443", "blobs.spkg"}); err != nil {
		t.Fatal(err)
	}
}
