/**
 * Resource type metadata for Treasury API.
 */

export interface ResourceTypeMeta {
  singular: string;
  plural: string;
  nested?: string; // parent resource type plural
}

const RESOURCE_TYPES: Record<string, ResourceTypeMeta> = {
  "access-rule": { singular: "access-rule", plural: "access-rules" },
  account: { singular: "account", plural: "accounts" },
  address: { singular: "address", plural: "addresses", nested: "chains" },
  airdrop: { singular: "airdrop", plural: "airdrops" },
  asset: { singular: "asset", plural: "assets", nested: "chains" },
  backup: { singular: "backup", plural: "backups", nested: "treasuries" },
  call: { singular: "call", plural: "calls" },
  "call-rule": { singular: "call-rule", plural: "call-rules" },
  chain: { singular: "chain", plural: "chains" },
  credential: { singular: "credential", plural: "credentials", nested: "users" },
  feature: { singular: "feature", plural: "features" },
  host: { singular: "host", plural: "hosts" },
  key: { singular: "key", plural: "keys" },
  operation: { singular: "operation", plural: "operations" },
  role: { singular: "role", plural: "roles" },
  signatory: { singular: "signatory", plural: "signatories" },
  signature: { singular: "signature", plural: "signatures" },
  signer: { singular: "signer", plural: "signers" },
  "software-update": { singular: "software-update", plural: "software-updates" },
  staking: { singular: "staking", plural: "stakings" },
  "staking-rule": { singular: "staking-rule", plural: "staking-rules" },
  symbol: { singular: "symbol", plural: "symbols", nested: "chains" },
  tag: { singular: "tag", plural: "tags" },
  transaction: { singular: "transaction", plural: "transactions" },
  transfer: { singular: "transfer", plural: "transfers" },
  "transfer-rule": { singular: "transfer-rule", plural: "transfer-rules" },
  treasury: { singular: "treasury", plural: "treasuries" },
  type: { singular: "type", plural: "types" },
  user: { singular: "user", plural: "users" },
};

export function getResourceType(name: string): ResourceTypeMeta {
  // Try singular first
  const direct = RESOURCE_TYPES[name];
  if (direct) return direct;

  // Try looking up by plural
  for (const meta of Object.values(RESOURCE_TYPES)) {
    if (meta.plural === name) return meta;
  }

  throw new Error(`Unknown resource type: ${name}`);
}

export function getResourceTypeBySingular(singular: string): ResourceTypeMeta {
  const meta = RESOURCE_TYPES[singular];
  if (!meta) throw new Error(`Unknown resource type: ${singular}`);
  return meta;
}

export function getResourceTypeByPlural(plural: string): ResourceTypeMeta {
  for (const meta of Object.values(RESOURCE_TYPES)) {
    if (meta.plural === plural) return meta;
  }
  throw new Error(`Unknown resource type plural: ${plural}`);
}

export function allResourceTypes(): ResourceTypeMeta[] {
  return Object.values(RESOURCE_TYPES);
}

export function singularToPlural(singular: string): string {
  return getResourceTypeBySingular(singular).plural;
}

export function pluralToSingular(plural: string): string {
  return getResourceTypeByPlural(plural).singular;
}

/**
 * Parse a resource name like "accounts/my-account" into its parts.
 */
export function parseResourceName(name: string): {
  resourceType: string;
  ids: string[];
} {
  const parts = name.split("/");
  if (parts.length < 1) {
    throw new Error(`Invalid resource name: ${name}`);
  }
  const resourcePlural = parts[parts.length - 2] || parts[0];
  const ids = parts.filter((_, i) => i % 2 === 1);
  return { resourceType: pluralToSingular(resourcePlural), ids };
}

/**
 * Build a resource path for an API request.
 */
export function buildResourcePath(opts: {
  action: "create" | "get" | "list" | "update" | "delete" | string;
  resourceType: string;
  id?: string;
  parentId?: string;
}): string {
  const meta = getResourceTypeBySingular(opts.resourceType);
  const isNested = !!meta.nested;
  const action = opts.action;

  if (isNested) {
    const parentPlural = meta.nested!;
    switch (action) {
      case "get":
      case "update":
      case "delete":
        if (!opts.parentId || !opts.id) {
          throw new Error(`${action} on nested resource ${opts.resourceType} requires parentId and id`);
        }
        return `${parentPlural}/${opts.parentId}/${meta.plural}/${opts.id}`;
      case "create":
        if (opts.id && opts.parentId) {
          return `${parentPlural}/${opts.parentId}/${meta.plural}/${opts.id}`;
        }
        if (opts.parentId) {
          return `${parentPlural}/${opts.parentId}/${meta.plural}`;
        }
        throw new Error(`create on nested resource ${opts.resourceType} requires parentId`);
      case "list":
        if (opts.parentId) {
          return `${parentPlural}/${opts.parentId}/${meta.plural}`;
        }
        return meta.plural;
      default:
        // Custom action
        if (opts.id && opts.parentId) {
          return `${parentPlural}/${opts.parentId}/${meta.plural}/${opts.id}/${action}`;
        }
        if (opts.id) {
          return `${meta.plural}/${opts.id}/${action}`;
        }
        return `${meta.plural}/${action}`;
    }
  } else {
    switch (action) {
      case "get":
      case "update":
      case "delete":
        if (!opts.id) {
          throw new Error(`${action} on ${opts.resourceType} requires id`);
        }
        return `${meta.plural}/${opts.id}`;
      case "create":
        if (opts.id) {
          return `${meta.plural}/${opts.id}`;
        }
        return meta.plural;
      case "list":
        return meta.plural;
      default:
        // Custom action
        if (opts.id) {
          return `${meta.plural}/${opts.id}/${action}`;
        }
        return `${meta.plural}/${action}`;
    }
  }
}

/**
 * Get the HTTP method for an action.
 */
export function actionMethod(action: string): string {
  switch (action) {
    case "create":
      return "POST";
    case "get":
    case "list":
      return "GET";
    case "update":
      return "PUT";
    case "delete":
      return "DELETE";
    default:
      // Custom actions use POST
      return "POST";
  }
}
