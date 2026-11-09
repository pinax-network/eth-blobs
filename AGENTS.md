# AGENTS.md — operating this repo

**Canonical operating knowledge for agents and contributors working in `blob-service`.**
`README.md` and `blobs/README.md` are thin orientation layers; when they disagree with this
file, this file wins.

Ethereum consensus-layer blobs (EIP-4844) as a service: a Substreams package extracts every
beacon block's blobs, a sink writes them to an object store (S3-compatible, GCS, Azure or a local
directory), and a Beacon-compatible
REST API serves them as a drop-in replacement for a consensus client's blob endpoints, with
history far beyond the 18 days a beacon node keeps. Pinax runs it as `<chain>.blobs.pinax.network`;
the deployment manifests live in a private repository and are out of scope here.

## Discovery paths

| Path | Role |
|---|---|
| `AGENTS.md` | **canonical** instructions — author here |
| `CLAUDE.md → AGENTS.md` | symlink; Claude Code's native instruction filename |

Claude Code reads `CLAUDE.md`, not `AGENTS.md`, and has no setting to redirect — keep the symlink.

## Layout

```text
proto/        pinax.ethereum.blobs.v1 (Slot, Blob) and the legacy sink-kv protos
substreams/   Rust package `eth-blobs`: map_blobs (the product), graph_out (subgraph), kv_out (legacy)
blobs/        Go module, ONE binary `blobs`: `api`, `sink`, `migrate-kv`
subgraph/     Substreams-based subgraph over graph_out
validator/    bun CLI that checks the API against execution and consensus clients
```

The GitHub repo is `pinax-network/eth-blobs` (the clone may be named `blob-service`); the Go
module is `github.com/pinax-network/blob-service/blobs`; the image is
`ghcr.io/pinax-network/eth-blobs`.

## The `blobs` binary

| Command | Job |
|---|---|
| `blobs sink <endpoint> <spkg> [<start>:<stop>]` | stream `map_blobs` into the store |
| `blobs api --config config.yaml` | serve `/eth/v1/beacon/blob_sidecars/{id}`, `/eth/v1/beacon/blobs/{id}`, `/health` |
| `blobs migrate-kv <kv-serve-addr> [<from>:<to>]` | copy slots out of a legacy `substreams-sink-kv serve` |

Packages: `store/` (object layout on top of dstore, in-memory test store), `sink/` (the handler on top
of the substreams-sink library), `migrate/`, `cmd/` (cobra), and the API packages
`controllers/`, `services/`, `dto/`, `server/`, `config/`, `swagger/`.

### Object layout and invariants

The store is a [dstore](https://github.com/streamingfast/dstore) URL (`--store-url` on `sink` and
`migrate-kv`, `store.url` in the API config): `s3://host:port/bucket[/prefix]?region=none&insecure=true`
for an S3-compatible endpoint, `s3://bucket?region=us-east-1` for AWS, `gs://bucket`,
`az://account.container`, or `file:///path` / a plain path for a local directory. Credentials come
from each backend's standard environment, never from flags or config.

Keys inside the store URL (a bucket or directory, optionally with a prefix):

```text
slots/<slot>.pb     zstd frame of the serialized pinax.ethereum.blobs.v1.Slot, one per beacon block
roots/0x<root>      decimal slot number of the block with that root
head/<writer>       JSON {slot, root, timestamp, writer}: newest slot <writer> fully wrote
cursor/<writer>     the writer's opaque Substreams cursor
```

These rules are what make the design safe; do not weaken them:

1. **A slot object is the raw `map_blobs` output, zstd-compressed and otherwise untouched.**
   Decompressed it is byte-identical to the value the legacy KV sink stored under `slot:<n>`,
   which is why `migrate-kv` copies without transforming and why writers can overwrite each other
   harmlessly. Only slot objects are compressed (`store/slotcodec.go`, fastest level: blobs are
   often zero-padded, ~20% saved on mainnet, and higher levels gain nothing). Objects written
   before compression are raw; `store.DecodeSlot` tells them apart by the zstd frame header
   (`28 b5 2f fd`), which a serialized `Slot` can never start with. The API decodes the `Slot`.
