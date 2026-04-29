import { ethers } from "ethers";
import { createHash } from "crypto";

/**
 * Validator that checks blobs in the service against execution layer or consensus layer
 */

type ValidationMode = "el" | "cl";

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
	mode: ValidationMode;
	executionRpcUrl?: string;
	consensusRpcUrl?: string;
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
	executionRpcUrl: string;
	consensusRpcUrl: string;
	blobServiceUrl: string;
}

const CHAIN_CONFIGS: Record<string, ChainConfig> = {
	eth: {
		chainId: 1,
		genesisTime: 1606824023,
		secondsPerSlot: 12,
		name: "Ethereum Mainnet",
		defaultStartBlock: 19426587,
		executionRpcUrl: "http://eth.rpcx.riv-prod1.pinax.io",
		consensusRpcUrl: "http://eth-arch909.riv.eosn.io:5052",
		blobServiceUrl: "https://eth.blobs.pinax.network",
	},
	sepolia: {
		chainId: 11155111,
		genesisTime: 1655733600,
		secondsPerSlot: 12,
		name: "Sepolia",
		defaultStartBlock: 5000000,
		executionRpcUrl: "http://sepolia.rpcx.riv-prod1.pinax.io",
		consensusRpcUrl: "http://sepolia-arch43.kan.eosn.io:5052",
		blobServiceUrl: "https://sepolia.blobs.pinax.network",
	},
	gnosis: {
		chainId: 100,
		genesisTime: 1638993340,
		secondsPerSlot: 5,
		name: "Gnosis",
		defaultStartBlock: 33000000, // After Dencun (March 11, 2024)
		executionRpcUrl: "http://gnosis.rpcx.riv-prod1.pinax.io",
		consensusRpcUrl: "http://gnosis-arch908.riv.eosn.io:5052",
		blobServiceUrl: "https://gnosis.blobs.pinax.network",
	},
	hoodi: {
		chainId: 17001,
		genesisTime: 1728891000,
		secondsPerSlot: 12,
		name: "Hoodi",
		defaultStartBlock: 1000000,
		executionRpcUrl: "http://hoodi-arch815.riv.eosn.io:8545",
		consensusRpcUrl: "http://hoodi-arch909.riv.eosn.io:5052",
		blobServiceUrl: "https://hoodi.blobs.pinax.network",
	},
};

class SmartBlobValidator {
	private provider?: ethers.JsonRpcProvider;
	private config: ValidationConfig;
	private chainConfig!: ChainConfig;

