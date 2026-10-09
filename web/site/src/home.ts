export const homeCopy = {
  en: {
    "home.eyebrow": "Configuration forensics · local by design", "home.verified": "VERIFIED", "home.preview": "PREVIEW",
    "home.explore": "Explore the evidence ↗", "home.github": "View on GitHub ↗", "home.synthetic": "The public walkthrough is synthetic. The real inspection stays local.",
    "home.instrument": "Source → inheritance → evidence", "home.captureAlt": "HarnessScope synthetic dashboard overview", "home.orbitLabel": "Explore the evidence chapters",
    "home.captureLabel": "Synthetic dashboard capture", "home.captureSource": "Playwright synthetic fixture · HarnessScope Overview", "home.concept": "Concept photograph", "home.atmosphere": " · used as atmosphere",
    "home.storyKicker": "An evidence trail, from source to decision", "home.discover": "Discover", "home.trace": "Trace", "home.resolve": "Resolve",
    "home.discoverNote": "Map the local landscape", "home.discoverBody": "See which configuration and instruction files exist across your coding agents. Start with the sources you can inspect.",
    "home.traceNote": "Ask where a rule came from", "home.traceBody": "Follow provenance through the graph and inspector. Connect rules, MCP servers, hooks and skills to their declared sources.",
    "home.resolveNote": "Inspect before you apply", "home.resolveBody": "Review structural findings and safe-fix previews. Applying a safe plan requires explicit confirmation; backups and rollback stay local.",
    "home.galleryKicker": "Provenance over guesswork", "home.galleryTitle": "Follow the source.", "home.galleryBody": "Configuration can span clients, files and inherited instructions. HarnessScope makes the declared relationships visible, so you can inspect the evidence behind a finding.",
    "home.galleryBoundary": "Structural checks are evidence-bounded. They do not guarantee client runtime behavior or interpret instruction semantics.", "home.galleryLink": "Walk through fictional evidence ↗",
    "home.clientsKicker": "Compatibility is a claim with a boundary", "home.clientsTitle": "Know the evidence tier.", "home.matrixCaption": "Adapter support and verification boundaries", "home.client": "Client", "home.tier": "Tier", "home.boundary": "Evidence boundary",
    "home.exactVersion": "Exact ruleset verified; other versions are compatibility unknown.", "home.cursorBoundary": "Documented local rule and MCP sources. Effective precedence is not confirmed.", "home.opencodeBoundary": "Documented local JSON/JSONC and instruction sources. Remote configuration is not fetched; effective merge precedence is not asserted.",
    "home.localKicker": "Local is a design choice", "home.localTitle": "Your machine. Your evidence.", "home.localBody": "Scans run locally. Reports contain no external assets or network requests. HarnessScope does not execute MCP servers or contact remote configuration endpoints.", "home.localBoundary": "Credential patterns are redacted after parsing. No secret detector is perfect: review every report before publishing it.", "home.localLink": "Read the operating boundaries ↗",
    "home.sourceKicker": "Open the observatory on your machine", "home.sourceTitle": "Build from source.", "home.sourceBody": "Start with the repository and the documented local workflow.", "home.sourceLink": "Read the build instructions ↗",
  },
  "zh-CN": {
    "home.eyebrow": "配置溯源 · 为本地而设计", "home.verified": "已验证", "home.preview": "预览",
    "home.explore": "探索配置证据 ↗", "home.github": "在 GitHub 查看 ↗", "home.synthetic": "公开演示使用合成数据。实际检查在本地完成。",
    "home.instrument": "来源 → 继承 → 证据", "home.captureAlt": "HarnessScope 合成数据仪表盘总览", "home.orbitLabel": "探索配置证据章节",
    "home.captureLabel": "合成数据仪表盘截图", "home.captureSource": "Playwright 合成测试数据 · HarnessScope 总览", "home.concept": "概念照片", "home.atmosphere": " · 仅用于氛围展示",
    "home.storyKicker": "从来源到决策，沿证据追溯", "home.discover": "发现", "home.trace": "追溯", "home.resolve": "解决",
    "home.discoverNote": "梳理本地配置", "home.discoverBody": "查看各编程助手有哪些配置与指令文件。从可以检查的来源开始。",
    "home.traceNote": "查明规则从何而来", "home.traceBody": "通过关系图与检查器追溯来源，将规则、MCP 服务器、钩子与技能连接到它们的声明来源。",
    "home.resolveNote": "应用前先检查", "home.resolveBody": "审阅结构性发现与安全修复预览。应用安全计划需要明确确认，备份与回滚均在本地完成。",
    "home.galleryKicker": "依靠来源证据", "home.galleryTitle": "沿着来源追溯。", "home.galleryBody": "配置可能跨越客户端、文件与继承指令。HarnessScope 展示已声明的关系，让你检查发现背后的证据。",
    "home.galleryBoundary": "结构性检查受证据边界限制，不保证客户端运行行为，也不解释指令语义。", "home.galleryLink": "体验虚构配置证据 ↗",
    "home.clientsKicker": "兼容性声明有明确边界", "home.clientsTitle": "了解证据等级。", "home.matrixCaption": "适配器支持与验证边界", "home.client": "客户端", "home.tier": "等级", "home.boundary": "证据边界",
    "home.exactVersion": "已验证该确切版本的规则集；其他版本的兼容性未知。", "home.cursorBoundary": "已记录的本地规则与 MCP 来源。尚未确认实际优先级。", "home.opencodeBoundary": "已记录的本地 JSON/JSONC 与指令来源。不获取远程配置，不声明实际合并优先级。",
    "home.localKicker": "本地运行是设计选择", "home.localTitle": "你的计算机，你的证据。", "home.localBody": "扫描在本地运行，报告不包含外部资源或网络请求。HarnessScope 不执行 MCP 服务器，也不访问远程配置端点。", "home.localBoundary": "解析后即脱敏凭据模式。任何检测都无法保证识别所有秘密，公开报告前请逐项检查。", "home.localLink": "阅读运行边界 ↗",
    "home.sourceKicker": "在自己的计算机上打开观测站", "home.sourceTitle": "从源码构建。", "home.sourceBody": "从代码仓库与文档中的本地工作流开始。", "home.sourceLink": "阅读构建说明 ↗",
  },
} satisfies Record<"en" | "zh-CN", Record<string, string>>;

