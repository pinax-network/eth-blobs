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
$ task start:service
```

### Query
```bash
$ curl -v http://localhost:8080/v1/blobs/by_slot/7677000
```