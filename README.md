# Ethereum Blobs
This repository contains several components:
- `proto` - Blobs protobuf definitions
- `substreams` - Substreams modules to stream blobs data from substreams-enabled Consensus Layer Firehose
- `subgraph` - Substreams-based Subgraph that uses substreams module
- `api-service` - REST API service to access to blobs via substreams sink that can be used as a drop-in replacement for Consensus Layer clients' blob_sidecar API

## Substreams
For source of block data, you need to have access to consensus layer substreams-enabled firehose endpoint for your chain, powered by [Beacon Firehose](https://github.com/pinax-network/firehose-beacon).

You can either run your own stack or use endpoints provided by [Pinax](https://pinax.network/).

Currently, the following chains are supported:
- Ethereum: `eth-cl.substreams.pinax.network:443`
- Holesky: `holesky-cl.substreams.pinax.network:443`
- Sepolia: `sepolia-cl.substreams.pinax.network:443`
- Goerli: `goerli-cl.substreams.pinax.network:443`
- Gnosis: `gnosis-cl.substreams.pinax.network:443`
- Chiado: `chiado-cl.substreams.pinax.network:443`