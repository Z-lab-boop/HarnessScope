import { APIClient } from "./api.js";
import { diagnosticAction } from "./export.js";
import { Store } from "./store.js";
import type { SnapshotMetadata, UIState } from "./types.js";
import { button, element, emptyFilters, initialViews, select, type Actions } from "./views.js";

export class DriftView {
  private panel: HTMLElement | null = null;
  private baselines: SnapshotMetadata[] = [];
  private baseline = "";
  private name = "";
  private loading = false;
  private generation = 0;
  constructor(private store: Store, private api: APIClient, private actions: Actions, private announce: (message: string) => void) {}

  render(state: Readonly<UIState>): HTMLElement {
    this.panel = element("section", "", "diagnostic-panel"); this.panel.setAttribute("aria-label", "Drift workspace");
    this.loading = true; this.paint(state);
    const generation = ++this.generation;
    void this.api.snapshots().then((items) => {
      if (generation !== this.generation) return;
      this.baselines = [...(items ?? [])].sort((a, b) => a.name.localeCompare(b.name));
      if (!this.baselines.some((item) => item.name === this.baseline)) this.baseline = this.baselines[0]?.name ?? "";
    }).catch((error: unknown) => {
      if (generation === this.generation) this.store.update({ error: error instanceof Error ? error.message : "Could not list baselines." });
    }).finally(() => {
      if (generation !== this.generation) return;
      this.loading = false; this.paint(this.store.get());
    });
    return this.panel;
  }

  private paint(state: Readonly<UIState>): void {
    const panel = this.panel; if (!panel) return;
    panel.replaceChildren(element("h2", "Saved baselines"), element("p", "Save this sanitized revision locally, then compare normalized entity metadata. Raw before/after values are never displayed. Saving an existing name replaces that baseline.", "muted"));
    const tools = element("div", "", "diagnostic-tools");
    const label = element("label", "Snapshot name", "filter-control"), input = element("input");
    input.value = this.name; input.maxLength = 64; input.autocomplete = "off"; input.spellcheck = false;
    input.addEventListener("input", () => { this.name = input.value; this.sync(this.store.get()); }); label.append(input);
    const save = button("Save snapshot", () => void diagnosticAction(this.store, this.api, async (revision) => {
      const item = await this.api.saveSnapshot(revision, this.name);
      this.baselines = [...this.baselines.filter((old) => old.name !== item.name), item].sort((a, b) => a.name.localeCompare(b.name));
      this.baseline = item.name; this.paint(this.store.get()); this.announce(`Saved baseline ${item.name}.`);
    })); save.dataset.saveSnapshot = "";
    tools.append(label, save); panel.append(tools);
    panel.append(element("p", "Use 1–64 letters, digits, dots, underscores or hyphens; start with a letter or digit.", "muted"));
    if (this.loading) panel.append(element("p", "Loading baselines…", "muted"));
    else if (!this.baselines.length) panel.append(element("p", "No saved baselines yet.", "empty"));
    const compareTools = element("div", "", "diagnostic-tools");
    const picker = select("Baseline", this.baselines.map((item) => item.name), this.baseline, (name) => { this.baseline = name; this.sync(this.store.get()); }, false);
    picker.querySelector("select")!.setAttribute("aria-label", "Baseline");
    compareTools.append(picker);
    const compare = button("Compare baseline", () => void diagnosticAction(this.store, this.api, async (revision) => {
      const dashboard = await this.api.drift(revision, this.baseline);
      this.store.update({ dashboard, selection: null }); this.announce(`Compared baseline ${this.baseline}.`);
    })); compare.dataset.compareBaseline = ""; compareTools.append(compare); panel.append(compareTools);
    const drift = state.dashboard?.drift;
    if (drift) {
      panel.append(element("h2", `Comparison · ${drift.baseline}`));
      const changes = drift.changes ?? [];
      if (!changes.length) panel.append(element("p", "No normalized drift from this baseline.", "empty"));
      for (const kind of ["ADDED", "REMOVED", "CHANGED"] as const) {
        for (const entity of ["CLIENT", "COMPATIBILITY", "FINDING", "NODE", "SOURCE"] as const) {
          const members = changes.filter((change) => change.kind === kind && change.entity_type === entity).sort((a, b) => a.id.localeCompare(b.id));
          if (!members.length) continue;
          const group = element("section", "", "drift-group"); group.append(element("h3", `${kind[0]}${kind.slice(1).toLowerCase()} · ${entity}`));
          for (const change of members) {
            const row = element("div", "", "drift-row"); row.append(element("code", change.id));
            if (entity === "NODE" && state.dashboard?.result.analysis.graph.nodes?.some((node) => node.id === change.id)) {
              row.append(button(`Inspect node ${change.id}`, () => this.actions.focusNode(change.id)));
            } else if (entity === "FINDING") {
              const rule = change.id.replace(/:[0-9a-f]{16}$/, "");
              // The existing Findings view indexes rules, not drift fingerprints.
              // A surviving same-rule finding cannot stand in for a removed identity.
              if (change.kind !== "REMOVED" && state.dashboard?.result.analysis.findings?.some((finding) => finding.rule_id === rule)) {
                row.append(button(`View current findings for rule ${rule}`, () => {
                  const url = new URL(location.href); url.searchParams.set("view", "findings"); history.pushState(null, "", url.pathname + url.search);
                  this.store.update({ route: "findings", selection: null, views: { ...(this.store.get().views ?? initialViews()), findings: { ...emptyFilters(), text: rule } } });
                }));
              } else row.append(element("span", "Not in current snapshot", "muted"));
            } else if (entity === "NODE") row.append(element("span", "Not in current snapshot", "muted"));
            group.append(row);
          }
          panel.append(group);
        }
      }
    }
    this.sync(state);
  }

  sync(state: Readonly<UIState>): void {
    this.panel?.querySelectorAll<HTMLButtonElement | HTMLInputElement | HTMLSelectElement>("button, input, select").forEach((control) => {
      control.disabled = state.busy || this.loading || (control.hasAttribute("data-save-snapshot") && !/^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/.test(this.name)) || (control.hasAttribute("data-compare-baseline") && !this.baseline);
    });
  }
}
