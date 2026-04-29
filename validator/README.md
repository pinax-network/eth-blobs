# Blob Validator

A TypeScript validator that verifies blobs stored in the blob-service against either the Ethereum execution layer (EL) or consensus layer (CL).

## Validation Modes

### Execution Layer (EL) Mode
1. **Query Execution Layer**: Fetches blob transactions from EL RPC and extracts `blobVersionedHashes`
2. **Calculate Slot**: Maps EL block number to CL slot number using timestamp
3. **Fetch from Blob Service**: Retrieves blobs from the blob-service API
4. **Calculate Hashes**: Computes versioned hashes from KZG commitments
5. **Compare**: Validates that all expected blobs are present and match

### Consensus Layer (CL) Mode
1. **Query Consensus Layer**: Fetches blobs directly from CL beacon node
2. **Fetch from Blob Service**: Retrieves blobs from the blob-service API for the same slot
3. **Calculate Hashes**: Computes versioned hashes from KZG commitments
4. **Compare**: Validates that blobs in service match those in CL

## Installation

```bash
cd validator
bun install
```

## Usage

```bash
bun validate <mode> <chain> [start_block] [blocks_to_validate]
```

### Arguments

- **mode**: `el` (execution layer) or `cl` (consensus layer) (default: `el`)
- **chain**: `eth`, `sepolia`, `holesky`, `gnosis`, `chiado`, `hoodi` (default: `eth`)
- **start_block**: Block number (EL mode) or slot number (CL mode) to start from, or negative offset from latest for EL mode (default: chain-specific)
- **blocks_to_validate**: Number of blocks/slots to validate (default: `100`)

### Examples

#### Execution Layer (EL) Mode

```bash
# Validate 100 blocks on Ethereum from default start (EL mode)
bun validate el eth

# Validate 50 blocks starting from block 24986100
bun validate el eth 24986100 50

# Validate last 100 blocks (negative offset from latest)
bun validate el eth -100

# Validate last 50 blocks
bun validate el eth -50 50

# Validate 200 blocks on Sepolia
bun validate el sepolia

# Validate 10 blocks on Gnosis starting from block 30000000
bun validate el gnosis 30000000 10
```

#### Consensus Layer (CL) Mode

```bash
# Validate 100 slots on Ethereum from default start (CL mode)
bun validate cl eth

# Validate 50 slots starting from slot 9000000
bun validate cl eth 9000000 50

# Validate 10 slots on Holesky starting from slot 1000000
bun validate cl holesky 1000000 10

# Validate 200 slots on Sepolia
bun validate cl sepolia 5000000 200
```

## Output

The validator provides detailed output with progress tracking:

### EL Mode Output

```
🚀 Blob Validator
   Mode:           EL
   Chain:          Ethereum Mainnet
   Execution RPC:  http://eth.rpcx.riv-prod1.pinax.io
   Blob Service:   https://eth.blobs.pinax.network
   Range:          24986100 - 24986109
   Count:          10

🔍 Validating blocks 24986100 to 24986109

[1/10] 📦 Block 24986100 (slot 14220497): 1 blob txs
   ✅ Tx 0xbb6a...: 5 blobs OK
   Total: 5 | Validated: 5 | Failed: 0

[2/10] 📦 Block 24986101 (slot 14220498): 1 blob txs
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

### CL Mode Output

```
🚀 Blob Validator
   Mode:           CL
   Chain:          Ethereum Mainnet
   Consensus RPC:  http://eth-arch909.riv.eosn.io:5052
   Blob Service:   https://eth.blobs.pinax.network
   Range:          9000000 - 9000009
   Count:          10

🔍 Validating blocks 9000000 to 9000009

[1/10] 📦 Slot 9000000: 6 blobs in CL
   ✅ All 6 blobs match
   Total: 6 | Validated: 6 | Failed: 0

[2/10] 📦 Slot 9000001: 4 blobs in CL
   ✅ All 4 blobs match
   Total: 4 | Validated: 4 | Failed: 0

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

      - name: Validate last 100 blocks (EL)
        run: |
          cd validator
          bun validate el eth -100

      - name: Validate last 100 slots (CL)
        run: |
          cd validator
          bun validate cl eth 9000000 100
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
- **goerli**: Goerli Testnet (deprecated)
- **hoodi**: Hoodi Testnet

All chains use Pinax RPC and blob service endpoints automatically.

**Note**: For CL mode, the consensus RPC endpoint is: `http://<chain>-arch909.riv.eosn.io:5052`

## License

Same as parent project
