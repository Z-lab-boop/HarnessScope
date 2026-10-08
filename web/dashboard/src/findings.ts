import type { UIState } from "./types.js";
import { badge, button, element, filters, initialViews, type Actions } from "./views.js";

export function renderFindings(state: Readonly<UIState>, actions: Actions): HTMLElement {
  const panel = element("section"); panel.setAttribute("aria-label", "Findings workspace"); panel.append(filters(state, "findings", actions));
  const filter = (state.views ?? initialViews()).findings;
  const findings = (state.dashboard?.result.analysis.findings ?? []).filter((finding) => (!filter.text || `${finding.rule_id} ${finding.summary} ${finding.reason} ${finding.impact} ${finding.remediation}`.toLowerCase().includes(filter.text.toLowerCase())) && (!filter.client || finding.affected_clients?.includes(filter.client)) && (!filter.severity || finding.severity === filter.severity) && (!filter.evidence || finding.evidence === filter.evidence));
  if (!findings.length) panel.append(element("p", "No findings match these filters.", "empty"));
  for (const finding of findings) {
    const card = element("article", "", "finding-card");
    const meta = element("div", "", "finding-meta"); meta.append(badge(finding.severity), badge(finding.evidence), element("code", finding.rule_id));
    card.append(meta, element("h2", finding.summary), element("p", finding.reason), element("p", `Impact: ${finding.impact}`, "muted"), element("p", `Remediation: ${finding.remediation}`, "muted"));
    for (const id of finding.graph_references ?? []) {
      const node = state.dashboard?.result.analysis.graph.nodes?.find((node) => node.id === id);
      if (node) card.append(button(`Focus ${node.display_name} in graph`, () => actions.focusNode(id)));
    }
    panel.append(card);
  }
  return panel;
}
