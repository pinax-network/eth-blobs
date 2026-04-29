import { ethers } from "ethers";
import { createHash } from "crypto";

/**
 * Validator that checks blobs in the service against execution layer
 */

interface BlobServiceBlob {
	index: string;
	blob: string;
	kzg_commitment: string;
	signed_block_header: {
		message: {
			slot: string;
		};
	};
}

interface ValidationConfig {
	executionRpcUrl: string;
	blobServiceUrl: string;
	startBlock: number;
	endBlock?: number;
}

interface ChainConfig {
	genesisTime: number;
	secondsPerSlot: number;
	name: string;
	chainId: number;
	defaultStartBlock: number;
}

const CHAIN_CONFIGS: Record<string, ChainConfig> = {
	eth: {
		chainId: 1,
		genesisTime: 1606824023,
		secondsPerSlot: 12,
		name: "Ethereum Mainnet",
		defaultStartBlock: 19426587, // First block after Dencun
	},
	goerli: {
		chainId: 5,
		genesisTime: 1616508000,
		secondsPerSlot: 12,
		name: "Goerli",
		defaultStartBlock: 10000000,
	},
	sepolia: {
		chainId: 11155111,
		genesisTime: 1655733600,
		secondsPerSlot: 12,
		name: "Sepolia",
		defaultStartBlock: 5000000,
	},
	holesky: {
		chainId: 17000,
		genesisTime: 1695902400,
		secondsPerSlot: 12,
		name: "Holesky",
		defaultStartBlock: 1000000,
	},
	gnosis: {
		chainId: 100,
		genesisTime: 1638993340,
		secondsPerSlot: 5,
		name: "Gnosis",
		defaultStartBlock: 30000000,
	},
	chiado: {
		chainId: 10200,
		genesisTime: 1665396300,
		secondsPerSlot: 5,
		name: "Chiado",
		defaultStartBlock: 5000000,
	},
	hoodi: {
		chainId: 17001,
		genesisTime: 1728891000,
		secondsPerSlot: 12,
		name: "Hoodi",
		defaultStartBlock: 1000000,
	},
};

class SmartBlobValidator {
	private provider: ethers.JsonRpcProvider;
	private config: ValidationConfig;
	private chainConfig!: ChainConfig;

	constructor(config: ValidationConfig) {
		this.config = config;
		this.provider = new ethers.JsonRpcProvider(config.executionRpcUrl);
	}

	/**
	 * Initialize validator
	 */
	async initialize(chainConfig: ChainConfig): Promise<void> {
		this.chainConfig = chainConfig;

		console.log(
			`🔗 Chain: ${this.chainConfig.name} (ID: ${this.chainConfig.chainId})`,
		);
		console.log(`   Genesis time: ${this.chainConfig.genesisTime}`);
		console.log(`   Seconds per slot: ${this.chainConfig.secondsPerSlot}`);
	}

	/**
	 * Calculate slot from block timestamp
	 */
	private slotFromTimestamp(timestamp: number): number {
		return Math.floor(
			(timestamp - this.chainConfig.genesisTime) /
				this.chainConfig.secondsPerSlot,
		);
	}

	private calculateVersionedHash(commitment: string): string {
		const commitmentBytes = commitment.startsWith("0x")
			? commitment.slice(2)
			: commitment;
		const hash = createHash("sha256")
			.update(Buffer.from(commitmentBytes, "hex"))
			.digest();
		hash[0] = 0x01;
		return "0x" + hash.toString("hex");
	}

	/**
	 * Validate blob data integrity
	 * Verifies blob size and format
	 */
	private validateBlobData(blobHex: string, commitmentHex: string): boolean {
		try {
			// Remove 0x prefix if present
			const blob = blobHex.startsWith("0x") ? blobHex.slice(2) : blobHex;
			const commitment = commitmentHex.startsWith("0x")
				? commitmentHex.slice(2)
				: commitmentHex;

			// Check blob is exactly 131072 bytes (4096 field elements * 32 bytes)
			if (blob.length !== 131072 * 2) {
				// *2 because hex
				console.warn(
					`⚠️  Invalid blob size: expected ${131072 * 2} hex chars, got ${blob.length}`,
				);
				return false;
			}

			// Check commitment is 48 bytes (BLS12-381 G1 point)
			if (commitment.length !== 48 * 2) {
				console.warn(
					`⚠️  Invalid commitment size: expected ${48 * 2} hex chars, got ${commitment.length}`,
				);
				return false;
			}

			// Verify blob contains valid hex
			if (!/^[0-9a-fA-F]+$/.test(blob) || !/^[0-9a-fA-F]+$/.test(commitment)) {
				console.warn("⚠️  Invalid hex data in blob or commitment");
				return false;
			}

			return true;
		} catch (error) {
			console.error("Blob validation error:", error);
			return false;
		}
	}

