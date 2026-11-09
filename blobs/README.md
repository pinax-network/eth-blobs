# blobs

One Go binary with three jobs:

| Command | What it does |
|---|---|
| `blobs api` | Serves the Beacon-compatible blob endpoints from the store |
| `blobs sink` | Streams the eth-blobs Substreams (`map_blobs`) into the store |
| `blobs migrate-kv` | Copies slots out of a `substreams-sink-kv serve` endpoint into the store |

The store is a [dstore](https://github.com/streamingfast/dstore) URL, so it can be any
S3-compatible bucket (Ceph RGW, MinIO, AWS, R2), Google Cloud Storage, Azure Blob Storage or a
local directory:

| Store | `--store-url` / `store.url` |
|---|---|
| S3-compatible endpoint | `s3://localhost:9000/blobs-eth?region=none&insecure=true` (drop `insecure` for https) |
| AWS S3 | `s3://blobs-eth?region=us-east-1` |
| Google Cloud Storage | `gs://blobs-eth` |
| Azure Blob Storage | `az://account.container/blobs-eth` (a prefix is required; see below) |
| Local directory | `file:///var/lib/blobs/eth` (sink and API must share the filesystem) |

Append a path to use a prefix inside the bucket (`s3://host:9000/blobs/eth?...`), which lets
several chains share one. For Azure the prefix is mandatory: dstore's backend lists a container
root under the wrong prefix, so the API would never find a head. The tests cover the S3 and local
backends; GCS and Azure are untested here. Credentials come from each backend's standard environment
(`AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY`, `GOOGLE_APPLICATION_CREDENTIALS`, ...), never from a
config file. For `s3://` the binary defaults `AWS_REQUEST_CHECKSUM_CALCULATION` and
`AWS_RESPONSE_CHECKSUM_VALIDATION` to `when_required`, which S3-compatible servers need.

## Object layout

```
slots/<slot>.pb.zst zstd frame of the serialized pinax.ethereum.blobs.v1.Slot, one per beacon block (also for blocks without blobs)
roots/0x<root>      decimal slot number of the block with that root
head/<writer>       JSON {slot, root, timestamp, writer}: the newest slot <writer> has fully written
cursor/<writer>     the writer's opaque Substreams cursor
```

Several sinks (one per DC) may write the same bucket as long as each has its own `--writer`:
slot and root objects are identical for a given block, so overwrites are harmless, and head and
cursor are per writer. The API takes the highest head across writers. A slot fetched through a
block root must still carry that root, so a root index entry left behind by a reorg resolves to
`404`.

## Build and test

```bash
task build            # ./blobs
task test             # go test ./...
task generate:go      # regenerate swagger docs from the api annotations
task protogen         # regenerate Go protobuf bindings from ../proto
```

The store test runs against a local directory and an in-process fake S3; set
`BLOBS_TEST_STORE_URL` (and the backend's credentials) to also run it against a real store.

## Run

```bash
# API
cp config.example.yaml config.yaml   # point store.url at the sink's store
AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... ./blobs api --config config.yaml
curl -s localhost:8080/health
curl -s localhost:8080/eth/v1/beacon/blob_sidecars/head | head -c 300

# Sink, from a recent slot to the head and on (SUBSTREAMS_API_KEY for the public endpoint)
./blobs sink eth-cl.substreams.pinax.network:443 \
    https://github.com/pinax-network/eth-blobs/releases/download/v0.10.0/eth-blobs-v0.10.0.spkg 15600000: \
    --writer=dc1 \
    --store-url="s3://localhost:9000/blobs-eth?region=none&insecure=true" --metrics-listen-addr=:9102

# One-off migration out of the existing KV store, then start the sink from <to>+1
./blobs migrate-kv blobs-serve:8000 8626176: --workers=32 --writer=migration \
    --store-url="s3://localhost:9000/blobs-eth?region=none&insecure=true"
```

Sink behaviour worth knowing:

- While catching up, blocks are written in batches of `--batch-size` with `--parallelism`
  concurrent object writes; at the head every block is written as it arrives. Each flush writes
  slots and roots, then the head, then the cursor, so a reader never sees a head or cursor ahead
  of the data.
- Reorgs are handled by the sink, not by the library's undo buffer (there is no
  `--undo-buffer-size`). When the stream skips slots (no block on the chain), their objects are
  deleted in the same flush, which removes orphans and crash leftovers. On an undo the sink drops
  pending blocks above the last valid one, deletes the orphans it wrote that still hold its block
  (never a block another writer rewrote), republishes its head and saves the undo's cursor.
- Startup deletes nothing, so restarting one writer never removes blocks another writer wrote.
- Each failed store write (the batch, then the head, then the cursor) is retried `--flush-retries`
  times with backoff before the sink exits;
  every write is idempotent so a retry is always safe.

Metrics: the substreams-sink library's `head_block_number`, `head_block_time_drift` and
`substreams_sink_*`, plus `blobs_sink_slots_written`, `blobs_sink_blobs_written`,
`blobs_sink_bytes_written`, `blobs_sink_flush`, `blobs_sink_last_flush_duration_seconds`,
`blobs_sink_undo`, `blobs_sink_slots_deleted` and `blobs_sink_head_slot`. The API exposes gin
request metrics on `/metrics`.
