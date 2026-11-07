# Ethereum Blobs
This repository contains several components:
- `proto` - Blobs protobuf definitions
- `substreams` - Substreams modules to stream blobs data from substreams-enabled Consensus Layer Firehose
- `subgraph` - Substreams-based Subgraph that uses substreams module
- `blobs` - one binary: the Substreams sink writing blobs to an S3-compatible bucket, the REST API serving them as a drop-in replacement for Consensus Layer clients' blob APIs, and a migration tool for the legacy KV store
- `validator` - CLI tool to validate blobs data against execution and consensus layer

## Substreams
For source of block data, you need to have access to consensus layer substreams-enabled firehose endpoint for your chain, powered by [Beacon Firehose](https://github.com/pinax-network/firehose-beacon).

You can either run your own stack or use endpoints provided by [Pinax](https://pinax.network/).

Currently, the following chains are supported:
- Ethereum: `eth-cl.substreams.pinax.network:443`
- Sepolia: `sepolia-cl.substreams.pinax.network:443`
- Gnosis: `gnosis-cl.substreams.pinax.network:443`
- Hoodi: `hoodi-cl.substreams.pinax.network:443`
