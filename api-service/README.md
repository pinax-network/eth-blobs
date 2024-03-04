# Blobs REST API Service
This service provides REST API access to Ethereum blobs via KV sink backend.
To run it you need to have KV sink backend running and consuming [blobs substreams](../substreams) locally or on the server

### Quick Start

- From source:
```bash
> task protogen         # generate Go protobuf bindings
> task generate:go      # generate swagger docs
> task start:service    # build and start service
```

- Or, with Docker:
```bash
> task start:docker
```


### Query
```bash
> curl -v http://localhost:8080/eth/v1/beacon/blob_sidecars/7677000
> curl -v http://localhost:8080/health
```