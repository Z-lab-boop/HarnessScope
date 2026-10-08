import { APIClient, APIError } from "./api.js";
import { Store } from "./store.js";
import type { UIState } from "./types.js";
import { button, element } from "./views.js";

// All diagnostic mutations share the same revision gate. Refresh on conflict,
// but never replay a save, comparison or export after its revision was rejected.
export async function diagnosticAction(store: Store, api: APIClient, action: (revision: number) => Promise<void>): Promise<void> {
  const state = store.get();
  if (state.busy || !state.dashboard) return;
  store.update({ busy: true, error: null });
  try { await action(state.dashboard.revision); }
  catch (error) {
    if (error instanceof APIError && error.code === "stale_revision") {
      try {
        const dashboard = await api.state();
        store.update({ dashboard, selection: null, error: "The workspace changed. Review the refreshed snapshot and retry the action. Nothing was repeated." });
      } catch { store.update({ error: "The snapshot changed, but refresh failed. Rescan the workspace and retry." }); }
    } else store.update({ error: error instanceof Error ? error.message : "The diagnostic request failed." });
  } finally { store.update({ busy: false }); }
}

export async function downloadBundle(api: APIClient, revision: number, baseline?: string): Promise<void> {
  const blob = await api.exportBundle(revision, baseline);
  const url = URL.createObjectURL(blob);
  try {
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = "harnessscope-diagnostic.zip";
    anchor.click();
  } finally { URL.revokeObjectURL(url); }
}

export class ExportView {
  private panel: HTMLElement | null = null;
  constructor(private store: Store, private api: APIClient, private announce: (message: string) => void) {}
  render(state: Readonly<UIState>): HTMLElement {
    const panel = element("section", "", "diagnostic-panel"); panel.setAttribute("aria-label", "Export workspace");
    panel.append(element("h2", "Diagnostic bundle"), element("p", "Download report.json, report.html, README.txt and a manifest of member hashes. Published drift, when present, is included as drift.json.", "muted"));
    const warning = element("p", "Human review is required before public upload. Sanitization removes known secrets and local roots, but project names and other sensitive context may remain. Inspect every bundle member before sharing.", "privacy-warning");
    warning.id = "export-privacy-warning";
    panel.append(warning, element("p", state.dashboard?.drift ? `Includes normalized drift against ${state.dashboard.drift.baseline}.` : "No baseline comparison in this revision.", "muted"));
    const download = button("Download diagnostic ZIP", () => void diagnosticAction(this.store, this.api, async (revision) => {
      await downloadBundle(this.api, revision, this.store.get().dashboard?.drift?.baseline);
      this.announce("Diagnostic ZIP downloaded. Review all members before sharing.");
    }));
    download.setAttribute("aria-describedby", warning.id);
    panel.append(download); this.panel = panel; this.sync(state); return panel;
  }
  sync(state: Readonly<UIState>): void { this.panel?.querySelectorAll("button").forEach((button) => { button.disabled = state.busy; }); }
}