	/**
	 * Fetch blobs from service and build index map
	 */
	private async fetchServiceBlobs(slot: number): Promise<Map<number, string>> {
		const url = `${this.config.blobServiceUrl}/eth/v1/beacon/blob_sidecars/${slot}`;
		const response = await fetch(url);
		const data = await response.json();
		const serviceBlobs: BlobServiceBlob[] = data.data || [];

		const serviceBlobMap = new Map<number, string>();
		for (const blob of serviceBlobs) {
			const index = parseInt(blob.index, 10);

			// Validate blob data integrity
			const isValid = this.validateBlobData(blob.blob, blob.kzg_commitment);
			if (!isValid) {
				console.warn(
					`⚠️  Blob ${index} in slot ${slot}: Data validation failed`,
				);
			}

			const hash = this.calculateVersionedHash(blob.kzg_commitment);
			serviceBlobMap.set(index, hash);
		}

		return serviceBlobMap;
	}

	/**
	 * Get blob transactions from a block
	 */
	private async getBlobTransactions(
		blockNumber: number,
	): Promise<ethers.TransactionResponse[]> {
		const block = await this.provider.getBlock(blockNumber, false);
		if (!block) return [];

		const txHashes = block.transactions as string[];

		// Fetch all transactions in parallel
		const txPromises = txHashes.map((hash) =>
			this.provider.getTransaction(hash).catch(() => null),
		);
		const transactions = await Promise.all(txPromises);

		// Filter for blob transactions (type 3)
		return transactions.filter(
			(tx) => tx && tx.type === 3,
		) as ethers.TransactionResponse[];
	}

	/**
	 * Validate blocks one by one
	 */
	async validateRange(
		startBlock: number,
		endBlock?: number,
	): Promise<{ success: boolean; failedBlobs: number }> {
		const end = endBlock || startBlock;

		console.log(`\n🔍 Validating blocks ${startBlock} to ${end}\n`);

		let totalBlobs = 0;
		let validatedBlobs = 0;
		let blocksWithBlobs = 0;
		const totalBlockCount = end - startBlock + 1;
		let currentBlock = 0;

		for (let blockNumber = startBlock; blockNumber <= end; blockNumber++) {
			currentBlock++;

			// Get block and calculate slot
			const block = await this.provider.getBlock(blockNumber);
			if (!block) {
				console.log(
					`[${currentBlock}/${totalBlockCount}] ❌ Block ${blockNumber}: not found`,
				);
				console.log("\n" + "=".repeat(60));
				console.log("❌ VALIDATION FAILED");
				console.log("=".repeat(60));
				console.log(
					`Block ${blockNumber} not found - chain may not be synced or block range is invalid`,
				);
				console.log("=".repeat(60) + "\n");
				return { success: false, failedBlobs: 1 };
			}

			const slot = this.slotFromTimestamp(Number(block.timestamp));

			// Validate this block
			const result = await this.validateBlock(
				blockNumber,
				slot,
				currentBlock,
				totalBlockCount,
			);

			if (result.totalBlobs > 0) {
				blocksWithBlobs++;
			}

			totalBlobs += result.totalBlobs;
			validatedBlobs += result.validatedBlobs;

			// Exit immediately if any blobs failed
			const failedBlobs = totalBlobs - validatedBlobs;
			if (failedBlobs > 0) {
				console.log("\n" + "=".repeat(60));
				console.log("❌ VALIDATION FAILED");
				console.log("=".repeat(60));
				console.log(`Blocks checked:        ${currentBlock}`);
				console.log(`Blocks with blobs:     ${blocksWithBlobs}`);
				console.log(`Total blobs:           ${totalBlobs}`);
				console.log(`✅ Validated blobs:    ${validatedBlobs}`);
				console.log(`❌ Failed blobs:       ${failedBlobs}`);
				console.log("=".repeat(60) + "\n");
				return { success: false, failedBlobs };
			}
		}

		console.log("\n" + "=".repeat(60));
		console.log("📊 VALIDATION SUMMARY");
		console.log("=".repeat(60));
		console.log(`Blocks checked:        ${end - startBlock + 1}`);
		console.log(`Blocks with blobs:     ${blocksWithBlobs}`);
		console.log(`Total blobs:           ${totalBlobs}`);
		console.log(`✅ Validated blobs:    ${validatedBlobs}`);
		console.log(`❌ Failed blobs:       0`);
		console.log("=".repeat(60) + "\n");

		return { success: true, failedBlobs: 0 };
	}

