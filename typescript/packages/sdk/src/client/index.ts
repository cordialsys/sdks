/**
 * Treasury API Client.
 */
import { SigningKey } from "../crypto/keys.js";
import { signRequest } from "../signature/index.js";
import { buildResourcePath, actionMethod, getResourceType, getResourceTypeBySingular } from "./resource-types.js";
import type {
  TreasuryResource,
  TreasuryResourceName,
  TreasuryResourcePage,
  TreasuryResourceType,
} from "./resources.js";
import type {
  Account,
  AccountName,
  AccountPage,
  ExplicitFeePayerPolicy,
  Operation,
  OperationName,
} from "../types/resources.js";

export { buildResourcePath, actionMethod, getResourceType, getResourceTypeBySingular, getResourceTypeByPlural, singularToPlural, pluralToSingular, parseResourceName, allResourceTypes } from "./resource-types.js";
export type { ResourceTypeMeta } from "./resource-types.js";
export type {
  TreasuryResource,
  TreasuryResourceMap,
  TreasuryResourceName,
  TreasuryResourcePage,
  TreasuryResourceType,
} from "./resources.js";

export interface TreasuryClientOptions {
  baseUrl?: string;
  treasuryId: string;
  hostId?: string;
  apiKey?: string;
  signingKey?: SigningKey;
  timeout?: number;
}

export interface TreasuryError {
  code: number | string;
  status: string;
  message: string;
  details?: string[];
}

export function isTreasuryError(value: unknown): value is TreasuryError {
  return (
    typeof value === "object" &&
    value !== null &&
    "status" in value &&
    "message" in value
  );
}

export class TreasuryApiError extends Error {
  code: number | string;
  status: string;
  details?: string[];

  constructor(error: TreasuryError) {
    super(error.message);
    this.name = "TreasuryApiError";
    this.code = error.code;
    this.status = error.status;
    this.details = error.details;
  }
}

export class TreasuryClient {
  static readonly defaultBaseUrl = "https://treasury.cordialapis.com/";

  readonly baseUrl: string;
  readonly treasuryId: string;
  readonly hostId?: string;
  readonly apiKey?: string;
  private _signingKey?: SigningKey;
  private _timeout: number;

  constructor(opts: TreasuryClientOptions) {
    this.baseUrl = (opts.baseUrl ?? TreasuryClient.defaultBaseUrl).replace(/\/$/, "");
    this.treasuryId = opts.treasuryId;
    this.hostId = opts.hostId;
    this.apiKey = opts.apiKey ? normalizeApiKey(opts.apiKey) : undefined;
    this._signingKey = opts.signingKey;
    this._timeout = opts.timeout ?? 30000;

    if (!this.treasuryId) {
      throw new Error("treasuryId is required");
    }
    if (isDefaultBaseUrl(this.baseUrl) && !this.apiKey) {
      throw new Error("apiKey is required when using the default Treasury API base URL");
    }
  }

  get signingKey(): SigningKey | undefined {
    return this._signingKey;
  }

  setSigningKey(key: SigningKey): void {
    this._signingKey = key;
  }

  /**
   * Execute a raw HTTP request to the Treasury API.
   */
  async request(opts: {
    method: string;
    path: string;
    body?: string;
    query?: Record<string, string>;
    tag?: string;
  }): Promise<unknown> {
    const url = new URL(`/v1${opts.path}`, this.baseUrl);
    if (opts.query) {
      for (const [k, v] of Object.entries(opts.query)) {
        if (v !== undefined && v !== "") {
          url.searchParams.set(k, v);
        }
      }
    }

    const body = opts.body ?? "{}";
    const headers: Record<string, string> = {
      "content-type": "application/json",
      Treasury: this.treasuryId,
    };

    if (this.hostId) {
      headers["Treasury-Host"] = this.hostId;
    }

    if (this.apiKey) {
      headers["Authorization"] = `Bearer ${this.apiKey}`;
    }

    if (this._signingKey) {
      const signed = signRequest({
        signingKey: this._signingKey,
        method: opts.method,
        path: url.pathname,
        query: url.searchParams.toString(),
        body,
        treasuryId: this.treasuryId,
        hostId: this.hostId,
        tag: opts.tag,
      });

      headers["content-digest"] = signed["content-digest"];
      headers["signature-input"] = signed["signature-input"];
      headers["signature"] = signed["signature"];
    }

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this._timeout);

