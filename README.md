# Ethereum Consensus Layer Blob Service

### From Source

- Build substreams and start KV sink
```bash
$ cd substreams
$ task protogen # if needed
$ task sink
```

- Start Blobs service
```bash
$ task protogen
$ task generate:go
$ task start:service
```

### Or, with Docker:

- Start KV sink
```bash
$ cd substreams
$ task start:docker`
```

- Start Blob service
```bash
$ task start:docker`
```


### Query
```bash
$ curl -v http://localhost:8080/eth/v1/beacon/blob_sidecars/7677000
```