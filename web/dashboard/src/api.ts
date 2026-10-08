import type { APIErrorEnvelope, BackupSummary, ComparisonResponse, DashboardState, ExplainResponse, FixPlan, Items, SnapshotMetadata } from "./types.js";

export class APIError extends Error {
  constructor(message: string, readonly status: number, readonly code: string) { super(message); this.name = "APIError"; }
}

// Called exactly once during boot. The token lives only in the API instance.
export function consumeFragmentToken(): string {
  const params = new URLSearchParams(window.location.hash.slice(1));
  window.history.replaceState(null, "", window.location.pathname + window.location.search);
  return params.getAll("token").length === 1 ? params.get("token") ?? "" : "";
}

export class APIClient {
  constructor(private readonly token: string, private readonly base = "") {}
  state(): Promise<DashboardState> { return this.request("/api/v1/state"); }
  explain(id: string): Promise<ExplainResponse> { return this.request(`/api/v1/explain?${new URLSearchParams({ id })}`); }
  compare(left: string, right: string): Promise<ComparisonResponse> { return this.request(`/api/v1/compare?${new URLSearchParams({ left, right })}`); }
  backups(): Promise<Items<BackupSummary>> { return this.request("/api/v1/backups"); }
  snapshots(): Promise<Items<SnapshotMetadata>> { return this.request("/api/v1/snapshots"); }
  rescan(revision: number): Promise<DashboardState> { return this.request("/api/v1/rescan", { revision }); }
  planFixes(revision: number): Promise<Items<FixPlan>> { return this.request("/api/v1/fixes/plan", { revision }); }
  applyFixes(revision: number, fixIDs: string[]): Promise<DashboardState> { return this.request("/api/v1/fixes/apply", { revision, fix_ids: fixIDs }); }
  rollback(revision: number, backupID: string): Promise<DashboardState> { return this.request("/api/v1/rollback", { revision, backup_id: backupID }); }
  saveSnapshot(revision: number, name: string): Promise<SnapshotMetadata> { return this.request("/api/v1/snapshots", { revision, name }); }
  drift(revision: number, baseline: string): Promise<DashboardState> { return this.request("/api/v1/drift", { revision, baseline }); }
  async export(revision: number, baseline?: string): Promise<Blob> {
    const response = await this.fetch("/api/v1/export", { revision, ...(baseline ? { baseline } : {}) });
    if (response.headers.get("content-type")?.split(";")[0].trim() !== "application/zip") throw new APIError("Expected a ZIP export.", response.status, "invalid_response");
    return response.blob();
  }
  private async request<T>(path: string, body?: object): Promise<T> {
    const response = await this.fetch(path, body);
    return this.json(response) as Promise<T>;
  }
  private async json(response: Response): Promise<unknown> {
    if (response.headers.get("content-type")?.split(";")[0].trim() !== "application/json") throw new APIError("Expected a JSON response from the local server.", response.status, "invalid_response");
    try { return await response.json(); }
    catch { throw new APIError("The local server returned invalid JSON.", response.status, "invalid_response"); }
  }
  private async fetch(path: string, body?: object): Promise<Response> {
    if (!this.token) throw new APIError("Open the launch URL printed by hscope serve to start an authenticated session.", 401, "missing_token");
    const url = new URL(this.base + path, window.location.origin);
    if (url.origin !== window.location.origin || url.protocol !== "http:" || !["127.0.0.1", "localhost", "[::1]"].includes(url.hostname) || url.username || url.password) {
      throw new APIError("Only same-origin loopback API requests are allowed.", 0, "invalid_origin");
    }
    let response: Response;
    try {
      response = await window.fetch(url, {
        method: body ? "POST" : "GET",
        headers: { "X-HarnessScope-Token": this.token, Accept: "application/json, application/zip", ...(body ? { "Content-Type": "application/json" } : {}) },
        ...(body ? { body: JSON.stringify(body) } : {}),
        credentials: "omit", mode: "same-origin", redirect: "error", cache: "no-store", referrerPolicy: "no-referrer",
      });
    } catch { throw new APIError("Cannot reach the local server. Check that hscope serve is still running.", 0, "network_error"); }
    if (!response.ok) {
      const value = await this.json(response) as Partial<APIErrorEnvelope> | null;
      if (!value || typeof value.code !== "string" || typeof value.message !== "string" || !value.details || typeof value.details !== "object" || Array.isArray(value.details) || Object.values(value.details).some((item) => typeof item !== "string")) {
        throw new APIError("The local server returned an invalid JSON error envelope.", response.status, "invalid_response");
      }
      throw new APIError(value.message.split(this.token).join("[REDACTED]"), response.status, value.code);
    }
    return response;
  }
}
