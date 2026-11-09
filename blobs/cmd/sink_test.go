package cmd

import (
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

// The range argument keeps its meaning on top of the library's flags.
func TestSinkBlockRangeArgument(t *testing.T) {
	for _, tc := range []struct{ arg, start, stop string }{
		{"15600000:", "15600000", "0"},
		{"15600000:15600020", "15600000", "15600020"},
		{"15600000:+20", "15600000", "+20"},
		{":15600020", "", "15600020"},
		{"15600020", "", "15600020"},
		{":", "", "0"},
	} {
		cmd := &cobra.Command{Use: "sink"}
		addSinkFlags(cmd.Flags())
		if err := setBlockRange(cmd.Flags(), tc.arg); err != nil {
			t.Fatalf("%q: %v", tc.arg, err)
		}
		start, stop := sflags.MustGetString(cmd, "start-block"), sflags.MustGetString(cmd, "stop-block")
		if start != tc.start || stop != tc.stop {
			t.Fatalf("%q: start-block %q stop-block %q, want %q %q", tc.arg, start, stop, tc.start, tc.stop)
		}
	}
}
