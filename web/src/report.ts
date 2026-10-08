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
  if (panel) panel.hidden = !panel.hidden;
});
