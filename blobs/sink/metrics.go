package sink

import "github.com/streamingfast/dmetrics"

var metrics = dmetrics.NewSet()

// RegisterMetrics exposes the sink's own counters next to the ones the
// substreams-sink library registers (head_block_number, head_block_time_drift,
// substreams_sink_*). Call once before serving /metrics.
func RegisterMetrics() { metrics.Register() }

var (
	SlotsWritten      = metrics.NewCounter("blobs_sink_slots_written", "Slot objects written to the store")
	BlobsWritten      = metrics.NewCounter("blobs_sink_blobs_written", "Blobs contained in the slot objects written")
	BytesWritten      = metrics.NewCounter("blobs_sink_bytes_written", "Bytes of slot objects written")
	FlushCount        = metrics.NewCounter("blobs_sink_flush", "Flushes (batches of slot objects followed by a head and cursor write)")
	LastFlushDuration = metrics.NewGauge("blobs_sink_last_flush_duration_seconds", "Wall time of the last flush")
	UndoCount         = metrics.NewCounter("blobs_sink_undo", "Undo signals handled")
	SlotsDeleted      = metrics.NewCounter("blobs_sink_slots_deleted", "Slot objects deleted: missed slots the stream skipped and orphans removed on undo")
	HeadSlot          = metrics.NewGauge("blobs_sink_head_slot", "Newest slot published as this writer's head")
)
