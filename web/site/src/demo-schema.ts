export type Evidence = "CONFIRMED" | "LIKELY" | "UNKNOWN";
export interface DemoNode { id: string; label: string; kind: "client" | "source" | "rule" | "mcp"; client: "codex" | "claude" | "cursor" | "opencode"; evidence: Evidence; origin: string; }
export interface DemoEdge { from: string; to: string; relation: "loads" | "overrides" | "duplicates"; evidence: Evidence; }
export interface Demo { schema_version: "1"; nodes: DemoNode[]; edges: DemoEdge[]; conflict: { title: string; node_ids: string[]; }; fix_preview: { title: string; risk: "SAFE"; patch: string[]; }; }

function invalid(): never { throw new Error("Invalid synthetic demo"); }
function record(value: unknown, keys: string[]): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) invalid();
  const object = value as Record<string, unknown>;
  if (Object.keys(object).length !== keys.length || keys.some(key => !Object.prototype.hasOwnProperty.call(object, key))) invalid();
  return object;
}
function text(value: unknown): string {
  if (typeof value !== "string" || !value.trim() || value.length > 300 || /[\u0000-\u0008\u000b-\u001f\u007f]/.test(value)) invalid();
  // Validate every displayable string, including patch lines and finding titles.
  if (/(?:^|[^a-z\d_.-])(?:\/|[a-z]:[\\/]|~(?:[^\s]*[\\/]|$)|\\)/i.test(value)
    || /(?:[a-z][a-z\d+.-]*:\/\/|\b(?:https?|ftp|file|data|mailto|javascript|ssh):|www\.|[\w.+-]+@[\w.-]+\.[a-z]{2,})/i.test(value)
    || /(?:sk-|gh[pousr]_|github_pat_|AKIA[A-Z\d]{12}|ASIA[A-Z\d]{12}|xox[baprs]-|-----BEGIN .*PRIVATE KEY|\bBearer\s+\S+|\beyJ[\w-]+\.[\w-]+\.[\w-]+|\b(?:session[_-]?token|api[_-]?key|access[_-]?token|password|secret)\s*[:=]\s*\S+)/i.test(value)) invalid();
  return value;
}
function choice<T extends string>(value: unknown, allowed: readonly T[]): T {
  const result = text(value);
  if (!allowed.includes(result as T)) invalid();
  return result as T;
}
function id(value: unknown): string {
  const result = text(value);
  if (!/^[a-z][a-z\d-]{0,59}$/.test(result)) invalid();
  return result;
}
function array(value: unknown): unknown[] {
  if (!Array.isArray(value) || value.length < 1 || value.length > 40) invalid();
  return value;
}
const evidence = (value: unknown) => choice(value, ["CONFIRMED", "LIKELY", "UNKNOWN"] as const);
const compare = (a: string, b: string) => a < b ? -1 : a > b ? 1 : 0;

export function parseDemo(input: unknown): Demo {
  const root = record(input, ["schema_version", "nodes", "edges", "conflict", "fix_preview"]);
  const schema_version = choice(root.schema_version, ["1"] as const);
  const nodes: DemoNode[] = array(root.nodes).map(value => {
    const node = record(value, ["id", "label", "kind", "client", "evidence", "origin"]);
    const origin = text(node.origin);
    if (!/^\.\/(?:[a-z\d_.-]+\/)*[a-z\d_.-]+$/i.test(origin) || origin.split("/").slice(1).some(part => part === ".." || part === ".")) invalid();
    return { id: id(node.id), label: text(node.label), kind: choice(node.kind, ["client", "source", "rule", "mcp"] as const), client: choice(node.client, ["codex", "claude", "cursor", "opencode"] as const), evidence: evidence(node.evidence), origin };
  });
  const ids = new Set(nodes.map(node => node.id));
  if (ids.size !== nodes.length) invalid();
  const reference = (value: unknown) => { const result = id(value); if (!ids.has(result)) invalid(); return result; };
  const edges: DemoEdge[] = array(root.edges).map(value => {
    const edge = record(value, ["from", "to", "relation", "evidence"]);
    return { from: reference(edge.from), to: reference(edge.to), relation: choice(edge.relation, ["loads", "overrides", "duplicates"] as const), evidence: evidence(edge.evidence) };
  });
  const conflict = record(root.conflict, ["title", "node_ids"]);
  const node_ids = array(conflict.node_ids).map(reference);
  if (node_ids.length < 2 || new Set(node_ids).size !== node_ids.length) invalid();
  const fix = record(root.fix_preview, ["title", "risk", "patch"]);
  return {
    schema_version, nodes: nodes.sort((a, b) => compare(a.id, b.id)),
    edges: edges.sort((a, b) => compare([a.from, a.to, a.relation, a.evidence].join("\0"), [b.from, b.to, b.relation, b.evidence].join("\0"))),
    conflict: { title: text(conflict.title), node_ids: node_ids.sort(compare) },
    fix_preview: { title: text(fix.title), risk: choice(fix.risk, ["SAFE"] as const), patch: array(fix.patch).map(text) },
  };
}
