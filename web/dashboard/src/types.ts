// Stable, sanitized JSON contracts from internal/model and internal/server.
// Go slices can serialize as null; consumers must not assume an array.
export type Items<T> = T[] | null;
export type Tier = "VERIFIED" | "PREVIEW";
export type Evidence = "CONFIRMED" | "LIKELY" | "UNKNOWN";
export type Severity = "HIGH" | "MEDIUM" | "LOW" | "INFO";
export type Scope = "SYSTEM" | "MANAGED" | "USER" | "PROFILE" | "PROJECT" | "NESTED" | "LOCAL" | "BUILTIN";
export type NodeType = "CLIENT" | "SCOPE" | "SOURCE" | "INSTRUCTION" | "RULE" | "SKILL" | "HOOK" | "MCP_SERVER" | "ENVIRONMENT_REFERENCE" | "EFFECTIVE_OUTPUT";
export type EdgeType = "LOADS" | "IMPORTS" | "OVERRIDES" | "SHADOWS" | "DUPLICATES" | "REFERENCES" | "EFFECTIVE_AS";
export interface SafeValue { kind: string; display: string; present: boolean; secret_category?: string }
export interface Origin { source_id: string; logical_path?: string; line?: number; column?: number; field_path?: string; scope: Scope; precedence_rank?: number; rule: string }
export interface ConfigNode { id: string; type: NodeType; client: string; display_name: string; attributes?: Record<string, SafeValue>; origins?: Items<Origin>; load_condition?: string; adapter_confidence: Evidence }
export interface Edge { id: string; from: string; to: string; type: EdgeType; ruleset: string; evidence: Evidence; origin?: Origin }
export interface ConfigSource { id: string; logical_path: string; canonical_path?: string; client: string; scope: Scope; format: "JSON" | "JSONC" | "TOML" | "YAML" | "MARKDOWN" | "UNKNOWN"; exists: boolean; readable: boolean; kind: string; discovery_reason: string; symlink: boolean }
export interface Finding { rule_id: string; severity: Severity; evidence: Evidence; summary: string; reason: string; impact: string; affected_clients?: Items<string>; origins?: Items<Origin>; graph_references?: Items<string>; remediation: string; fix_plan_id?: string }
export interface ContextEstimate { client: string; source_id: string; category: string; bytes: number; code_points: number; min_tokens: number; max_tokens: number; method: string }
export interface Edit { source_id: string; target_path: string; expected_hash: string; operation: string; start_offset?: number; end_offset?: number; replacement?: string; original_mode?: number; result_mode?: number; redacted_patch: string }
export interface FixPlan { id: string; finding_rule_id: string; risk: "SAFE" | "REVIEW" | "MANUAL"; edits: Items<Edit>; preconditions: Items<string>; postconditions: Items<string>; backup_required: boolean; rollback: Items<string> }
export interface EffectiveConfig { client: string; sources: Items<ConfigSource>; nodes: Items<ConfigNode>; edges: Items<Edge>; findings: Items<Finding>; limitations?: Items<string> }
export interface ClientResult {
  id: string;
  detection: { installed: boolean; executable?: string; version?: string; error?: string };
  capabilities: { precedence: boolean; instructions: boolean; skills: boolean; hooks: boolean; mcp: boolean };
  compatibility: { tier: Tier; state: "VERIFIED" | "COMPATIBILITY_UNKNOWN"; installed_version?: string; verified_versions?: Items<string>; ruleset_version: string; last_verification_at: string; evidence_references?: Items<string>; supported_features?: Items<string> };
  effective: EffectiveConfig;
}
export interface Analysis { clients: Items<ClientResult>; sources: Items<ConfigSource>; graph: { nodes: Items<ConfigNode>; edges: Items<Edge> }; findings: Items<Finding>; context?: Items<ContextEstimate>; fix_plans?: Items<FixPlan> }
export interface ScanResult { schema_version: string; analysis: Analysis; run_metadata?: { generated_at?: string; cwd?: string; os?: string } }
export interface BackupSummary { id: string; created_at: string; file_count: number }
export interface SnapshotMetadata { name: string; schema_version: string; created_at: string }
export interface DriftChange { kind: "ADDED" | "REMOVED" | "CHANGED"; entity_type: "CLIENT" | "SOURCE" | "NODE" | "FINDING" | "COMPATIBILITY"; id: string; summary: string; clients?: Items<string> }
export interface Drift { schema_version: string; baseline?: string; changes: Items<DriftChange> }
export interface DashboardState { schema_version: string; revision: number; scanned_at: string; workspace: string; result: ScanResult; fix_plans: Items<FixPlan>; backups: Items<BackupSummary>; drift?: Drift }
export interface APIErrorEnvelope { code: string; message: string; details: Record<string, string> }
export interface ExplainResponse { node: ConfigNode; edges: Items<Edge> }
export interface ComparisonRow { name: string; type: NodeType; status: string; left?: ConfigNode; right?: ConfigNode }
export interface ComparisonResponse { left: string; right: string; rows: Items<ComparisonRow> }
export type Route = "overview" | "graph" | "findings" | "compare" | "fixes" | "drift" | "export";
export interface Filters { text: string; client: string; scope: string; evidence: string; severity: string }
export interface ViewState { graph: Filters; findings: Filters; left: string; right: string }
export interface UIState { dashboard: DashboardState | null; route: Route; selection: string | null; busy: boolean; error: string | null; views?: ViewState }
