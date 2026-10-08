import { APIClient, APIError, consumeFragmentToken } from "./api.js";
import { Store } from "./store.js";
import type { DashboardState, Route, UIState } from "./types.js";
import { renderGraph } from "./graph.js";
import { renderFindings } from "./findings.js";
import { renderCompare } from "./compare.js";
import { renderInspector } from "./inspector.js";
import { emptyFilters, initialViews, type Actions } from "./views.js";

const routes: Record<Route, string> = { overview: "Overview", graph: "Graph", findings: "Findings", compare: "Compare", fixes: "Fix Center", drift: "Drift", export: "Export" };
function readRoute(): Route {
  const value = new URLSearchParams(location.search).get("view") ?? "overview";
  return Object.prototype.hasOwnProperty.call(routes, value) ? value as Route : "overview";
}
function element<K extends keyof HTMLElementTagNameMap>(tag: K, text = "", className = ""): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  node.textContent = text;
  if (className) node.className = className;
  return node;
}
function badge(text: string, kind = ""): HTMLElement { return element("span", text, `badge ${kind}`); }
function section(title: string, className = ""): HTMLElement {
  const node = element("section", "", className);
  node.setAttribute("aria-label", title);
  node.append(element("h2", title));
  return node;
}

const api = new APIClient(consumeFragmentToken());
const store = new Store({ dashboard: null, route: readRoute(), selection: null, busy: true, error: null, views: initialViews() });
const root = document.querySelector<HTMLDivElement>("#dashboard")!;
const skip = element("a", "Skip to workbench", "skip-link");
skip.href = "#workbench";
const rail = element("aside", "", "rail");
const brand = element("div", "", "brand");
brand.append(element("span", "H / S", "brand-mark"), element("strong", "HarnessScope"), element("span", "LOCAL CONTROL CENTER", "eyebrow"));
const nav = element("nav");
nav.setAttribute("aria-label", "Primary");
const links = new Map<Route, HTMLAnchorElement>();
Object.entries(routes).forEach(([key, label], index) => {
  const route = key as Route;
  const link = element("a", "", "nav-link");
  const number = element("span", String(index + 1).padStart(2, "0"), "nav-number");
  number.setAttribute("aria-hidden", "true");
  link.append(number, element("span", label));
  link.href = `?view=${route}`;
  link.addEventListener("click", (event) => {
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    const url = new URL(location.href);
    url.searchParams.set("view", route);
    history.pushState(null, "", url.pathname + url.search);
    store.update({ route, selection: null });
    main.querySelector<HTMLElement>("h1")?.focus();
  });
  links.set(route, link);
  nav.append(link);
});
const railFoot = element("div", "", "rail-foot");
railFoot.append(element("span", "◈  LOOPBACK SESSION", "eyebrow"), element("p", "Your workspace.\nYour machine."), element("small", "No external assets · no telemetry"));
rail.append(brand, nav, railFoot);
const top = element("header", "", "topbar");
const workspace = element("div", "", "workspace");
workspace.append(element("span", "SCAN ROOT", "eyebrow"));
const workspacePath = element("code", "Waiting for local state…");
workspace.append(workspacePath);
const telemetry = element("div", "", "telemetry");
const revision = badge("REV —");
const connection = element("span", "Connecting", "connection");
const rescan = element("button", "↻  Rescan", "rescan");
rescan.type = "button";
rescan.setAttribute("aria-label", "Rescan workspace");
rescan.addEventListener("click", () => void load(true));
telemetry.append(revision, connection, rescan);
top.append(workspace, telemetry);
const main = element("main", "", "workbench");
main.id = "workbench";
main.tabIndex = -1;
skip.addEventListener("click", (event) => { event.preventDefault(); main.focus(); });
const inspector = element("aside", "", "inspector");
inspector.setAttribute("aria-label", "Inspector");
const toast = element("div", "", "toast-region");
toast.setAttribute("role", "status");
toast.setAttribute("aria-live", "polite");
const alert = element("div", "", "error-region");
alert.setAttribute("role", "alert");
const modalRoot = element("div");
modalRoot.id = "modal-root";
root.append(skip, rail, top, main, inspector, toast, alert, modalRoot);

