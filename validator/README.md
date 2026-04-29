# Blob Validator

A TypeScript validator that verifies blobs stored in the blob-service against the Ethereum execution layer.

## How It Works

1. **Query Execution Layer**: Fetches blob transactions from EL RPC and extracts `blobVersionedHashes`
2. **Calculate Slot**: Maps EL block number to CL slot number using timestamp
3. **Fetch from Blob Service**: Retrieves blobs from the blob-service API
4. **Calculate Hashes**: Computes versioned hashes from KZG commitments
5. **Compare**: Validates that all expected blobs are present and match

## Installation

```bash
cd validator
bun install
```

## Usage

```bash
bun validate <chain> [start_block] [blocks_to_validate]
```

### Arguments

- **chain**: `eth`, `sepolia`, `holesky`, `gnosis`, `chiado` (default: `eth`)
- **start_block**: Block number to start from, or negative offset from latest (default: chain-specific)
- **blocks_to_validate**: Number of blocks to validate (default: `100`)

### Examples

```bash
# Validate 100 blocks on Ethereum from default start
bun validate eth

# Validate 50 blocks starting from block 24986100
bun validate eth 24986100 50

# Validate last 100 blocks (negative offset from latest)
bun validate eth -100

# Validate last 50 blocks
bun validate eth -50 50

# Validate 200 blocks on Sepolia
bun validate sepolia

# Validate 10 blocks on Gnosis starting from block 30000000
bun validate gnosis 30000000 10
```

## Output

The validator provides detailed output with progress tracking:

```
� Blob Validator
   Chain:          Ethereum Mainnet
   Execution RPC:  http://eth.rpcx.riv-prod1.pinax.io
   Blob Service:   https://eth.blobs.pinax.network
   Block Range:    24986100 - 24986109
   Blocks:         10

� Validating blocks 24986100 to 24986109

[1/10] 📦 Block 24986100 (slot 14220497):
   Blob transactions: 1
   ✅ Tx 0xbb6a...: 5 blobs OK
   Total: 5 | Validated: 5 | Failed: 0

[2/10] 📦 Block 24986101 (slot 14220498):
   Blob transactions: 1
   ✅ Tx 0x3c68...: 3 blobs OK
   Total: 3 | Validated: 3 | Failed: 0

============================================================
📊 VALIDATION SUMMARY
============================================================
Blocks checked:        10
Blocks with blobs:     8
Total blobs:           42
✅ Validated blobs:    42
❌ Failed blobs:       0
============================================================

✅ All blobs validated successfully!
```

## How Versioned Hashes Are Calculated

Per EIP-4844, the versioned hash is calculated as:

```typescript
versioned_hash = VERSIONED_HASH_VERSION_KZG | sha256(kzg_commitment)[1:]
```

Where:
- `VERSIONED_HASH_VERSION_KZG = 0x01`
- The first byte of SHA256(commitment) is replaced with `0x01`

## Exit Codes

- `0`: All blobs validated successfully
- `1`: Validation failed (missing blobs, mismatched blobs, or block not found)

The validator exits immediately on first failure for fast feedback.

## Integration with CI/CD

```yaml
# .github/workflows/validate-blobs.yml
name: Validate Blobs

on:
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: oven-sh/setup-bun@v1

      - name: Install dependencies
        run: |
          cd validator
          bun install

      - name: Validate last 100 blocks
        run: |
          cd validator
          bun validate eth -100
```

## Troubleshooting

### "Block not found"
The validator will exit immediately with an error if a block is not found. This usually means:
- Your execution RPC is not synced to that block yet
- The block number is beyond the chain tip (use negative offset: `-100` instead)
- The block range is invalid

### "Slot not found" (404 from blob service)
- The slot may not have any blobs (validator will skip it)
- The blob service may not have indexed that slot yet
- Blobs older than ~18 days may not be available unless archived

### "Failed blobs"
This indicates a data integrity issue:
- Verify the blob service is using the correct substreams
- Check for database corruption
- Ensure the blob service and RPC are on the same chain

## Supported Chains

- **eth**: Ethereum Mainnet
- **sepolia**: Sepolia Testnet
- **holesky**: Holesky Testnet
- **gnosis**: Gnosis Chain
- **chiado**: Chiado Testnet

All chains use Pinax RPC and blob service endpoints automatically.

## License

Same as parent project
