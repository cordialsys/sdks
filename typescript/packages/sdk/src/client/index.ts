/**
 * Treasury API Client.
 */
import { SigningKey } from "../crypto/keys.js";
import { signRequest } from "../signature/index.js";
import { buildResourcePath, actionMethod, getResourceType } from "./resource-types.js";

export { buildResourcePath, actionMethod, getResourceType, getResourceTypeBySingular, getResourceTypeByPlural, singularToPlural, pluralToSingular, parseResourceName, allResourceTypes } from "./resource-types.js";
export type { ResourceTypeMeta } from "./resource-types.js";

export interface TreasuryClientOptions {
  baseUrl: string;
  treasuryId: string;
  hostId?: string;
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
  readonly baseUrl: string;
  readonly treasuryId: string;
  readonly hostId?: string;
  private _signingKey?: SigningKey;
  private _timeout: number;

  constructor(opts: TreasuryClientOptions) {
    this.baseUrl = opts.baseUrl.replace(/\/$/, "");
    this.treasuryId = opts.treasuryId;
    this.hostId = opts.hostId;
    this._signingKey = opts.signingKey;
    this._timeout = opts.timeout ?? 30000;
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
      treasury: this.treasuryId,
    };

    if (this.hostId) {
      headers["treasury-host"] = this.hostId;
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
    const body = JSON.stringify(data);
    return this.extractOpName(await this.request({ method: "PUT", path, body }));
  }

  /**
   * Delete a resource. Returns the operation name.
   */
  async delete(resourceName: string): Promise<string> {
    const path = `/${resourceName}`;
    return this.extractOpName(await this.request({ method: "DELETE", path }));
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