function heading(title: string, subtitle: string): void {
  const header = element("div", "", "view-heading");
  const titleNode = element("h1", title);
  titleNode.tabIndex = -1;
  header.append(element("p", "WORKSPACE / " + routes[store.get().route].toUpperCase(), "eyebrow"), titleNode, element("p", subtitle, "muted"));
  main.append(header);
}
function metric(title: string, value: string, note: string, accent = ""): HTMLElement {
  const card = section(title, `metric ${accent}`);
  card.append(element("strong", value, "metric-value"), element("p", note, "muted"));
  return card;
}
function overview(data: DashboardState): void {
  const analysis = data.result.analysis;
  const clients = analysis.clients ?? [];
  const findings = analysis.findings ?? [];
  const estimates = analysis.context ?? [];
  const verified = clients.filter((client) => client.compatibility.state === "VERIFIED").length;
  const high = findings.filter((finding) => finding.severity === "HIGH").length;
  const min = estimates.reduce((sum, item) => sum + item.min_tokens, 0);
  const max = estimates.reduce((sum, item) => sum + item.max_tokens, 0);
  heading("Configuration observatory", "Trace what loads. Inspect what conflicts. Keep evidence in view.");
  const metrics = element("div", "", "metrics");
  const risk = metric("Findings", String(findings.length), "Observed configuration findings", high ? "risk" : "");
  risk.append(badge(`HIGH · ${high}`, high ? "high" : ""));
  metrics.append(metric("Client coverage", `${verified} / ${clients.length}`, "Compatibility verified / scanned"), risk,
    metric("Estimated context", estimates.length ? `${min}–${max}` : "Not estimated", "Token range · not measured usage"),
    metric("Portability", String(clients.length - verified), "Clients with unverified compatibility", "violet"));
  main.append(metrics);

  const panel = section("Client signal rail", "signal-panel");
  const panelHeader = element("div", "", "panel-meta");
  panelHeader.append(element("p", "Snapshot only · no live monitoring", "muted"), element("code", `${clients.length} CLIENTS / ${findings.length} FINDINGS`));
  panel.append(panelHeader);
  if (!clients.length) panel.append(element("p", "No clients in this scan.", "empty"));
  for (const client of clients) {
    const row = element("button", "", "client-signal");
    row.type = "button";
    row.setAttribute("aria-label", `Inspect ${client.id}`);
    row.dataset.client = client.id;
    const identity = element("span", "", "client-identity");
    const dot = element("span", "", "signal-dot");
    dot.setAttribute("aria-hidden", "true");
    identity.append(dot, element("strong", client.id), element("small", client.detection.installed ? `Detected · ${client.detection.version ?? "version unknown"}` : "Not detected"));
    const trace = element("span", "", "signal-trace");
    trace.setAttribute("aria-hidden", "true");
    const count = findings.filter((finding) => finding.affected_clients?.includes(client.id)).length;
    const end = element("span", "", "client-readings");
    end.append(badge(client.compatibility.tier, client.compatibility.tier === "PREVIEW" ? "preview" : "verified"), element("span", `${count} finding${count === 1 ? "" : "s"}`, count ? "risk-text" : "muted"));
    row.append(identity, trace, end);
    row.addEventListener("click", () => store.update({ selection: client.id }));
    panel.append(row);
  }
  main.append(panel);
  const summary = section("Evidence boundary", "boundary");
  summary.append(element("p", "Adapter tier describes support maturity. Compatibility, detection and findings are separate signals; a VERIFIED adapter does not certify a safe configuration."));
  const scanTime = element("time", data.scanned_at);
  scanTime.dateTime = data.scanned_at;
  const meta = element("p", "Snapshot captured ", "muted");
  meta.append(scanTime);
  summary.append(meta);
  main.append(summary);
}

