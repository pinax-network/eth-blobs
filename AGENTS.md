# AGENTS.md — operating this repo

**Canonical operating knowledge for agents and contributors working in `blob-service`.**
`README.md` and `blobs/README.md` are thin orientation layers; when they disagree with this
file, this file wins.

Ethereum consensus-layer blobs (EIP-4844) as a service: a Substreams package extracts every
beacon block's blobs, a sink writes them to an S3-compatible bucket, and a Beacon-compatible
REST API serves them as a drop-in replacement for a consensus client's blob endpoints, with
history far beyond the 18 days a beacon node keeps. Deployment lives in
[`k8s-blobs`](https://github.com/pinax-network/k8s-blobs).

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
| `blobs sink <endpoint> <spkg> [<start>:<stop>]` | stream `map_blobs` into the bucket |
| `blobs api --config config.yaml` | serve `/eth/v1/beacon/blob_sidecars/{id}`, `/eth/v1/beacon/blobs/{id}`, `/health` |
| `blobs migrate-kv <kv-serve-addr> [<from>:<to>]` | copy slots out of a legacy `substreams-sink-kv serve` |

Packages: `store/` (object layout, S3 client, in-memory test store), `sink/` (the handler on top
of the substreams-sink library), `migrate/`, `cmd/` (cobra), and the API packages
`controllers/`, `services/`, `dto/`, `server/`, `config/`, `swagger/`.

### Object layout and invariants

Keys inside a bucket, optionally under a prefix:

```text
slots/<slot>.pb     serialized pinax.ethereum.blobs.v1.Slot, one per beacon block
roots/0x<root>      decimal slot number of the block with that root
head/<writer>       JSON {slot, root, timestamp, writer}: newest slot <writer> fully wrote
cursor/<writer>     the writer's opaque Substreams cursor
```

These rules are what make the design safe; do not weaken them:

1. **A slot object is the raw `map_blobs` output, untouched.** It is byte-identical to the value
   the legacy KV sink stored under `slot:<n>`, which is why `migrate-kv` copies without
   transforming and why writers can overwrite each other harmlessly. The API decodes it.
2. **Every beacon block gets a slot object, blobs or not.** Beacon API semantics: `200` with an
   empty list for a block without blobs, `404` only for an unknown block.
3. **Write order within a flush is slots and roots, then head, then cursor.** A reader must never
   see a head or cursor that is ahead of the data.
4. **Several writers may share a bucket; each has a unique `--writer`.** Slot and root objects
   are deterministic per block; head and cursor are per writer. The API takes the highest head
   across writers. Two writers with the same id corrupt each other's cursor.
5. **`START_BLOCK` applies only when the writer has no cursor object.** With a cursor the sink
   resumes from it, whatever the block range says. Deleting `cursor/<writer>` is how you restart a
   writer from the range.
6. **Undo:** the sink runs with `--undo-buffer-size=1` in production (one slot of delay; the
   library default is 12), so undo signals only come from deeper reorgs. On undo it drops pending
   blocks above the last valid one, deletes slot objects above it (at least `--cleanup-window`
   of them), republishes the head at the last valid block and advances the cursor. Root objects
   are left in place: the API checks that a slot fetched by root still carries that root and
   answers `404` otherwise.
7. **Startup cleanup:** slot objects within `--cleanup-window` above the cursor are deleted, in
   case a crash fell between the slot writes of a flush and its cursor write and a reorg has
   since emptied one of those slots. The stream rewrites the rest.
8. **A failed batch write is retried whole, then the sink exits.** Every write is idempotent.

### Library gotchas

- `github.com/streamingfast/substreams-sink` is replaced by the Pinax fork
  `github.com/pinax-network/substreams-sink-go` in `go.mod`; keep the `replace`.
- On a bounded run the library streams up to the stop block **plus the undo buffer**, so a
  `-s A -t B` run writes slightly past B. Production runs are unbounded.
- `migrate-kv` skips slots already in the store unless `--overwrite`, treats a KV `NotFound` as a
  missed slot, and publishes `head/<writer>` only when `--writer` is given. Omit `--writer` when
  backfilling below a live writer's head. KV keys are `slot:<n>` without padding, so it reads
  per key rather than scanning.
- The AWS SDK is set to compute and validate checksums only when required; Ceph RGW does not
  return them and the SDK otherwise logs a warning per GET.
- The API resolves `block_id` as `head`, a slot number or a `0x` block root. `genesis`,
  `finalized`, `justified` and SSZ responses are not implemented. `finalized` in responses is a
  heuristic (`head - slot >= chain.finality_lag`), not a consensus finality check.

## Build, test, run

```bash
cd blobs
task build             # ./blobs, static, version from the git tag or 'untagged'
task test              # go test ./... (store test runs against an in-process fake S3)
task test:format       # gofmt check, same as CI
golangci-lint run ./...
task generate:go       # swagger from the annotations in cmd/api.go — commit the result
task protogen          # Go bindings from ../proto
```

Point the store test at a real endpoint with `BLOBS_S3_TEST_ENDPOINT=http://host:9000` and AWS
credentials in the environment. For a full local run, start any S3 (MinIO, or gofakes3 as in
`store/s3_test.go`), create a bucket, and stream a short range from the public endpoint:

```bash
SUBSTREAMS_API_KEY=$PINAX_KEY AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... \
  ./blobs sink eth-cl.substreams.pinax.network:443 ../substreams/eth-blobs-v0.10.0.spkg 15600000:15600020 \
  --writer dev --batch-size 5 --s3-endpoint http://localhost:9000 --s3-bucket blobs-eth
./blobs api --config config.yaml      # store section pointing at the same bucket
curl localhost:8080/eth/v1/beacon/blob_sidecars/15600010
```

In-mesh tier1 endpoints (`<chain>-cl.substreams.<zone>-prod-firehose-a.pinax.io:80`) need
`--plaintext` and no key; the public ones need `SUBSTREAMS_API_KEY`.

## CI, images, releases

| Workflow | Runs on | Does |
|---|---|---|
| `go.yml` | every push and PR | `task build`, `task test` in `blobs/` |
| `golangci-lint.yaml` | PRs to `main` | `task test:format` and golangci-lint |
| `docker.yml` | push to `main` touching `blobs/**` or `proto/**`, tags `v*`, manual dispatch | `task build:docker` |

Image tags: a `v*` git tag yields `eth-blobs:vX.Y.Z` **and moves `latest`**; any other build is
tagged with the short commit. For a trial build of a branch use `gh workflow run docker.yml
--ref <branch>` and pin the resulting short-commit tag in k8s-blobs. Releasing is tagging `main`
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
| `k8s-blobs` | the deployment: sink and api tiers, buckets, alerts, dashboards, cutover runbook |
| `k8s-pinax-fleet` | which clusters run the blobs tenant and from which branch |
| `firehose-beacon` | the beacon Firehose and the `beacon` Substreams import |
| `opstrom` | the monitoring agent; components `blobs_inject` (the write path) and `blobs_api` |
| `opstrom-viz` | `topology/blobs.yaml`, the drawn topology of this stack |
