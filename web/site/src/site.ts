import { copy, type CopyKey, type Locale } from "./i18n.js";
import { homeCopy, initHome } from "./home.js";

const storageKey = "harnessscope.locale";

function translate(key: string | undefined, locale: Locale, fallback: string): string {
  if (key && Object.prototype.hasOwnProperty.call(copy.en, key)) return copy[locale][key as CopyKey];
  if (key && Object.prototype.hasOwnProperty.call(homeCopy.en, key)) return homeCopy[locale][key as keyof typeof homeCopy.en];
  if (process.env.NODE_ENV !== "production") throw new Error(`Unknown translation key: ${key}`);
  return fallback;
}

export function setLocale(locale: Locale): void {
  document.documentElement.lang = locale;
  document.querySelectorAll<HTMLElement>("[data-i18n]").forEach(element => {
    element.textContent = translate(element.dataset.i18n, locale, element.textContent ?? "");
  });
  document.querySelectorAll<HTMLElement>("[data-i18n-aria]").forEach(element => {
    element.setAttribute("aria-label", translate(element.dataset.i18nAria, locale, element.getAttribute("aria-label") ?? ""));
  });
  document.querySelectorAll<HTMLImageElement>("[data-i18n-alt]").forEach(element => {
    element.alt = translate(element.dataset.i18nAlt, locale, element.alt);
  });
  const pageKey = document.body.dataset.page;
  if (pageKey) document.title = `${translate(`${pageKey}.title`, locale, document.title)} — HarnessScope`;
  const description = document.querySelector<HTMLMetaElement>('meta[name="description"]');
  if (description && pageKey) description.content = translate(`${pageKey}.body`, locale, description.content);
  document.querySelectorAll<HTMLButtonElement>("[data-language-switch]").forEach(button => {
    button.textContent = locale === "en" ? "中文" : "EN";
    button.setAttribute("aria-label", copy[locale]["language.switch"]);
  });
}

let initial: Locale = "en";
try {
  const stored = localStorage.getItem(storageKey);
  if (stored === "en" || stored === "zh-CN") initial = stored;
} catch { /* Storage is optional; navigation must remain usable. */ }
setLocale(initial);
initHome();
document.querySelectorAll<HTMLButtonElement>("[data-language-switch]").forEach(button => {
  button.addEventListener("click", () => {
    const locale: Locale = document.documentElement.lang === "en" ? "zh-CN" : "en";
    setLocale(locale);
    try { localStorage.setItem(storageKey, locale); } catch { /* Keep the explicit choice in memory. */ }
  });
});
