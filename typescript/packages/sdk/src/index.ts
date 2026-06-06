export {
  TreasuryClient,
  TreasuryApiError,
  isTreasuryError,
  buildResourcePath,
  actionMethod,
  getResourceType,
  getResourceTypeBySingular,
  getResourceTypeByPlural,
  singularToPlural,
  pluralToSingular,
  parseResourceName,
  allResourceTypes,
} from "./client/index.js";
export type {
  TreasuryClientOptions,
  TreasuryError,
  ResourceTypeMeta,
} from "./client/index.js";

export { SigningKey, VerifyingKey } from "./crypto/keys.js";
export {
  parseAlgorithm,
  parseNormalAlgorithm,
  algorithmToNormal,
} from "./crypto/algorithms.js";
export type { Algorithm, NormalAlgorithm } from "./crypto/algorithms.js";
export { sha256, sha512, keccak256 } from "./crypto/digest.js";

export { signRequest, contentDigest, generateNonce } from "./signature/index.js";

export { Keyring } from "./keyring/index.js";
export type { KeyringEntry } from "./keyring/index.js";

export type * from "./types/resources.js";
