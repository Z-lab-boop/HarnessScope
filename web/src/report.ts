type FindingRow = HTMLTableRowElement & { dataset: DOMStringMap };

const filter = document.querySelector<HTMLSelectElement>("#severity-filter");
const rows = Array.from(document.querySelectorAll<FindingRow>("#findings tbody tr"));

filter?.addEventListener("change", () => {
  const selected = filter.value;
  for (const row of rows) {
    row.hidden = selected !== "ALL" && row.dataset.severity !== selected;
  }
});

document.querySelector<HTMLButtonElement>("#toggle-data")?.addEventListener("click", () => {
  const panel = document.querySelector<HTMLElement>("#raw-data");
  const source = document.querySelector<HTMLElement>("#report-data");
  if (!panel) return;
  if (!panel.textContent && source?.textContent) {
    const binary = atob(source.textContent.trim());
    const bytes = Uint8Array.from(binary, (character) => character.charCodeAt(0));
    panel.textContent = new TextDecoder().decode(bytes);
  }
  panel.hidden = !panel.hidden;
});
