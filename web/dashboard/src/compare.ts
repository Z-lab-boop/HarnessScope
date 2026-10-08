import type { ConfigNode, UIState } from "./types.js";
import { element, initialViews, safeValue, select, type Actions } from "./views.js";

function signature(nodes: ConfigNode[]): string {
  return JSON.stringify(nodes.map((node) => JSON.stringify(Object.entries(node.attributes ?? {}).sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0).map(([key, value]) => [key, value.kind, value.present, value.secret_category ?? "", safeValue(value)]))).sort());
}
export function normalize(nodes: ConfigNode[], left: string, right: string) {
  const rows = new Map<string, { name: string; type: string; left: ConfigNode[]; right: ConfigNode[] }>();
  for (const node of nodes) {
    if (![left, right].includes(node.client) || ["CLIENT", "SOURCE"].includes(node.type) || !node.display_name) continue;
    const key = JSON.stringify([node.type, node.display_name]);
    const row = rows.get(key) ?? { name: node.display_name, type: node.type, left: [], right: [] };
    if (node.client === left) row.left.push(node);
    if (node.client === right) row.right.push(node);
    rows.set(key, row);
  }
  return [...rows.entries()].sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0).map(([, row]) => ({ ...row, status: !row.left.length || !row.right.length ? "Missing" : signature(row.left) === signature(row.right) ? "Present" : "Divergent" }));
}
export function renderCompare(state: Readonly<UIState>, actions: Actions): HTMLElement {
  const panel = element("section"); panel.setAttribute("aria-label", "Compare workspace");
  const clients = (state.dashboard?.result.analysis.clients ?? []).map((client) => client.id);
  if (clients.length < 2) { panel.append(element("p", "Two clients are needed to compare this snapshot.", "empty")); return panel; }
  const views = state.views ?? initialViews(), left = clients.includes(views.left) ? views.left : clients[0], right = clients.includes(views.right) ? views.right : clients[1];
  const controls = element("div", "", "filters");
  controls.append(select("Left client", clients, left, (value) => actions.update({ views: { ...views, left: value } }), false), select("Right client", clients, right, (value) => actions.update({ views: { ...views, right: value } }), false)); panel.append(controls);
  panel.append(element("p", "Matched by type and display name. Comparison uses redacted attributes; hidden secret values cannot be compared.", "muted"));
  const table = element("table", "", "compare-table"), head = element("thead"), header = element("tr");
  table.append(element("caption", `Configuration comparison: ${left} and ${right}`));
  for (const title of ["Type / name", left, right, "Result"]) { const cell = element("th", title); cell.scope = "col"; header.append(cell); } head.append(header); table.append(head);
  const body = element("tbody");
  const rows = normalize(state.dashboard?.result.analysis.graph.nodes ?? [], left, right);
  for (const row of rows) {
    const tr = element("tr"), name = element("th", `${row.type} ${row.name}`); name.scope = "row";
    tr.append(name, element("td", row.left.length ? "Present" : `Missing in ${left}`), element("td", row.right.length ? "Present" : `Missing in ${right}`), element("td", row.status, row.status === "Divergent" ? "risk-text" : "")); body.append(tr);
  }
  table.append(body); panel.append(table);
  if (!rows.length) panel.append(element("p", "No comparable configuration entries.", "empty"));
  return panel;
}
