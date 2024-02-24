# Ethereum Consensus Layer Blob Service

### Build substreams and start KV sink
```bash
$ cd substreams
$ task protogen # if needed
$ task build
$ task sink
```

### Start Blobs service
```bash
$ task protogen
$ task build
$ task start:service
```

### Query
```bash
$ curl -v http://localhost:8080/blobs/by_slot/7677000
```