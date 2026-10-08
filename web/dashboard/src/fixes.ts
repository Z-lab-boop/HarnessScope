import { APIClient, APIError } from "./api.js";
import { Store } from "./store.js";
import type { BackupSummary, DashboardState, FixPlan, UIState } from "./types.js";
import { badge, button, element } from "./views.js";

export class FixCenter {
  private selected = new Set<string>();
  private dashboard: DashboardState | null = null;
  private panel: HTMLElement | null = null;
  private dialog: HTMLDialogElement | null = null;
  private feedback = "";
  constructor(private store: Store, private api: APIClient, private modalRoot: HTMLElement, private announce: (message: string) => void) {}

  render(state: Readonly<UIState>): HTMLElement {
    if (this.dashboard !== state.dashboard) { this.selected.clear(); this.dashboard = state.dashboard; }
    const panel = element("section", "", "fix-center"); panel.setAttribute("aria-label", "Fix Center workspace");
    panel.append(element("h2", "Dry fix plans"), element("p", "Preview operations from this snapshot. Only explicitly selected SAFE fixes can be applied in the browser.", "muted"));
    panel.append(element("p", this.feedback, "fix-feedback"));
    const tools = element("div", "", "fix-tools");
    const refresh = button("Refresh dry plan", () => void this.refresh());
    const apply = button("Apply selected SAFE fixes", () => this.confirmApply()); apply.dataset.fixApply = "";
    tools.append(refresh, apply); panel.append(tools);
    const plans = state.dashboard?.fix_plans ?? [];
    if (!plans.length) panel.append(element("p", "No fix plans in this snapshot.", "empty"));
    for (const plan of plans) panel.append(this.card(plan));
    const history = element("section", "", "backup-history"); history.setAttribute("aria-label", "Backup history");
    history.append(element("h2", "Backup history"), element("p", "Backups retain original bytes and file modes locally. Rollback overwrites the current files with the selected backup.", "muted"));
    const backups = state.dashboard?.backups ?? [];
    if (!backups.length) history.append(element("p", "No backups yet.", "empty"));
    for (const backup of backups) {
      const item = element("article", "", "backup-card");
      item.append(element("h3", backup.id), element("p", `${backup.file_count} file${backup.file_count === 1 ? "" : "s"} · ${backup.created_at}`, "muted"), button(`Rollback ${backup.id}`, () => this.confirmRollback(backup)));
      history.append(item);
    }
    panel.append(history); this.panel = panel; this.sync(state); return panel;
  }

  sync(state: Readonly<UIState>): void {
    const feedback = this.panel?.querySelector<HTMLElement>(".fix-feedback");
    if (feedback) { feedback.textContent = this.feedback; feedback.hidden = !this.feedback; }
    this.panel?.querySelectorAll<HTMLButtonElement | HTMLInputElement>("button, input").forEach((control) => {
      control.disabled = state.busy || control.dataset.unavailable === "true" || (control.hasAttribute("data-fix-apply") && !this.selected.size);
    });
  }

  private card(plan: FixPlan): HTMLElement {
    const card = element("article", "", "fix-card"); card.setAttribute("aria-label", plan.id);
    const header = element("div", "", "finding-meta"); header.append(element("h3", plan.id), badge(plan.risk)); card.append(header);
    const label = element("label", "", "fix-selection"), checkbox = element("input"); checkbox.type = "checkbox";
    checkbox.setAttribute("aria-label", `Select ${plan.id}`); checkbox.dataset.unavailable = String(plan.risk !== "SAFE"); checkbox.checked = this.selected.has(plan.id);
    checkbox.addEventListener("change", () => {
      if (plan.risk !== "SAFE" || this.store.get().busy) return;
      if (checkbox.checked) this.selected.add(plan.id); else this.selected.delete(plan.id);
      this.sync(this.store.get());
    });
    label.append(checkbox, element("span", plan.risk === "SAFE" ? "Select this SAFE fix" : `Browser apply unavailable: ${plan.risk === "REVIEW" ? "requires human review before editing." : "requires manual changes outside the browser."}`)); card.append(label);
    for (const edit of plan.edits ?? []) {
      card.append(element("p", `${edit.target_path} · ${edit.operation}`, "muted"), element("pre", edit.redacted_patch || "No redacted patch available.", "fix-patch"));
    }
    for (const condition of plan.preconditions ?? []) card.append(element("p", `Before: ${condition}`, "muted"));
    for (const condition of plan.postconditions ?? []) card.append(element("p", `After: ${condition}`, "muted"));
    return card;
  }

