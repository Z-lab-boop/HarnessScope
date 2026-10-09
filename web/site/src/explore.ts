import { parseDemo, type Demo, type DemoNode } from "./demo-schema.js";
import type { Locale } from "./i18n.js";

export const exploreCopy = {
  en: {
    "explore.kicker": "A small fictional workspace · an evidence trail", "explore.loading": "Loading fictional evidence…",
    "explore.unavailable": "Walkthrough unavailable", "explore.failure": "The fictional dataset could not be validated. No evidence is displayed.", "explore.repository": "Read the repository demo instructions ↗",
    "explore.chapters": "Walkthrough chapters", "explore.discover": "Discover", "explore.trace": "Trace", "explore.resolve": "Resolve",
    "explore.discoverBody": "Start with nine fictional declarations. Select a node to inspect its source; filter the view to narrow the trail.",
    "explore.traceBody": "Follow declared links into the inspector. Evidence describes this fictional dataset; it does not verify a client’s runtime behavior.",
    "explore.resolveBody": "Compare two fictional MCP declarations, then inspect a safe-fix illustration. Every change shown here is a preview.",
    "explore.clientFilter": "Client", "explore.allClients": "All clients", "explore.evidenceFilter": "Evidence", "explore.allEvidence": "All evidence", "explore.confirmed": "CONFIRMED", "explore.likely": "LIKELY", "explore.unknown": "UNKNOWN",
    "explore.showConflicts": "Show conflicts", "explore.reset": "Reset view", "explore.graph": "Fictional evidence graph", "explore.svgLabel": "Interactive fictional graph", "explore.graphKicker": "Declared relationships · synthetic data",
    "explore.legend": "● Confirmed · ◇ Likely · □ Unknown | dashed line: duplicate declaration", "explore.listTitle": "Inspect the evidence", "explore.empty": "No declarations match these filters.", "explore.inspector": "Evidence inspector",
    "explore.boundary": "All sources are fictional relative paths. This page reads its committed demo file only; it does not scan, upload, or modify configuration.",
    "explore.conflictRegion": "Conflict evidence", "explore.comparisonKicker": "Across clients · fictional comparison", "explore.conflictTitle": "Fictional duplicate MCP declaration",
    "explore.conflictBody": "Both sources declare demo-tools. Structural similarity suggests duplication; actual merge behavior is unknown. This comparison always shows both declarations, independently of the graph filters.",
    "explore.preview": "Preview safe fix", "explore.previewRegion": "Safe-fix preview", "explore.fixTitle": "Remove the fictional duplicate declaration", "explore.previewOnly": "Preview only · nothing was changed", "explore.previewOpened": "Safe-fix preview opened · nothing was changed", "explore.risk": "SAFE · fictional plan", "explore.close": "Close preview",
    "explore.inspect": "Inspect", "explore.selectGraph": "Select in graph:", "explore.origin": "Fictional origin", "explore.chain": "Declared origin chain", "explore.noSelection": "Select a declaration to inspect its evidence.", "explore.count": "visible fictional declarations", "explore.selected": "Selected", "explore.loads": "loads", "explore.overrides": "overrides", "explore.duplicates": "duplicates",
    "explore.client": "client", "explore.source": "source", "explore.rule": "rule", "explore.mcp": "MCP declaration",
    "node.project-instructions": "project instructions", "node.codex-mcp": "Codex demo-tools", "node.claude-mcp": "Claude demo-tools", "node.cursor-rule": "Cursor project rule", "node.opencode-instructions": "OpenCode instructions",
  },
  "zh-CN": {
    "explore.kicker": "小型虚构工作区 · 一条证据链", "explore.loading": "正在加载虚构证据…",
    "explore.unavailable": "交互演示暂不可用", "explore.failure": "虚构数据未能通过校验，因此不显示任何证据。", "explore.repository": "阅读仓库演示说明 ↗",
    "explore.chapters": "演示章节", "explore.discover": "发现", "explore.trace": "追溯", "explore.resolve": "解决",
    "explore.discoverBody": "从九条虚构声明开始。选择节点查看来源，通过筛选缩小证据范围。",
    "explore.traceBody": "沿已声明的关系进入检查器。证据描述的是这组虚构数据，不验证客户端的实际运行行为。",
    "explore.resolveBody": "比较两条虚构 MCP 声明，再检查安全修复示例。此处展示的所有更改均为预览。",
    "explore.clientFilter": "客户端", "explore.allClients": "所有客户端", "explore.evidenceFilter": "证据等级", "explore.allEvidence": "所有证据", "explore.confirmed": "已确认", "explore.likely": "可能", "explore.unknown": "未知",
    "explore.showConflicts": "显示冲突", "explore.reset": "重置视图", "explore.graph": "虚构证据关系图", "explore.svgLabel": "可交互虚构关系图", "explore.graphKicker": "已声明的关系 · 合成数据",
    "explore.legend": "● 已确认 · ◇ 可能 · □ 未知 | 虚线：重复声明", "explore.listTitle": "检查证据", "explore.empty": "没有符合筛选条件的声明。", "explore.inspector": "证据检查器",
    "explore.boundary": "所有来源均为虚构相对路径。页面仅读取随站点提供的演示文件，不扫描、上传或修改配置。",
    "explore.conflictRegion": "冲突证据", "explore.comparisonKicker": "跨客户端 · 虚构比较", "explore.conflictTitle": "虚构的重复 MCP 声明",
    "explore.conflictBody": "两个来源都声明了 demo-tools。结构相似提示可能重复，实际合并行为未知。本比较始终展示两条声明，不受关系图筛选影响。",
    "explore.preview": "预览安全修复", "explore.previewRegion": "安全修复预览", "explore.fixTitle": "移除虚构的重复声明", "explore.previewOnly": "仅供预览 · 未做任何更改", "explore.previewOpened": "已打开安全修复预览 · 未做任何更改", "explore.risk": "安全 · 虚构计划", "explore.close": "关闭预览",
    "explore.inspect": "检查", "explore.selectGraph": "在图中选择：", "explore.origin": "虚构来源", "explore.chain": "已声明的来源链", "explore.noSelection": "选择一条声明，检查其证据。", "explore.count": "条可见虚构声明", "explore.selected": "已选择", "explore.loads": "加载", "explore.overrides": "覆盖", "explore.duplicates": "重复",
    "explore.client": "客户端", "explore.source": "来源", "explore.rule": "规则", "explore.mcp": "MCP 声明",
    "node.project-instructions": "项目指令", "node.codex-mcp": "Codex demo-tools", "node.claude-mcp": "Claude demo-tools", "node.cursor-rule": "Cursor 项目规则", "node.opencode-instructions": "OpenCode 指令",
  },
} satisfies Record<Locale, Record<string, string>>;
type Key = keyof typeof exploreCopy.en;
const locale = (): Locale => document.documentElement.lang === "zh-CN" ? "zh-CN" : "en";
const t = (key: Key) => exploreCopy[locale()][key];
function element<K extends keyof HTMLElementTagNameMap>(tag: K, content?: string, className?: string): HTMLElementTagNameMap[K] {
  const item = document.createElement(tag);
  if (content !== undefined) item.textContent = content;
  if (className) item.className = className;
  return item;
}
function label(node: DemoNode): string {
  const key = `node.${node.id}`;
  return Object.prototype.hasOwnProperty.call(exploreCopy.en, key) ? t(key as Key) : node.label;
}
const tier = (value: DemoNode["evidence"]) => t(`explore.${value.toLowerCase()}` as Key);
const symbol = (value: DemoNode["evidence"]) => ({ CONFIRMED: "●", LIKELY: "◇", UNKNOWN: "□" })[value];

