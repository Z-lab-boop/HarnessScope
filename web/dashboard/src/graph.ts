import type { ConfigNode, EdgeType, UIState } from "./types.js";
import { button, element, filters, initialViews, safeText, type Actions } from "./views.js";

export interface PositionedNode extends ConfigNode { x: number; y: number }
export function layout(nodes: ConfigNode[]): PositionedNode[] {
  const counts = new Map<number, number>();
  return [...nodes].sort((a, b) => a.id < b.id ? -1 : a.id > b.id ? 1 : 0).map((node) => {
    const column = node.type === "CLIENT" ? 0 : node.type === "SOURCE" ? 1 : 2;
    const row = counts.get(column) ?? 0; counts.set(column, row + 1);
    return { ...node, x: 120 + column * 300, y: 70 + row * 76 };
  });
}
function svg<K extends keyof SVGElementTagNameMap>(tag: K, attrs: Record<string, string> = {}): SVGElementTagNameMap[K] {
  const node = document.createElementNS("http://www.w3.org/2000/svg", tag);
  for (const [key, value] of Object.entries(attrs)) node.setAttribute(key, value);
  return node;
}
const patterns: Record<EdgeType, string> = { LOADS: "none", IMPORTS: "8 3", OVERRIDES: "3 3", SHADOWS: "10 3 2 3", DUPLICATES: "1 4", REFERENCES: "12 5", EFFECTIVE_AS: "6 2 1 2" };
export function renderGraph(state: Readonly<UIState>, actions: Actions): HTMLElement {
  const panel = element("section", "", "graph-panel"); panel.setAttribute("aria-label", "Graph workspace");
  const all = layout(state.dashboard?.result.analysis.graph.nodes ?? []);
  const filter = (state.views ?? initialViews()).graph;
  const nodes = all.filter((node) => (!filter.text || `${node.display_name} ${node.id} ${node.type}`.toLowerCase().includes(filter.text.toLowerCase())) && (!filter.client || node.client === filter.client) && (!filter.evidence || node.adapter_confidence === filter.evidence) && (!filter.scope || node.origins?.some((origin) => origin.scope === filter.scope) || state.dashboard?.result.analysis.sources?.some((source) => source.id === node.id && source.scope === filter.scope)));
  panel.append(filters(state, "graph", actions));
  const toolbar = element("div", "", "graph-tools");
  const output = element("output", "100%", "zoom-reading"); output.setAttribute("aria-label", "Graph zoom");
  const height = Math.max(440, ...all.map((node) => node.y + 65));
  const canvas = svg("svg", { viewBox: `0 0 870 ${height}`, role: "group", "aria-label": "Configuration graph", class: "graph-canvas" });
  const description = svg("desc"); description.textContent = "Configuration provenance. Use arrow keys to select nodes and Enter to inspect. Drag the background to pan; use zoom controls or the wheel to zoom."; canvas.append(description);
  const viewport = svg("g", { "data-graph-viewport": "", transform: "translate(0 0) scale(1)" });
  canvas.append(viewport);
  let x = 0, y = 0, scale = 1;
  const transform = () => { viewport.setAttribute("transform", `translate(${x} ${y}) scale(${scale})`); output.textContent = `${Math.round(scale * 100)}%`; };
  const zoom = (amount: number) => { scale = Math.max(.5, Math.min(2.5, Math.round((scale + amount) * 100) / 100)); transform(); };
  toolbar.append(button("Zoom in", () => zoom(.2)), button("Zoom out", () => zoom(-.2)), button("Reset graph view", () => { x = y = 0; scale = 1; transform(); }), output, element("span", `${nodes.length} / ${all.length} nodes`, "muted")); panel.append(toolbar);
  canvas.addEventListener("wheel", (event) => { event.preventDefault(); zoom(event.deltaY < 0 ? .1 : -.1); }, { passive: false });
  let drag: { id: number; x: number; y: number } | null = null;
  canvas.addEventListener("pointerdown", (event) => {
    if (event.button !== 0 || drag || (event.target as Element).closest("[data-node-id]")) return;
    drag = { id: event.pointerId, x: event.clientX, y: event.clientY }; canvas.setPointerCapture(event.pointerId);
  });
  canvas.addEventListener("pointermove", (event) => {
    if (!drag || drag.id !== event.pointerId) return;
    const matrix = canvas.getScreenCTM(); if (!matrix) return;
    x += (event.clientX - drag.x) / matrix.a; y += (event.clientY - drag.y) / matrix.d;
    drag.x = event.clientX; drag.y = event.clientY; transform();
  });
  const release = (event: PointerEvent) => {
    if (drag?.id !== event.pointerId) return;
    drag = null;
    if (canvas.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId);
  };
  canvas.addEventListener("pointerup", release); canvas.addEventListener("pointercancel", release); canvas.addEventListener("lostpointercapture", release);
  const visible = new Map(nodes.map((node) => [node.id, node]));
  for (const edge of state.dashboard?.result.analysis.graph.edges ?? []) {
    const from = visible.get(edge.from), to = visible.get(edge.to); if (!from || !to) continue;
    const path = svg("path", { d: `M ${from.x + 110} ${from.y} C ${from.x + 160} ${from.y}, ${to.x - 160} ${to.y}, ${to.x - 110} ${to.y}`, "data-edge-id": edge.id, "stroke-dasharray": patterns[edge.type], opacity: edge.evidence === "CONFIRMED" ? "1" : edge.evidence === "LIKELY" ? ".65" : ".35", class: "graph-edge", role: "img", "aria-label": safeText(`${edge.type}: ${from.display_name} to ${to.display_name}; ${edge.evidence}`) });
    viewport.append(path);
  }
  const groups: SVGGElement[] = [];
  for (const node of nodes) {
    const group = svg("g", { transform: `translate(${node.x} ${node.y})`, "data-node-id": node.id, class: "graph-node", role: "button", tabindex: node.id === state.selection || (!state.selection && !groups.length) ? "0" : "-1", "aria-pressed": String(node.id === state.selection), "aria-label": safeText(`${node.display_name}; ${node.type}; ${node.client}; ${node.adapter_confidence}`) });
    const title = svg("title"); title.textContent = safeText(`${node.display_name} · ${node.type} · ${node.adapter_confidence}`);
    const rect = svg("rect", { x: "-110", y: "-27", width: "220", height: "54", rx: "3" });
    const label = svg("text", { x: "-96", y: "-3", class: "node-label" }); const name = safeText(node.display_name); label.textContent = name.length > 24 ? name.slice(0, 23) + "…" : name;
    const meta = svg("text", { x: "-96", y: "15", class: "node-meta" }); meta.textContent = `${node.type} / ${node.adapter_confidence}`;
    group.append(title, rect, label, meta);
    group.addEventListener("click", () => { actions.update({ selection: node.id }); group.focus(); });
    group.addEventListener("keydown", (event) => {
      if (event.key === "Enter" || event.key === " ") { event.preventDefault(); actions.update({ selection: node.id }); }
      const direction = ["ArrowDown", "ArrowRight"].includes(event.key) ? 1 : ["ArrowUp", "ArrowLeft"].includes(event.key) ? -1 : 0;
      if (direction) { event.preventDefault(); const next = (groups.indexOf(group) + direction + groups.length) % groups.length; actions.update({ selection: nodes[next].id }); groups[next].focus(); }
    });
    groups.push(group); viewport.append(group);
  }
  if (!nodes.length) panel.append(element("p", "No nodes match these filters.", "empty"));
  panel.append(canvas, element("p", "CLIENT → SOURCE → CONFIGURATION · Edge dashes identify relationships; opacity reflects evidence. Arrow keys select · Enter inspects.", "graph-legend muted"));
  const legend = element("div", "", "edge-legend");
  for (const [kind, pattern] of Object.entries(patterns)) { const item = element("span", kind); const swatch = svg("svg", { viewBox: "0 0 32 8", "aria-hidden": "true" }); swatch.append(svg("path", { d: "M 0 4 H 32", "stroke-dasharray": pattern, class: "graph-edge" })); item.prepend(swatch); legend.append(item); }
  panel.append(legend); return panel;
}