2. **Every beacon block gets a slot object, blobs or not.** Beacon API semantics: `200` with an
   empty list for a block without blobs, `404` only for an unknown block.
3. **Write order within a flush is slots, roots and missed-slot deletes, then head, then the
   cursor of the last block in the flush.** A reader must never see a head or cursor that is
   ahead of the data, and a cursor never moves past a delete that has not happened.
4. **Several writers may share a bucket; each has a unique `--writer`.** Slot and root objects
   are deterministic per block; head and cursor are per writer. The API takes the highest head
   across writers. Two writers with the same id corrupt each other's cursor.
5. **`START_BLOCK` applies only when the writer has no cursor object.** With a cursor the sink
   resumes from it, whatever the block range says. Deleting `cursor/<writer>` is how you restart a
   writer from the range.
6. **No library undo buffer; the sink handles every undo.** `blobs sink` does not register
   `--undo-buffer-size`. With a buffer the library swallows undo signals, a fork deeper than the
   buffer kills the stream, and on restart the empty buffer swallows the server's undo again, so
   orphans already written would never be cleaned up. Every block is written as it arrives.
7. **Deletes never touch a valid block, from any writer.** Only two deletes exist:
   - *Missed slots:* when the stream delivers block S right after block P (or after the cursor's
     block on startup, or the last valid block of an undo), the slots in (P, S) hold no block on
     that chain, so their objects are deleted in S's flush. This removes orphans the new chain has
     no block for and leftovers of a crash, including those written before a restart.
   - *Own orphans on undo:* slots this process wrote above the last valid block (it tracks its
     non-final writes) are deleted only if they still hold the block it wrote; a slot another
     writer already refilled with the new chain's block stays. Orphans the new chain has a block
     for are overwritten by the stream.

   On undo the sink also drops pending blocks above the last valid one, republishes its head
   there and saves the undo's cursor. Startup deletes nothing. Root objects are left in place:
   the API checks that a slot fetched by root still carries that root and answers `404`
   otherwise. Between an undo and the new chain's next block, a slot query can still return an
   orphan another writer has not cleaned yet (seconds).
8. **A failed batch write is retried whole, then the sink exits.** Every write is idempotent.

### Library gotchas

- `github.com/streamingfast/substreams-sink` is replaced by the Pinax fork
  `github.com/pinax-network/substreams-sink-go` in `go.mod`; keep the `replace`.
- The library's range-completion callback passes the last cursor *received*; the sink ignores it
  and saves the cursor of the last block it wrote. A bounded `A:B` run writes slots A..B-1.
  Production runs are unbounded.
- `migrate-kv` does not recopy slots already in the store unless `--overwrite`, but checks and
  repairs their root object and counts them towards the head; it treats a KV `NotFound` as a
  missed slot, and publishes `head/<writer>` only when `--writer` is given. Omit `--writer` when
  backfilling below a live writer's head. KV keys are `slot:<n>` without padding, so it reads
  per key rather than scanning.
- dstore is used with no extension, no compression and overwrite on (`store.Open`). Keep
  overwrite on: with it off, dstore silently skips writes to existing keys, so heads and cursors
  would stop advancing. Each dstore write does a HEAD before the PUT, and deletes go one object at a
  time (run 16 in parallel).
- dstore's `memory://` backend cannot list, so it cannot serve heads; `store.Open` rejects it and
  tests use `store.NewMemStore`. The local backend writes through `<name>.<random>.tmp` and renames;
  `GetHead` skips `.tmp` names and writer ids may not end in `.tmp`.
- For `s3://` URLs `store.Open` defaults `AWS_REQUEST_CHECKSUM_CALCULATION` and
  `AWS_RESPONSE_CHECKSUM_VALIDATION` to `when_required` unless already set. Ceph RGW and MinIO do not
  return the SDK's default checksums (it would warn on every GET), and older S3-compatible servers
  reject its trailing PUT checksums. dstore builds its client from the default AWS config, which
  reads these variables.
