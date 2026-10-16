# Ethereum Blobs Subgraph

### Running locally

- Set up `$SUBSTREAMS_ENDPOINT` and `$SUBSTREAMS_API_TOKEN` env variables

- Start Graph node, IPFS node and Postgres database in Docker:
```bash
> cd ./graph-node
> ./up.sh -c        # use -c flag to cleanup database and start from scratch
```

- Build, create and deploy the subgraph
```bash
> bun run build
> bun run create-local
> bun run deploy-local
```

- Query: http://localhost:8000/subgraphs/name/blobs/graphql