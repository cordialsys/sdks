/**
 * Keyring - manages signing keys stored as TOML files.
 *
 * Key files are stored in: ~/.local/share/treasury/<treasury-id>/keyring/<key-name>.toml
 * (matches the Rust keyring's data_dir() which uses dirs::data_dir() + "treasury")
 */
import * as fs from "node:fs";
import * as path from "node:path";
import * as os from "node:os";
import { parse as parseToml, stringify as stringifyToml } from "smol-toml";
import { SigningKey } from "../crypto/keys.js";
import type { NormalAlgorithm } from "../crypto/algorithms.js";

export interface KeyringEntry {
  name: string;
  algorithm: NormalAlgorithm;
  secret_key: string;
  credential?: string | null;
}

/**
 * Determine the data directory, matching the Rust `dirs::data_dir()` + "treasury".
 * On Linux: ~/.local/share/treasury
 * On macOS: ~/Library/Application Support/treasury
 */
function defaultDataDir(): string {
  const platform = os.platform();
  if (platform === "darwin") {
    return path.join(os.homedir(), "Library", "Application Support", "treasury");
  }
  // Linux and others: XDG_DATA_HOME or ~/.local/share
  const xdgData = process.env.XDG_DATA_HOME || path.join(os.homedir(), ".local", "share");
  return path.join(xdgData, "treasury");
}

export class Keyring {
  private readonly _dir: string;

  constructor(treasuryId: string) {
    if (!treasuryId) {
      throw new Error("Treasury ID must not be empty");
    }
    const dataDir = process.env.TREASURY_DATA_DIR || defaultDataDir();
    this._dir = path.join(dataDir, treasuryId, "keyring");
  }

  get dir(): string {
    return this._dir;
  }

  private filePath(name: string): string {
    return path.join(this._dir, `${name}.toml`);
  }

  /**
   * Get a signing key by name.
   */
  getSigningKey(name: string): SigningKey | null {
    const filePath = this.filePath(name);
    if (!fs.existsSync(filePath)) return null;

    try {
      const content = fs.readFileSync(filePath, "utf-8");
      const data = parseToml(content) as unknown as KeyringEntry;
      return SigningKey.fromSecretBytesHex(data.algorithm, data.secret_key);
    } catch {
      return null;
    }
  }

  /**
   * List all key names in the keyring.
   */
  listNames(): string[] {
    if (!fs.existsSync(this._dir)) return [];
    return fs
      .readdirSync(this._dir)
      .filter((f) => f.endsWith(".toml"))
      .map((f) => f.replace(".toml", ""));
  }

  /**
   * List all signing keys with their names.
   */
  listKeys(): Array<{ name: string; key: SigningKey }> {
    const names = this.listNames();
    const keys: Array<{ name: string; key: SigningKey }> = [];
    for (const name of names) {
      const key = this.getSigningKey(name);
      if (key) {
        keys.push({ name, key });
      }
    }
    return keys;
  }

  /**
   * Create a new random key and store it.
   */
  create(
    name: string,
    algorithm: NormalAlgorithm,
    overwrite = false,
  ): SigningKey {
    const key = SigningKey.generate(algorithm);
    this.store(name, key, overwrite);
    return key;
  }

  /**
   * Import a key from raw material.
   */
  import(
    name: string,
    algorithm: NormalAlgorithm,
    material: Uint8Array,
    overwrite = false,
  ): SigningKey {
    const key = SigningKey.fromSecretBytes(algorithm, material);
    if (!key) {
      throw new Error(`Invalid key material for algorithm ${algorithm}`);
    }
    this.store(name, key, overwrite);
    return key;
  }

  /**
   * Create a key from an invite code.
   */
  createInvite(name: string, code: string): SigningKey {
    const key = SigningKey.fromInvite(code);
    if (!key) {
      throw new Error(`Invalid invite code: ${code}`);
    }
    this.store(name, key, false);
    return key;
  }

  /**
   * Store a signing key.
   */
  store(name: string, key: SigningKey, overwrite = false): void {
    fs.mkdirSync(this._dir, { recursive: true });
    const filePath = this.filePath(name);

    if (!overwrite && fs.existsSync(filePath)) {
      throw new Error(`Key ${name} already exists. Use overwrite=true to replace.`);
    }

    const entry: KeyringEntry = {
      name,
      algorithm: key.algorithm,
      secret_key: key.toSecretBytesHex(),
    };

    const content = stringifyToml(entry as unknown as Record<string, unknown>);
    fs.writeFileSync(filePath, content, "utf-8");
  }

  /**
   * Delete a key from the keyring.
   */
  delete(name: string): boolean {
    const filePath = this.filePath(name);
    if (!fs.existsSync(filePath)) return false;
    fs.unlinkSync(filePath);
    return true;
  }
}
