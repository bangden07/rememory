// tlock.ts — Time-lock encryption/decryption wrapper for drand's League of Entropy.
// Bundled as IIFE, exposes window.rememoryTlock.

import { timelockEncrypt, timelockDecrypt, Buffer } from 'tlock-js';
import { HttpCachingChain, HttpChainClient, roundTime } from 'drand-client';
import type { ChainClient, ChainOptions } from 'drand-client';

// Quicknet chain parameters
const QUICKNET_CHAIN_HASH = '52db9ba70e0cc0f6eaf7803dd07447a1f5477735fd3f661792ba94600c84e971';
const QUICKNET_GENESIS = 1692803367;  // Unix seconds: 2023-08-23T11:22:47Z
const QUICKNET_PERIOD = 3;           // seconds

const DRAND_ENDPOINTS = [
  'https://api.drand.sh',
  'https://api2.drand.sh',
  'https://api3.drand.sh',
  'https://drand.cloudflare.com',
];

// Manifest metadata envelope types (mirrors Go ManifestMeta)
interface TlockMeta {
  v: number;
  method: string;
  round: number;
  unlock: string;
  chain: string;
}

interface ManifestMeta {
  v: number;
  rememory: string;
  tlock?: TlockMeta;
}

interface ManifestMetaResult {
  meta: ManifestMeta;
  ciphertext: Uint8Array;
}

// Create a chain client, trying endpoints in order
async function createClient(): Promise<ChainClient> {
  const options: ChainOptions = {
    disableBeaconVerification: false,
    noCache: false,
    chainVerificationParams: {
      chainHash: QUICKNET_CHAIN_HASH,
      publicKey: '83cf0f2896adee7eb8b5f01fcad3912212c437e0073e911fb90022d3e760183c8c4b450b6a0a6c3ac6a5776a2d1064510d1fec758c921cc22b0e17e63aaf4bcb5ed66304de9cf809bd274ca73bab4af5a6e9c76a4bc09e76eae8991ef5ece45a',
    },
  };

  let lastError: Error | undefined;
  for (const endpoint of DRAND_ENDPOINTS) {
    try {
      const url = `${endpoint}/${QUICKNET_CHAIN_HASH}`;
      const chain = new HttpCachingChain(url, options);
      const client = new HttpChainClient(chain, options);
      // Verify connectivity by fetching chain info
      await chain.info();
      return client;
    } catch (e) {
      lastError = e instanceof Error ? e : new Error(String(e));
    }
  }
  throw new Error(`Could not connect to drand: ${lastError?.message ?? 'all endpoints failed'}`);
}

// Encrypt plaintext for a specific round number
async function encrypt(plaintext: Uint8Array, roundNumber: number): Promise<Uint8Array> {
  const client = await createClient();
  const payload = Buffer.from(plaintext);
  const armored = await timelockEncrypt(roundNumber, payload, client);
  return new TextEncoder().encode(armored);
}

// Decrypt tlock ciphertext by fetching the beacon
async function decrypt(ciphertext: Uint8Array): Promise<Uint8Array> {
  const client = await createClient();
  const armored = new TextDecoder().decode(ciphertext);
  const decrypted = await timelockDecrypt(armored, client);
  return new Uint8Array(decrypted);
}

// Check if a round's beacon is available (time has passed)
async function isRoundAvailable(roundNumber: number): Promise<boolean> {
  try {
    const client = await createClient();
    const info = await client.chain().info();
    const rt = roundTime(info, roundNumber);
    return rt <= Date.now();
  } catch {
    return false;
  }
}

// Compute the round number for a target date
function roundForTime(target: Date): number {
  const elapsed = (target.getTime() / 1000) - QUICKNET_GENESIS;
  if (elapsed <= 0) return 1;
  return Math.ceil(elapsed / QUICKNET_PERIOD) + 1;
}

// Compute the time at which a round will be emitted
function timeForRound(round: number): Date {
  if (round <= 1) return new Date(QUICKNET_GENESIS * 1000);
  const timestamp = QUICKNET_GENESIS + (round - 1) * QUICKNET_PERIOD;
  return new Date(timestamp * 1000);
}

// Parse a manifest metadata envelope from raw MANIFEST.age data
function parseManifestMeta(data: Uint8Array): ManifestMetaResult | null {
  if (data.length === 0 || data[0] !== 0x7B) return null;  // 0x7B = '{'

  // Find the newline separating the JSON header from the inner ciphertext
  const newlineIdx = data.indexOf(0x0A);  // 0x0A = '\n'
  if (newlineIdx === -1) return null;

  const headerLine = new TextDecoder().decode(data.slice(0, newlineIdx));
  try {
    const meta: ManifestMeta = JSON.parse(headerLine);
    if (!meta.v) return null;
    const ciphertext = data.slice(newlineIdx + 1);
    return { meta, ciphertext };
  } catch {
    return null;
  }
}

// Expose on window for IIFE bundle
const api = {
  encrypt,
  decrypt,
  isRoundAvailable,
  roundForTime,
  timeForRound,
  parseManifestMeta,
  QUICKNET_CHAIN_HASH,
  QUICKNET_GENESIS,
  QUICKNET_PERIOD,
};

(window as any).rememoryTlock = api;