	private async validateBlock(
		blockNumber: number,
		slot: number,
		currentBlock: number,
		totalBlockCount: number,
	): Promise<{ totalBlobs: number; validatedBlobs: number }> {
		// Fetch blobs from service
		const serviceBlobMap = await this.fetchServiceBlobs(slot);

		// Get blob transactions from block
		const blobTxs = await this.getBlobTransactions(blockNumber);

		// Show block info
		console.log(
			`[${currentBlock}/${totalBlockCount}] 📦 Block ${blockNumber} (slot ${slot}): ${blobTxs.length} blob txs`,
		);

		// Only process if there are blob transactions
		if (blobTxs.length === 0) {
			return { totalBlobs: 0, validatedBlobs: 0 };
		}

		let totalBlobs = 0;
		let validatedBlobs = 0;
		let globalBlobIndex = 0;

		// Validate each blob transaction
		for (const tx of blobTxs) {
			if (!tx) continue;

			const expectedHashes = tx.blobVersionedHashes || [];
			totalBlobs += expectedHashes.length;

			let txValid = true;
			for (let i = 0; i < expectedHashes.length; i++) {
				const expectedHash = expectedHashes[i];
				const actualHash = serviceBlobMap.get(globalBlobIndex);

				if (!actualHash) {
					console.log(
						`   ❌ Tx ${tx.hash}: Missing blob ${i} (index ${globalBlobIndex})`,
					);
					txValid = false;
				} else if (actualHash.toLowerCase() !== expectedHash.toLowerCase()) {
					console.log(
						`   ❌ Tx ${tx.hash}: Mismatch at blob ${i} (index ${globalBlobIndex})`,
					);
					console.log(`      Expected: ${expectedHash}`);
					console.log(`      Got:      ${actualHash}`);
					txValid = false;
				} else {
					validatedBlobs++;
				}

				globalBlobIndex++;
			}

			if (txValid) {
				console.log(`   ✅ Tx ${tx.hash}: ${expectedHashes.length} blobs OK`);
			}
		}

		console.log(
			`   Total: ${totalBlobs} | Validated: ${validatedBlobs} | Failed: ${totalBlobs - validatedBlobs}`,
		);

		return { totalBlobs, validatedBlobs };
	}
}

async function main() {
	const args = process.argv.slice(2);

	// Parse arguments
	const chainName = args[0] || "eth";
	const startBlock = args[1] ? parseInt(args[1]) : undefined;
	const blocksToValidate = args[2] ? parseInt(args[2]) : 100;

	// Get chain config
	const chainConfig = CHAIN_CONFIGS[chainName];
	if (!chainConfig) {
		console.error(`❌ Unknown chain: ${chainName}`);
		console.error(
			`   Supported chains: ${Object.keys(CHAIN_CONFIGS).join(", ")}`,
		);
		process.exit(1);
	}

	// Generate URLs
	const executionRpcUrl = `http://${chainName}.rpcx.riv-prod1.pinax.io`;
	const blobServiceUrl = `https://${chainName}.blobs.pinax.network`;

	// Resolve start block (handle negative offsets from latest)
	let start: number;
	if (startBlock === undefined) {
		start = chainConfig.defaultStartBlock;
	} else if (startBlock < 0) {
		const provider = new ethers.JsonRpcProvider(executionRpcUrl);
		const latestBlock = await provider.getBlockNumber();
		start = latestBlock + startBlock;
		console.log(
			`🔍 Latest block: ${latestBlock}, offset ${startBlock} → starting at ${start}`,
		);
	} else {
		start = startBlock;
	}

	const end = start + blocksToValidate - 1;

	console.log("🚀 Blob Validator");
	console.log(`   Chain:          ${chainConfig.name}`);
	console.log(`   Execution RPC:  ${executionRpcUrl}`);
	console.log(`   Blob Service:   ${blobServiceUrl}`);
	console.log(`   Block Range:    ${start} - ${end}`);
	console.log(`   Blocks:         ${blocksToValidate}`);

	const config: ValidationConfig = {
		executionRpcUrl,
		blobServiceUrl,
		startBlock: start,
		endBlock: end,
	};

	const validator = new SmartBlobValidator(config);
	await validator.initialize(chainConfig);

	const result = await validator.validateRange(start, end);

	if (!result.success) {
		console.error(
			`\n❌ Validation failed: ${result.failedBlobs} blob(s) had errors\n`,
		);
		process.exit(1);
	}

	console.log("✅ All blobs validated successfully!\n");
}

main().catch((error) => {
	console.error("\n❌ Fatal error:", error);
	process.exit(1);
});
