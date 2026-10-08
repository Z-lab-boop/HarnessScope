import type { UIState } from "./types.js";
import { badge, element, safeValue } from "./views.js";

export function renderInspector(state: Readonly<UIState>): DocumentFragment {
  const panel = document.createDocumentFragment(); panel.append(element("p", "DETAIL CHANNEL", "eyebrow"), element("h2", "Inspector"));
  const analysis = state.dashboard?.result.analysis;
  const node = analysis?.graph.nodes?.find((node) => node.id === state.selection);
  const client = analysis?.clients?.find((client) => client.id === state.selection);
  const details = element("dl");
  const add = (label: string, value: string) => details.append(element("dt", label), element("dd", value));
  if (node) {
    panel.append(element("h3", node.display_name), badge(node.adapter_confidence));
    add("Type", node.type); add("Client", node.client); add("Load condition", node.load_condition || "Not recorded");
    panel.append(details, element("h3", "Redacted attributes"));
    const attributes = element("dl");
    for (const [key, value] of Object.entries(node.attributes ?? {}).sort(([a], [b]) => a.localeCompare(b))) attributes.append(element("dt", key), element("dd", safeValue(value)));
    panel.append(attributes);
    if (!Object.keys(node.attributes ?? {}).length) panel.append(element("p", "No attributes recorded.", "muted"));
    panel.append(element("h3", "Ordered origins"));
    const origins = element("ol", "", "origin-list");
    for (const origin of [...(node.origins ?? [])].sort((a, b) => (a.precedence_rank ?? Infinity) - (b.precedence_rank ?? Infinity) || a.source_id.localeCompare(b.source_id))) {
      const item = element("li");
      const source = analysis?.sources?.find((source) => source.id === origin.source_id);
      const path = origin.logical_path || source?.logical_path;
      const safePath = path && /^(?:\.(?:\/|$)|~(?:\/|$))/.test(path) ? path : "[REDACTED PATH]";
      item.append(element("code", safePath), element("p", `${origin.scope} · Rank ${origin.precedence_rank ?? "unknown"}`), element("p", `${origin.rule}${origin.line ? ` · Line ${origin.line}${origin.column ? `:${origin.column}` : ""}` : ""}`));
      if (origin.field_path) item.append(element("p", origin.field_path));
      origins.append(item);
    }
    panel.append(origins);
    if (!origins.children.length) panel.append(element("p", "No origins recorded.", "muted"));
  } else if (client) {
    panel.append(element("h3", client.id), element("span", client.compatibility.tier, `badge ${client.compatibility.tier === "PREVIEW" ? "preview" : "verified"}`));
    add("Detection", client.detection.installed ? "Installed" : "Not detected"); add("Compatibility", client.compatibility.state); add("Ruleset", client.compatibility.ruleset_version); add("Last verification", client.compatibility.last_verification_at || "Not recorded"); panel.append(details);
    if (client.effective.limitations?.length) { panel.append(element("h3", "Limitations")); const list = element("ul"); for (const limitation of client.effective.limitations) list.append(element("li", limitation)); panel.append(list); }
  } else {
    const reticle = element("div", "+", "reticle"); reticle.setAttribute("aria-hidden", "true");
    panel.append(reticle, element("h3", "Follow a signal"), element("p", "Select a client or graph node to inspect its evidence and origins.", "muted"));
  }
  return panel;
}