export function initHome(): void {
  if (document.body.dataset.page !== "home") return;
  const orbit = document.querySelector<HTMLElement>("[data-home-orbit]");
  const buttons = Array.from(document.querySelectorAll<HTMLButtonElement>("[data-orbit-target]"));
  const sections = Array.from(document.querySelectorAll<HTMLElement>("[data-story-section]"));
  let selectedId = sections.find(section => section.classList.contains("is-current"))?.id;
  const select = (id: string) => {
    selectedId = id;
    buttons.forEach(button => {
      if (button.dataset.orbitTarget === id) button.setAttribute("aria-current", "step");
      else button.removeAttribute("aria-current");
    });
    sections.forEach(section => section.classList.toggle("is-current", section.id === id));
  };
  buttons.forEach(button => button.addEventListener("click", () => {
    const section = sections.find(item => item.id === button.dataset.orbitTarget);
    if (!section) return;
    select(section.id);
    history.replaceState(null, "", `#${section.id}`);
    section.focus({ preventScroll: true });
    section.scrollIntoView({ block: "center", behavior: "auto" });
  }));
  if (orbit) orbit.hidden = false;
  if (!("IntersectionObserver" in window)) return;
  const visible = new Map<string, number>();
  const observer = new IntersectionObserver(entries => {
    entries.forEach(entry => {
      if (entry.isIntersecting) visible.set(entry.target.id, entry.intersectionRatio);
      else visible.delete(entry.target.id);
    });
    const best = [...visible.entries()].sort((a, b) => b[1] - a[1])[0];
    // Retain an explicit choice when multiple chapters are equally visible.
    if (best && (!selectedId || (visible.get(selectedId) ?? -1) < best[1])) select(best[0]);
  }, { threshold: [0, .25, .5, .75, 1] });
  sections.forEach(section => observer.observe(section));
}