  private confirmApply(): void {
    const state = this.store.get(); if (state.busy || !state.dashboard || this.dialog) return;
    const plans = (state.dashboard.fix_plans ?? []).filter((plan) => plan.risk === "SAFE" && this.selected.has(plan.id));
    if (!plans.length) return;
    const revision = state.dashboard.revision, ids = plans.map((plan) => plan.id);
    const count = new Set(plans.flatMap((plan) => (plan.edits ?? []).map((edit) => edit.target_path))).size;
    this.confirm("Apply SAFE fixes?", `${ids.length} SAFE fix${ids.length === 1 ? "" : "es"} will change ${count} file${count === 1 ? "" : "s"} at revision ${revision}. Each transaction creates a local backup of original bytes and file modes before writing. Failed batches roll back completed transactions in reverse order.`, "Confirm apply", () => this.api.applyFixes(revision, ids), "Applied selected SAFE fixes");
  }

  private confirmRollback(backup: BackupSummary): void {
    const state = this.store.get(); if (state.busy || !state.dashboard || this.dialog) return;
    const revision = state.dashboard.revision;
    this.confirm(`Rollback ${backup.id}?`, `Restore ${backup.file_count} file${backup.file_count === 1 ? "" : "s"} from backup ${backup.id}. This will overwrite current file contents and modes, including edits made after the backup.`, "Confirm rollback", () => this.api.rollback(revision, backup.id), `Rolled back ${backup.id}`, backup.id);
  }

  private confirm(title: string, description: string, submitText: string, request: () => Promise<DashboardState>, success: string, backupID?: string): void {
    const opener = document.activeElement as HTMLElement | null;
    const dialog = element("dialog", "", "fix-dialog"); this.dialog = dialog;
    dialog.setAttribute("aria-labelledby", "fix-dialog-title"); dialog.setAttribute("aria-describedby", "fix-dialog-description");
    const heading = element("h2", title); heading.id = "fix-dialog-title";
    const body = element("p", description); body.id = "fix-dialog-description";
    dialog.append(heading, body);
    const close = () => {
      dialog.close(); dialog.remove(); this.dialog = null;
      if (opener?.isConnected) opener.focus(); else document.querySelector<HTMLElement>("main h1")?.focus();
    };
    const cancel = button("Cancel", close);
    const submit = button(submitText, () => {
      if (this.store.get().busy || (backupID && input?.value !== backupID)) return;
      cancel.disabled = true; submit.disabled = true; if (input) input.disabled = true;
      dialog.setAttribute("aria-busy", "true");
      void this.mutate(request, success).finally(close);
    });
    let input: HTMLInputElement | undefined;
    if (backupID) {
      const label = element("label", "Type backup ID to confirm", "filter-control"); input = element("input"); input.type = "text"; input.autocomplete = "off"; input.spellcheck = false;
      input.addEventListener("input", () => { submit.disabled = this.store.get().busy || input?.value !== backupID; });
      label.append(input); dialog.append(label); submit.disabled = true;
    }
    const controls = element("div", "", "fix-tools"); controls.append(cancel, submit); dialog.append(controls);
    dialog.addEventListener("cancel", (event) => { event.preventDefault(); if (!this.store.get().busy) close(); });
    dialog.addEventListener("keydown", (event) => {
      if (event.key !== "Tab") return;
      const controls = Array.from(dialog.querySelectorAll<HTMLElement>("button:enabled, input:enabled"));
      if (!controls.length) { event.preventDefault(); return; }
      const first = controls[0], last = controls[controls.length - 1];
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
    });
    this.modalRoot.append(dialog); dialog.showModal(); cancel.focus();
  }

  private async refresh(): Promise<void> {
    const state = this.store.get(); if (state.busy || !state.dashboard) return;
    const current = state.dashboard;
    // A dry preview alone does not publish its IDs. Rescan keeps selectable
    // plans and the server's authoritative revision in the same snapshot.
    await this.mutate(() => this.api.rescan(current.revision), "Dry plan refreshed");
  }

  private async mutate(request: () => Promise<DashboardState>, success: string): Promise<void> {
    if (this.store.get().busy) return;
    this.feedback = "";
    this.store.update({ busy: true, error: null });
    this.selected.clear();
    try {
      const dashboard = await request();
      this.feedback = `${success}. Scan revision ${dashboard.revision} loaded.`;
      this.store.update({ dashboard, selection: null });
      this.announce(this.feedback);
    } catch (error) {
      if (error instanceof APIError && error.code === "stale_revision") {
        try {
          this.store.update({ dashboard: await this.api.state(), selection: null, error: "The workspace changed. Review the refreshed snapshot; use Rescan if files changed, then select fixes again. The rejected action was not repeated." });
        } catch (refreshError) {
          this.store.update({ error: `Snapshot refresh failed. Use Rescan to recover. ${refreshError instanceof Error ? refreshError.message : "Local server unavailable."}` });
        }
      } else {
        this.store.update({ error: error instanceof APIError && error.status === 409 ? "The target changed. Use Rescan and review a fresh plan before retrying." : error instanceof Error ? error.message : "Unable to complete the local operation." });
      }
    } finally {
      // Even when the snapshot is unchanged, rejected selections must be reset.
      this.panel?.querySelectorAll<HTMLInputElement>('input[type="checkbox"]').forEach((input) => { input.checked = false; });
      this.store.update({ busy: false });
    }
  }
}
