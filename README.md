# Ethereum Blobs Monorepo
This repository contains several components:
- `proto` - Blobs protobuf definitions
- `substreams` - Substreams modules to stream blobs data from substreams-enabled Consensus Layer Firehose
- `subgraph` - Substreams-based Subgraph that uses substreams module
- `api-service` - REST API service to access to blobs via substreams sink that can be used as a drop-in replacement for Consensus Layer clients' blob_sidecar API
