import type { Filters, SafeValue, UIState, ViewState } from "./types.js";

export const emptyFilters = (): Filters => ({ text: "", client: "", scope: "", evidence: "", severity: "" });
export const initialViews = (): ViewState => ({ graph: emptyFilters(), findings: emptyFilters(), left: "", right: "" });
export interface Actions { update: (change: Partial<UIState>) => void; focusNode: (id: string) => void }
// The API owns redaction. These checks also fail closed for unsafe origin paths
// or secret-tagged fields, without turning arbitrary strings into markup.
export function safeText(value: string): string { return value.replace(/\/(?:Users|home)\/[^\s<>"']+/g, "[REDACTED PATH]"); }
export function safeValue(value: SafeValue): string {
  return value.secret_category || /secret|credential|redact/i.test(value.kind) ? "[REDACTED]" : safeText(value.display);
}
export function element<K extends keyof HTMLElementTagNameMap>(tag: K, text = "", className = ""): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  node.textContent = safeText(text);
  node.className = className;
  return node;
}
export function badge(text: string): HTMLElement { return element("span", text, `badge ${text === "CONFIRMED" ? "verified" : text === "HIGH" ? "high" : "preview"}`); }
export function button(text: string, action: () => void): HTMLButtonElement {
  const node = element("button", text, "view-button"); node.type = "button"; node.addEventListener("click", action); return node;
}
export function select(label: string, options: string[], value: string, change: (value: string) => void, all = true): HTMLLabelElement {
  const wrapper = element("label", label, "filter-control");
  const input = element("select");
  input.dataset.control = label;
  for (const option of [...(all ? [""] : []), ...options]) {
    const item = element("option", option || "All"); item.value = option; input.append(item);
  }
  input.value = value;
  input.addEventListener("change", () => change(input.value));
  wrapper.append(input); return wrapper;
}
export function filters(state: Readonly<UIState>, kind: "graph" | "findings", actions: Actions): HTMLElement {
  const views = state.views ?? initialViews();
  const current = views[kind];
  const update = (key: keyof Filters, value: string) => actions.update({ views: { ...views, [kind]: { ...current, [key]: value } } });
  const bar = element("div", "", "filters");
  const label = element("label", `Search ${kind}`, "filter-control search-control");
  const input = element("input"); input.type = "search"; input.value = current.text; input.dataset.control = `Search ${kind}`;
  input.addEventListener("input", () => update("text", input.value)); label.append(input); bar.append(label);
  const prefix = kind === "graph" ? "Graph" : "Finding";
  bar.append(select(`${prefix} client`, (state.dashboard?.result.analysis.clients ?? []).map((client) => client.id), current.client, (value) => update("client", value)));
  if (kind === "graph") bar.append(select("Graph scope", ["SYSTEM", "MANAGED", "USER", "PROFILE", "PROJECT", "NESTED", "LOCAL", "BUILTIN"], current.scope, (value) => update("scope", value)));
  else bar.append(select("Finding severity", ["HIGH", "MEDIUM", "LOW", "INFO"], current.severity, (value) => update("severity", value)));
  bar.append(select(`${prefix} evidence`, ["CONFIRMED", "LIKELY", "UNKNOWN"], current.evidence, (value) => update("evidence", value)));
  return bar;
}