    try {
      const response = await fetch(url.toString(), {
        method: opts.method,
        headers,
        body: opts.method !== "GET" ? body : undefined,
        signal: controller.signal,
      });

      const responseBody = await response.text();

      if (!response.ok) {
        let error: TreasuryError;
        try {
          error = JSON.parse(responseBody);
        } catch {
          throw new TreasuryApiError({
            code: "UNKNOWN",
            status: "Unknown",
            message: `HTTP ${response.status}: ${responseBody}`,
          });
        }
        throw new TreasuryApiError(error);
      }

      if (!responseBody) return {};
      return JSON.parse(responseBody);
    } finally {
      clearTimeout(timeoutId);
    }
  }

  /**
   * Get a single resource by name.
   * @param resourceName Full resource name like "accounts/my-account"
   */
  async get(resourceName: string): Promise<Record<string, unknown>> {
    const path = `/${resourceName}`;
    return (await this.request({ method: "GET", path })) as Record<string, unknown>;
  }

  /**
   * List resources.
   */
  async list(
    resourceType: string,
    opts?: {
      parentId?: string;
      filter?: string;
      orderBy?: string;
      pageSize?: number;
      pageToken?: string;
      deleted?: boolean;
    },
  ): Promise<Record<string, unknown>> {
    const path = `/${buildResourcePath({
      action: "list",
      resourceType,
      parentId: opts?.parentId,
    })}`;

    const query: Record<string, string> = {};
    if (opts?.filter) query["filter"] = opts.filter;
    if (opts?.orderBy) query["order_by"] = opts.orderBy;
    if (opts?.pageSize) query["page_size"] = opts.pageSize.toString();
    if (opts?.pageToken) query["page_token"] = opts.pageToken;
    if (opts?.deleted) query["show_deleted"] = "true";

    return (await this.request({ method: "GET", path, query })) as Record<string, unknown>;
  }

  /**
   * Create a resource. Returns the operation name.
   */
  async create(
    resourceType: string,
    data: Record<string, unknown>,
    opts?: { id?: string; parentId?: string; variant?: string },
  ): Promise<string> {
    const path = `/${buildResourcePath({
      action: "create",
      resourceType,
      id: opts?.id,
      parentId: opts?.parentId,
    })}`;

    const body = JSON.stringify(data);
    return this.extractOpName(await this.request({ method: "POST", path, body }));
  }

  /**
   * Update a resource. Returns the operation name.
   */
  async update(
    resourceName: string,
    data: Record<string, unknown>,
  ): Promise<string> {
    const path = `/${resourceName}`;
    const existing = await this.get(resourceName);
    const body = JSON.stringify({ ...existing, ...data });
    return this.extractOpName(await this.request({ method: "PUT", path, body }));
  }

  /**
   * Delete a resource. Returns the operation name.
   */
  async delete(resourceName: string): Promise<string> {
    const path = `/${resourceName}`;
    return this.extractOpName(await this.request({ method: "DELETE", path }));
  }

  async listResources<T extends TreasuryResourceType>(
    resourceType: T,
    opts?: ListOptions,
  ): Promise<TreasuryResourcePage<T>> {
    return (await this.list(resourceType, opts)) as TreasuryResourcePage<T>;
  }

  async getResource<T extends TreasuryResourceType>(
    resourceType: T,
    idOrName: TreasuryResourceName<T> | string,
  ): Promise<TreasuryResource<T>> {
    const plural = getResourceTypeBySingular(resourceType).plural;
    return (await this.get(toResourceName(plural, idOrName))) as TreasuryResource<T>;
  }

  async createResource<T extends TreasuryResourceType>(
    resourceType: T,
    data: TreasuryResource<T>,
    opts?: { id?: string; parentId?: string },
  ): Promise<OperationName> {
    return await this.create(resourceType, data as unknown as JsonObject, opts);
  }

  async updateResource<T extends TreasuryResourceType>(
    resourceType: T,
    idOrName: TreasuryResourceName<T> | string,
    changes: Partial<TreasuryResource<T>>,
  ): Promise<OperationName> {
    const plural = getResourceTypeBySingular(resourceType).plural;
    const resourceName = toResourceName(plural, idOrName);
    const existing = await this.get(resourceName) as TreasuryResource<T>;
    return this.extractOpName(await this.request({
      method: "PUT",
      path: `/${resourceName}`,
      body: JSON.stringify({ ...existing, ...changes }),
    }));
  }

  async deleteResource<T extends TreasuryResourceType>(
    resourceType: T,
    idOrName: TreasuryResourceName<T> | string,
  ): Promise<OperationName> {
    const plural = getResourceTypeBySingular(resourceType).plural;
    return await this.delete(toResourceName(plural, idOrName));
  }

  /**
   * Approve an operation by replaying the original request with an approval tag.
   */
  async approve(operationName: string): Promise<string> {
    const operationId = operationName.replace("operations/", "");
    const op = await this.get(operationName);
    const req = op.request as Record<string, unknown>;
    const { method, path, body } = this.reconstructRequest(req);
    return this.extractOpName(await this.request({
      method,
      path,
      body,
      tag: `approve:${operationId}`,
    }));
  }

  /**
   * Cancel an operation by replaying the original request with a cancel tag.
   */
  async cancel(operationName: string): Promise<string> {
    const operationId = operationName.replace("operations/", "");
    const op = await this.get(operationName);
    const req = op.request as Record<string, unknown>;
    const { method, path, body } = this.reconstructRequest(req);
    return this.extractOpName(await this.request({
      method,
      path,
      body,
      tag: `cancel:${operationId}`,
    }));
  }

  /**
   * Reconstruct the original HTTP request from an operation's request data.
   */
  private reconstructRequest(req: Record<string, unknown>): {
    method: string;
    path: string;
    body: string;
  } {
    const action = req.action as string;
    const resource = req.resource as string;
    const id = req.id as string | undefined;
    const parent = req.parent as string | undefined;
    const bodyStr = (req.body as string) || "{}";

    const method = actionMethod(action);

    // Build the resource path from the operation's request data
    const resourceType = resource.toLowerCase().replace(/([a-z])([A-Z])/g, "$1-$2").toLowerCase();
    const meta = getResourceType(resourceType);

    let path: string;
    if (meta.nested && parent) {
      if (id) {
        path = `/${meta.nested}/${parent}/${meta.plural}/${id}`;
      } else {
        path = `/${meta.nested}/${parent}/${meta.plural}`;
      }
    } else if (id) {
      path = `/${meta.plural}/${id}`;
    } else {
      path = `/${meta.plural}`;
    }

    return { method, path, body: bodyStr };
  }

  /**
   * Execute a custom action on a resource.
   */
  async custom(
    resourceName: string,
    action: string,
    data?: Record<string, unknown> | string,
  ): Promise<string> {
    const path = `/${resourceName}/${action}`;
    const body = typeof data === "string" ? data : data ? JSON.stringify(data) : "{}";
    return this.extractOpName(await this.request({ method: "POST", path, body }));
  }

  private extractOpName(result: unknown): string {
    if (typeof result === "string") return result;
    return (result as Record<string, unknown>).name as string;
  }

  /**
   * Get an operation and return its current state.
   */
  async getOperation(operationName: string): Promise<Record<string, unknown>> {
    return this.get(operationName);
  }

  async getOperationTyped(operationName: OperationName): Promise<Operation> {
    return (await this.get(operationName)) as Operation;
  }

  async listAccounts(opts?: ListOptions): Promise<AccountPage> {
    return (await this.list("account", opts)) as AccountPage;
  }

  async getAccount(account: AccountName | string): Promise<Account> {
    return (await this.get(toResourceName("accounts", account))) as Account;
  }

  async createAccount(
    data: Account,
    opts?: { id?: string },
  ): Promise<OperationName> {
    return await this.create("account", data as JsonObject, { id: opts?.id });
  }

  async importAccount(account: string, data: Account): Promise<OperationName> {
    return this.extractOpName(await this.request({
      method: "POST",
      path: `/accounts/${account}`,
      body: JSON.stringify(data),
    }));
  }

  async updateAccount(
    account: AccountName | string,
    changes: Partial<Account>,
  ): Promise<OperationName> {
    const resourceName = toResourceName("accounts", account);
    const existing = await this.getAccount(resourceName);
    return this.extractOpName(await this.request({
      method: "PUT",
      path: `/${resourceName}`,
      body: JSON.stringify({ ...existing, ...changes }),
    }));
  }

  async deleteAccount(account: AccountName | string): Promise<OperationName> {
    return this.delete(toResourceName("accounts", account));
  }

  async setAccountFeePayer(
    account: AccountName | string,
    policy: ExplicitFeePayerPolicy,
  ): Promise<OperationName> {
    const resourceName = toResourceName("accounts", account);
    return this.extractOpName(await this.request({
      method: "POST",
      path: `/${resourceName}/fee-payer`,
      body: JSON.stringify(policy),
    }));
  }

  /**
   * Poll an operation until it reaches a terminal state.
   */
  async waitForOperation(
    operationName: string,
    opts?: { timeout?: number; interval?: number },
  ): Promise<Record<string, unknown>> {
    const timeout = opts?.timeout ?? 120000;
    const interval = opts?.interval ?? 500;
    const start = Date.now();

    // First, wait for the operation to appear (may be 404 briefly)
    let op: Record<string, unknown> | undefined;
    for (let i = 0; i < 20; i++) {
      try {
        op = await this.getOperation(operationName);
        break;
      } catch (e) {
        if (e instanceof TreasuryApiError && e.code === "NOT_FOUND") {
          await sleep(interval);
          continue;
        }
        throw e;
      }
    }

    if (!op) {
      throw new TreasuryApiError({
        code: 5,
        status: "Not Found",
        message: `Operation ${operationName} not found after retries`,
      });
    }

    // Now poll until terminal state
    while (Date.now() - start < timeout) {
      const state = (op.state as string) || "";
      if (state === "succeeded" || state === "failed") {
        return op;
      }
      await sleep(interval);
      op = await this.getOperation(operationName);
    }

    throw new TreasuryApiError({
      code: 4,
      status: "Deadline Exceeded",
      message: `Operation ${operationName} did not complete within ${timeout}ms`,
    });
  }

  /**
   * Create a resource and wait for the operation to complete.
   * Returns the created resource.
   */
  async createAndWait(
    resourceType: string,
    data: Record<string, unknown>,
    opts?: { id?: string; parentId?: string; variant?: string },
  ): Promise<Record<string, unknown>> {
    const opName = await this.create(resourceType, data, opts);
    const op = await this.waitForOperation(opName);
    if ((op.state as string) === "failed") {
      const error = (op as Record<string, unknown>)?.error as Record<string, unknown>;
      throw new TreasuryApiError({
        code: (error?.code as number | string) || "UNKNOWN",
        status: (error?.status as string) || "Unknown",
        message: (error?.message as string) || "Operation failed",
      });
    }
    const response = (op as Record<string, unknown>)?.response as Record<string, unknown>;
    const resourceName = response?.name as string;
    if (resourceName) {
      return this.get(resourceName);
    }
    return op;
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

type JsonObject = Record<string, unknown>;

export interface ListOptions {
  parentId?: string;
  filter?: string;
  orderBy?: string;
  pageSize?: number;
  pageToken?: string;
  deleted?: boolean;
}

function toResourceName(plural: string, idOrName: string): string {
  return idOrName.includes("/") ? idOrName : `${plural}/${idOrName}`;
}

function isDefaultBaseUrl(baseUrl: string): boolean {
  return baseUrl.replace(/\/$/, "") === TreasuryClient.defaultBaseUrl.replace(/\/$/, "");
}

export function normalizeApiKey(apiKey: string): string {
  const trimmed = apiKey.trim();
  if (!trimmed) {
    throw new Error("apiKey must not be empty");
  }
  const normalized = trimmed.replace(/=+$/, "");
  try {
    const encoded = Buffer.from(trimmed, "base64").toString("base64").replace(/=+$/, "");
    if (encoded === normalized) {
      return trimmed;
    }
  } catch {
    // Fall through and encode below.
  }
  return Buffer.from(trimmed, "utf8").toString("base64");
}

export async function lookupTreasuryId(
  baseUrl = TreasuryClient.defaultBaseUrl,
  opts?: { apiKey?: string; timeout?: number },
): Promise<string> {
  const url = new URL("/v1/treasury", baseUrl);
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), opts?.timeout ?? 30000);
  try {
    const headers: Record<string, string> = {};
    if (opts?.apiKey) {
      headers["Authorization"] = `Bearer ${normalizeApiKey(opts.apiKey)}`;
    }
    const response = await fetch(url.toString(), { headers, signal: controller.signal });
    if (!response.ok) {
      throw new Error(`Unable to look up treasury ID: HTTP ${response.status}`);
    }
    const body = await response.json() as { name?: string };
    const name = body.name;
    if (!name?.startsWith("treasuries/")) {
      throw new Error("Unable to look up treasury ID: response did not include a treasury name");
    }
    return name.slice("treasuries/".length);
  } finally {
    clearTimeout(timeoutId);
  }
}