export async function initExplore(): Promise<void> {
  if (document.body.dataset.page !== "explore") return;
  const workbench = document.querySelector<HTMLElement>("[data-explore-workbench]")!;
  const status = document.querySelector<HTMLElement>("[data-explore-status]")!;
  let demo: Demo;
  try {
    const response = await fetch("./data/demo.json", { credentials: "omit" });
    if (!response.ok) throw new Error("Unavailable");
    demo = parseDemo(await response.json());
  } catch {
    status.hidden = true;
    document.querySelector<HTMLElement>("[data-explore-error]")!.hidden = false;
    return;
  }
  // No partially validated data reaches the UI.
  status.removeAttribute("data-i18n");
  workbench.hidden = false;
  const graph = document.querySelector<SVGSVGElement>(".explore-graph")!;
  const list = document.querySelector<HTMLElement>("[data-explore-nodes]")!;
  const inspector = document.querySelector<HTMLElement>("[data-explore-inspector]")!;
  const client = document.querySelector<HTMLSelectElement>("[data-client-filter]")!;
  const evidence = document.querySelector<HTMLSelectElement>("[data-evidence-filter]")!;
  const conflict = document.querySelector<HTMLElement>("#conflict-evidence")!;
  const preview = document.querySelector<HTMLElement>("#fix-preview")!;
  const conflictButton = document.querySelector<HTMLButtonElement>("[data-show-conflicts]")!;
  const previewButton = document.querySelector<HTMLButtonElement>("[data-preview-fix]")!;
  const tabs = Array.from(document.querySelectorAll<HTMLButtonElement>("[data-chapter]"));
  let selected: string | undefined = "project-instructions";
  let visible: DemoNode[] = [];
  const byId = new Map(demo.nodes.map(node => [node.id, node]));
  function updateStatus(announceSelection = false): void {
    status.textContent = `${visible.length} / ${demo.nodes.length} ${t("explore.count")}${announceSelection && selected ? ` · ${t("explore.selected")}: ${label(byId.get(selected)!)}` : ""}`;
  }
  function renderInspector(): void {
    inspector.replaceChildren();
    const node = selected ? byId.get(selected) : undefined;
    if (!node) { inspector.append(element("p", t("explore.noSelection"))); return; }
    inspector.append(element("h2", label(node)), element("p", `${symbol(node.evidence)} ${tier(node.evidence)} · ${t(`explore.${node.kind}`)}`, "technical"), element("h3", t("explore.origin")), element("code", node.origin), element("h3", t("explore.chain")));
    const chain = element("ul", undefined, "explore-chain technical");
    const visited = new Set<string>();
    function trace(id: string): void {
      if (visited.has(id)) return;
      visited.add(id);
      demo.edges.filter(edge => edge.to === id && edge.relation !== "duplicates").forEach(edge => {
        trace(edge.from);
        const source = byId.get(edge.from)!;
        chain.append(element("li", `${label(source)} · ${source.origin} → ${t(`explore.${edge.relation}`)} · ${tier(edge.evidence)}`));
      });
    }
    trace(node.id);
    chain.append(element("li", `${label(node)} · ${node.origin}`));
    inspector.append(chain);
    demo.edges.filter(edge => edge.relation === "duplicates" && (edge.from === node.id || edge.to === node.id)).forEach(edge => {
      const other = byId.get(edge.from === node.id ? edge.to : edge.from)!;
      inspector.append(element("p", `${t("explore.duplicates")}: ${label(other)} · ${other.origin} · ${tier(edge.evidence)}`, "technical"));
    });
  }
  function select(id: string): void {
    selected = id;
    workbench.querySelectorAll<HTMLElement | SVGElement>("[data-node-id]").forEach(item => item.setAttribute("aria-pressed", String(item.dataset.nodeId === id)));
    renderInspector();
    updateStatus(true);
  }
  function svg<K extends keyof SVGElementTagNameMap>(tag: K, attrs: Record<string, string>): SVGElementTagNameMap[K] {
    const item = document.createElementNS("http://www.w3.org/2000/svg", tag);
    Object.entries(attrs).forEach(([key, value]) => item.setAttribute(key, value));
    return item;
  }
  function renderGraph(): void {
    visible = demo.nodes.filter(node => (client.value === "all" || node.client === client.value) && (evidence.value === "all" || node.evidence === evidence.value));
    if (!visible.some(node => node.id === selected)) selected = undefined;
    graph.replaceChildren(); list.replaceChildren();
    const positions = new Map<string, { x: number; y: number }>();
    visible.forEach((node, index) => {
      const angle = -Math.PI / 2 + index * Math.PI * 2 / visible.length;
      positions.set(node.id, { x: 320 + 228 * Math.cos(angle), y: 225 + 163 * Math.sin(angle) });
    });
    demo.edges.forEach(edge => {
      const from = positions.get(edge.from), to = positions.get(edge.to);
      if (!from || !to || evidence.value !== "all" && edge.evidence !== evidence.value) return;
      const line = svg("line", { x1: String(from.x), y1: String(from.y), x2: String(to.x), y2: String(to.y), class: `graph-edge ${edge.relation}`, "aria-hidden": "true" });
      graph.append(line);
    });
    visible.forEach(node => {
      const { x, y } = positions.get(node.id)!;
      const group = svg("g", { transform: `translate(${x} ${y})`, role: "button", tabindex: "0", "aria-label": `${t("explore.selectGraph")} ${label(node)} · ${tier(node.evidence)}`, "aria-pressed": String(node.id === selected), "data-node-id": node.id, class: `graph-node ${node.evidence.toLowerCase()}` });
      group.append(svg("circle", { r: "20", class: "graph-hit" }), svg("circle", { r: "9", class: "graph-dot" }));
      const name = svg("text", { y: y < 100 ? "-30" : "36", "text-anchor": "middle", "aria-hidden": "true" });
      name.textContent = `${symbol(node.evidence)} ${label(node)}`; group.append(name);
      group.addEventListener("click", () => select(node.id));
      group.addEventListener("keydown", event => { if (event.key === "Enter" || event.key === " ") { event.preventDefault(); select(node.id); } });
      graph.append(group);
      const item = element("li");
      const button = element("button", `${symbol(node.evidence)} ${label(node)} · ${tier(node.evidence)}`);
      button.type = "button"; button.dataset.nodeId = node.id;
      button.setAttribute("aria-label", `${t("explore.inspect")} ${label(node)}`);
      button.setAttribute("aria-pressed", String(node.id === selected));
      button.addEventListener("click", () => select(node.id));
      item.append(button); list.append(item);
    });
    document.querySelector<HTMLElement>("[data-explore-empty]")!.hidden = visible.length > 0;
    graph.toggleAttribute("hidden", visible.length === 0);
    renderInspector(); updateStatus();
  }
  function chapter(id: string, focus = false): void {
    tabs.forEach(tab => {
      const active = tab.dataset.chapter === id;
      tab.setAttribute("aria-selected", String(active)); tab.tabIndex = active ? 0 : -1;
      document.getElementById(`chapter-${tab.dataset.chapter}`)!.hidden = !active;
      if (active && focus) tab.focus();
    });
  }
  tabs.forEach((tab, index) => {
    tab.addEventListener("click", () => chapter(tab.dataset.chapter!));
    tab.addEventListener("keydown", event => {
      const next = event.key === "ArrowRight" ? (index + 1) % tabs.length : event.key === "ArrowLeft" ? (index + tabs.length - 1) % tabs.length : event.key === "Home" ? 0 : event.key === "End" ? tabs.length - 1 : undefined;
      if (next !== undefined) { event.preventDefault(); chapter(tabs[next].dataset.chapter!, true); }
    });
  });
  function closePreview(): void { preview.hidden = true; previewButton.setAttribute("aria-expanded", "false"); }
  conflictButton.addEventListener("click", () => {
    conflict.hidden = !conflict.hidden;
    conflictButton.setAttribute("aria-expanded", String(!conflict.hidden));
    if (!conflict.hidden) chapter("resolve"); else closePreview();
  });
  previewButton.addEventListener("click", () => { preview.hidden = !preview.hidden; previewButton.setAttribute("aria-expanded", String(!preview.hidden)); if (!preview.hidden) status.textContent = t("explore.previewOpened"); });
  document.querySelector("[data-close-preview]")!.addEventListener("click", () => { closePreview(); previewButton.focus(); updateStatus(); });
  document.querySelector("[data-reset-explore]")!.addEventListener("click", () => {
    client.value = "all"; evidence.value = "all"; selected = "project-instructions";
    conflict.hidden = true; conflictButton.setAttribute("aria-expanded", "false"); closePreview(); chapter("discover"); renderGraph();
  });
  client.addEventListener("change", renderGraph); evidence.addEventListener("change", renderGraph);
  function renderComparison(): void {
    const comparison = document.querySelector<HTMLElement>("[data-explore-comparison]")!;
    comparison.replaceChildren();
    const translatedTitle = (value: string, key: Key) => value === exploreCopy.en[key] ? t(key) : value;
    document.querySelector<HTMLElement>('[data-i18n="explore.conflictTitle"]')!.textContent = translatedTitle(demo.conflict.title, "explore.conflictTitle");
    document.querySelector<HTMLElement>('[data-i18n="explore.fixTitle"]')!.textContent = translatedTitle(demo.fix_preview.title, "explore.fixTitle");
    demo.conflict.node_ids.forEach(id => {
      const node = byId.get(id)!;
      const entry = element("div");
      entry.append(element("h3", label(node)), element("p", tier(node.evidence), "technical"), element("code", node.origin));
      comparison.append(entry);
    });
    document.querySelector<HTMLElement>("[data-explore-patch]")!.textContent = demo.fix_preview.patch.join("\n");
  }
  document.addEventListener("harnessscope:locale", () => { renderGraph(); renderComparison(); if (!preview.hidden) status.textContent = t("explore.previewOpened"); });
  renderGraph(); renderComparison();
}