const actions: Actions = {
  update: (change) => store.update(change),
  focusNode: (id) => {
    const url = new URL(location.href); url.searchParams.set("view", "graph"); history.pushState(null, "", url.pathname + url.search);
    store.update({ route: "graph", selection: id, views: { ...(store.get().views ?? initialViews()), graph: emptyFilters() } });
    Array.from(main.querySelectorAll<SVGGElement>("[data-node-id]")).find((node) => node.dataset.nodeId === id)?.focus();
  },
};
let renderedDashboard: DashboardState | null | undefined;
let renderedRoute: Route | undefined;
let renderedViews: UIState["views"];
function render(state: Readonly<UIState>): void {
  for (const [route, link] of links) {
    if (state.route === route) link.setAttribute("aria-current", "page"); else link.removeAttribute("aria-current");
  }
  workspacePath.textContent = state.dashboard?.workspace ?? "Waiting for local state…";
  revision.textContent = `REV ${state.dashboard?.revision ?? "—"}`;
  connection.textContent = state.busy ? "Reading snapshot…" : state.error ? "Request failed" : "Local snapshot loaded";
  rescan.disabled = state.busy || !state.dashboard;
  main.setAttribute("aria-busy", String(state.busy));
  alert.textContent = state.error ?? "";
  alert.hidden = !state.error;
  if (state.dashboard !== renderedDashboard || state.route !== renderedRoute || state.views !== renderedViews) {
    const active = document.activeElement as HTMLInputElement | HTMLSelectElement | null;
    const control = active?.dataset.control;
    const cursor = active instanceof HTMLInputElement ? active.selectionStart : null;
    main.replaceChildren();
    if (!state.dashboard) heading("Connect to your workspace", "Reading the authenticated local scan. If the server is unavailable, reopen its launch URL.");
    else if (state.route === "overview") overview(state.dashboard);
    else if (["graph", "findings", "compare"].includes(state.route)) {
      heading(routes[state.route], "Snapshot evidence · trace configuration and inspect its origins");
      main.append(state.route === "graph" ? renderGraph(state, actions) : state.route === "findings" ? renderFindings(state, actions) : renderCompare(state, actions));
    } else {
      heading(routes[state.route], "Workspace evidence channel");
      const panel = section(`${routes[state.route]} workspace`, "boundary");
      panel.append(element("p", "This workspace is reserved for the next control-center module. The Overview contains the current scan summary."));
      main.append(panel);
    }
    renderedDashboard = state.dashboard;
    renderedRoute = state.route;
    renderedViews = state.views;
    if (control) {
      const replacement = Array.from(main.querySelectorAll<HTMLInputElement | HTMLSelectElement>("[data-control]")).find((node) => node.dataset.control === control);
      replacement?.focus();
      if (replacement instanceof HTMLInputElement && cursor !== null) replacement.setSelectionRange(cursor, cursor);
    }
  }
  inspector.replaceChildren(renderInspector(state));
  const graphNodes = Array.from(main.querySelectorAll<SVGGElement>("[data-node-id]"));
  const selectedVisible = graphNodes.some((node) => node.dataset.nodeId === state.selection);
  graphNodes.forEach((node, index) => {
    const selected = node.dataset.nodeId === state.selection;
    node.setAttribute("aria-pressed", String(selected));
    node.setAttribute("tabindex", selected || (!selectedVisible && index === 0) ? "0" : "-1");
  });
  main.querySelectorAll<HTMLButtonElement>("[data-client]").forEach((button) => button.setAttribute("aria-pressed", String(button.dataset.client === state.selection)));
}
async function load(rescanning = false): Promise<void> {
  if (rescanning && store.get().busy) return;
  store.update({ busy: true, error: null });
  try {
    const previous = store.get().dashboard;
    const dashboard = rescanning && previous ? await api.rescan(previous.revision) : await api.state();
    store.update({ dashboard, busy: false, selection: null });
    toast.textContent = `Scan revision ${dashboard.revision} loaded.`;
  } catch (error) {
    if (error instanceof APIError && error.code === "stale_revision") {
      try {
        // Reuse the authenticated client, but never replay a rejected mutation.
        const dashboard = await api.state();
        store.update({ dashboard, busy: false, selection: null, error: "The workspace changed. Review the refreshed snapshot and retry the action." });
        toast.textContent = `Scan revision ${dashboard.revision} loaded. The rejected action was not repeated.`;
        return;
      } catch (refreshError) {
        store.update({ busy: false, error: `The snapshot changed, but refresh failed. ${refreshError instanceof Error ? refreshError.message : "Retry when the local server is available."}` });
        return;
      }
    }
    store.update({ busy: false, error: error instanceof Error ? error.message : "Unable to load the local scan." });
  }
}
window.addEventListener("popstate", () => { store.update({ route: readRoute(), selection: null }); main.querySelector<HTMLElement>("h1")?.focus(); });
store.subscribe(render);
render(store.get());
void load();