- `HexBytes.MarshalJSON` writes `"0x<hex>"` straight into one buffer. Encoding blobs to JSON is
  where the API spends its CPU on full `blob_sidecars` responses; do not route it back through
  `hex.EncodeToString` + `json.Marshal`.
- The API resolves `block_id` as `head`, a slot number or a `0x` block root. `genesis`,
  `finalized`, `justified` and SSZ responses are not implemented. `finalized` in responses is a
  heuristic (`head - slot >= chain.finality_lag`), not a consensus finality check.

## Build, test, run

```bash
cd blobs
task build             # ./blobs, static, version from the git tag or 'untagged'
task test              # go test ./... (store test runs against a temp dir and an in-process fake S3)
task test:format       # gofmt check, same as CI
golangci-lint run ./...
task generate:go       # swagger from the annotations in cmd/api.go — commit the result
task protogen          # Go bindings from ../proto
```

Add a real store to the store test with `BLOBS_TEST_STORE_URL=s3://host:9000/bucket?region=none&insecure=true`
and the backend's credentials in the environment. A full local run needs no S3 at all: stream a
short range from the public endpoint into a directory and serve it.

```bash
SUBSTREAMS_API_KEY=<your key> \
  ./blobs sink eth-cl.substreams.pinax.network:443 ../substreams/eth-blobs-v0.10.0.spkg 15600000:15600020 \
  --writer dev --batch-size 5 --store-url ./data/blobs-eth
./blobs api --config config.yaml      # store.url: ./data/blobs-eth
curl localhost:8080/eth/v1/beacon/blob_sidecars/15600010
```

Public Substreams endpoints need `SUBSTREAMS_API_KEY`; a self-hosted plaintext endpoint needs
`--plaintext` instead.

## CI, images, releases

| Workflow | Runs on | Does |
|---|---|---|
| `go.yml` | every push and PR | `task build`, `task test` in `blobs/` |
| `golangci-lint.yaml` | PRs to `main` | `task test:format` and golangci-lint |
| `docker.yml` | push to `main` touching `blobs/**` or `proto/**`, tags `v*`, manual dispatch | `task build:docker` |

Image tags: a `v*` git tag yields `eth-blobs:vX.Y.Z` **and moves `latest`**; any other build is
tagged with the short commit. For a trial build of a branch use `gh workflow run docker.yml
--ref <branch>` and deploy the resulting short-commit tag. Releasing is tagging `main`
with `v*`; the Substreams package has its own release line (`eth-blobs-vX.Y.Z.spkg` on GitHub
releases), bumped only when the Rust modules change.

The Substreams package: changing `map_blobs` output changes the module hash and needs a new
package version. `kv_out` only exists for the legacy KV deployment; `graph_out` feeds the
subgraph. New consensus specs (Deneb, Electra, Fusaka, Gloas, …) need the `Spec` enum and
`map_blobs` match arms extended and a new `firehose-beacon` import.

## Validating data

`validator/` compares the service with the chain: `bun validate cl <chain> <slot> <count>` pulls
the same slots from a beacon node, `bun validate el <chain> <block> <count>` derives expected
versioned hashes from execution-layer blob transactions. The target URLs are hardcoded in
`validator/src`; edit `blobServiceUrl` to point at a stage API. Use it on a random sample of
historical slots after any backfill and on the most recent few hundred after any sink change.

## Related repositories

| Repo | Owns |
|---|---|
| [`firehose-beacon`](https://github.com/pinax-network/firehose-beacon) | the beacon Firehose and the `beacon` Substreams import |
| [`substreams`](https://github.com/streamingfast/substreams) | the Substreams engine and CLI |
| [`substreams-sink-kv`](https://github.com/streamingfast/substreams-sink-kv) | the legacy KV sink `migrate-kv` reads from |
