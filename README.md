# Ethereum Consensus Layer Blob Service

### Build substreams and start KV sink
```bash
$ cd substreams
$ task protogen # if needed
$ task sink
```

### Start Blobs service
```bash
$ task protogen
$ task generate:go
$ task start:service
```

### Query
```bash
$ curl -v http://localhost:8080/eth/v1/beacon/blob_sidecars/7677000
```