	constructor(config: ValidationConfig) {
		this.config = config;
		if (config.mode === "el" && config.executionRpcUrl) {
			this.provider = new ethers.JsonRpcProvider(config.executionRpcUrl);
		}
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
		console.log(`   Mode: ${this.config.mode.toUpperCase()}`);
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

		if (!response.ok) {
			console.warn(
				`⚠️  Blob service returned ${response.status} for slot ${slot}`,
			);
			if (response.status === 404) {
				return new Map(); // Slot not found, return empty
			}
			throw new Error(
				`Blob service error: ${response.status} ${response.statusText}`,
			);
		}

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
	 * Fetch blobs from consensus layer
	 */
	private async fetchConsensusBlobs(
		slot: number,
	): Promise<Map<number, string>> {
		const url = `${this.config.consensusRpcUrl}/eth/v1/beacon/blob_sidecars/${slot}`;
		const response = await fetch(url);

		if (!response.ok) {
			console.warn(
				`⚠️  Consensus layer returned ${response.status} for slot ${slot}`,
			);
			if (response.status === 404) {
				return new Map(); // Slot not found, return empty
			}
			throw new Error(
				`Consensus layer error: ${response.status} ${response.statusText}`,
			);
		}

		const data = await response.json();
		const consensusBlobs: BlobServiceBlob[] = data.data || [];

		const consensusBlobMap = new Map<number, string>();
		for (const blob of consensusBlobs) {
			const index = parseInt(blob.index, 10);
			const hash = this.calculateVersionedHash(blob.kzg_commitment);
			consensusBlobMap.set(index, hash);
		}

		return consensusBlobMap;
	}

	/**
	 * Get blob transactions from a block
	 */
	private async getBlobTransactions(
		blockNumber: number,
	): Promise<ethers.TransactionResponse[]> {
		if (!this.provider) return [];
		const block = await this.provider.getBlock(blockNumber, false);
		if (!block) return [];

		const txHashes = block.transactions as string[];

		// Fetch all transactions in parallel
		const provider = this.provider;
		const txPromises = txHashes.map((hash) =>
			provider.getTransaction(hash).catch(() => null),
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

			let slot: number;
			if (this.config.mode === "el") {
				// Get block and calculate slot
				if (!this.provider) {
					console.log("❌ Provider not initialized for EL mode");
					return { success: false, failedBlobs: 1 };
				}
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
				slot = this.slotFromTimestamp(Number(block.timestamp));
			} else {
				// For CL mode, we need to get the slot from the block number
				// We'll fetch the first slot to determine the mapping
				slot = blockNumber; // In CL mode, we treat block number as slot number
			}

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
		if (this.config.mode === "el") {
			return this.validateBlockEL(
				blockNumber,
				slot,
				currentBlock,
				totalBlockCount,
			);
		} else {
			return this.validateBlockCL(slot, currentBlock, totalBlockCount);
		}
	}

	private async validateBlockEL(
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

	private async validateBlockCL(
		slot: number,
		currentBlock: number,
		totalBlockCount: number,
	): Promise<{ totalBlobs: number; validatedBlobs: number }> {
		// Fetch blobs from service
		const serviceBlobMap = await this.fetchServiceBlobs(slot);

		// Fetch blobs from consensus layer
		const consensusBlobMap = await this.fetchConsensusBlobs(slot);

		// Show slot info
		console.log(
			`[${currentBlock}/${totalBlockCount}] 📦 Slot ${slot}: ${consensusBlobMap.size} blobs in CL`,
		);

		// Only process if there are blobs
		if (consensusBlobMap.size === 0) {
			return { totalBlobs: 0, validatedBlobs: 0 };
		}

		const totalBlobs = consensusBlobMap.size;
		let validatedBlobs = 0;
		let allValid = true;

		// Compare blobs
		for (const [index, consensusHash] of consensusBlobMap) {
			const serviceHash = serviceBlobMap.get(index);

			if (!serviceHash) {
				console.log(`   ❌ Blob ${index}: Missing in service`);
				allValid = false;
			} else if (serviceHash.toLowerCase() !== consensusHash.toLowerCase()) {
				console.log(`   ❌ Blob ${index}: Hash mismatch`);
				console.log(`      CL:      ${consensusHash}`);
				console.log(`      Service: ${serviceHash}`);
				allValid = false;
			} else {
				validatedBlobs++;
			}
		}

		// Check for extra blobs in service
		for (const [index, serviceHash] of serviceBlobMap) {
			if (!consensusBlobMap.has(index)) {
				console.log(`   ⚠️  Blob ${index}: Extra blob in service (not in CL)`);
				console.log(`      Service: ${serviceHash}`);
			}
		}

		if (allValid && validatedBlobs === totalBlobs) {
			console.log(`   ✅ All ${totalBlobs} blobs match`);
		}

		console.log(
			`   Total: ${totalBlobs} | Validated: ${validatedBlobs} | Failed: ${totalBlobs - validatedBlobs}`,
		);

		return { totalBlobs, validatedBlobs };
	}
}

async function main() {
	const args = process.argv.slice(2);

	// Parse arguments: mode chain [startBlock] [count]
	const mode = (args[0] || "el") as ValidationMode;
	const chainName = args[1] || "eth";
	const startBlock = args[2] ? parseInt(args[2]) : undefined;
	const blocksToValidate = args[3] ? parseInt(args[3]) : 100;

	// Validate mode
	if (mode !== "el" && mode !== "cl") {
		console.error(`❌ Invalid mode: ${mode}`);
		console.error(`   Supported modes: el, cl`);
		console.error(
			`   Usage: bun run validate <mode> <chain> [startBlock] [count]`,
		);
		process.exit(1);
	}

	// Get chain config
	const chainConfig = CHAIN_CONFIGS[chainName];
	if (!chainConfig) {
		console.error(`❌ Unknown chain: ${chainName}`);
		console.error(
			`   Supported chains: ${Object.keys(CHAIN_CONFIGS).join(", ")}`,
		);
		process.exit(1);
	}

	// Get URLs from chain config
	const { executionRpcUrl, consensusRpcUrl, blobServiceUrl } = chainConfig;

	// Resolve start block/slot (handle negative offsets from latest)
	let start: number;
	if (startBlock === undefined) {
		start = chainConfig.defaultStartBlock;
	} else if (startBlock < 0) {
		if (mode === "el") {
			const provider = new ethers.JsonRpcProvider(executionRpcUrl);
			const latestBlock = await provider.getBlockNumber();
			start = latestBlock + startBlock;
			console.log(
				`🔍 Latest block: ${latestBlock}, offset ${startBlock} → starting at ${start}`,
			);
		} else {
			// CL mode: fetch latest slot from consensus layer
			const response = await fetch(
				`${consensusRpcUrl}/eth/v1/beacon/headers/head`,
			);
			const data = await response.json();
			const latestSlot = parseInt(data.data.header.message.slot, 10);
			start = latestSlot + startBlock;
			console.log(
				`🔍 Latest slot: ${latestSlot}, offset ${startBlock} → starting at ${start}`,
			);
		}
	} else {
		start = startBlock;
	}

	const end = start + blocksToValidate - 1;

	console.log("🚀 Blob Validator");
	console.log(`   Mode:           ${mode.toUpperCase()}`);
	console.log(`   Chain:          ${chainConfig.name}`);
	if (mode === "el") {
		console.log(`   Execution RPC:  ${executionRpcUrl}`);
	} else {
		console.log(`   Consensus RPC:  ${consensusRpcUrl}`);
	}
	console.log(`   Blob Service:   ${blobServiceUrl}`);
	console.log(`   Range:          ${start} - ${end}`);
	console.log(`   Count:          ${blocksToValidate}`);

	const config: ValidationConfig = {
		mode,
		executionRpcUrl: mode === "el" ? executionRpcUrl : undefined,
		consensusRpcUrl: mode === "cl" ? consensusRpcUrl : undefined,
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
