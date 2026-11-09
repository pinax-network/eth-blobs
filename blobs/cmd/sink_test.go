package cmd

import "testing"

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
