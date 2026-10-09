export type Locale = "en" | "zh-CN";
export type CopyKey = "nav.home" | "nav.explore" | "nav.docs" | "language.switch" | "nav.primary" | "skip" | "footer" | "home.title" | "home.body" | "explore.title" | "explore.body" | "docs.title" | "docs.body" | "explore.nojs" | "field.label" | "field.body";

export const copy: Record<Locale, Record<CopyKey, string>> = {
  en: {
    "nav.home": "Home", "nav.explore": "Explore", "nav.docs": "Docs & Download", "language.switch": "切换到中文",
    "nav.primary": "Primary", skip: "Skip to content", footer: "Local configuration. Traceable evidence.",
    "home.title": "Every rule leaves a trail.", "home.body": "Discover configuration. Trace its origins. Resolve conflicts on your local machine.",
    "explore.title": "Synthetic interactive walkthrough", "explore.body": "Explore fictional configuration evidence. The public walkthrough performs no scan or upload.",
    "docs.title": "Docs & Download", "docs.body": "Build from source. Read the documentation to understand local operation and compatibility boundaries.",
    "explore.nojs": "JavaScript is required for the interactive walkthrough. Navigation and documentation remain available.",
    "field.label": "Configuration observatory", "field.body": "Discover / Trace / Resolve",
  },
  "zh-CN": {
    "nav.home": "首页", "nav.explore": "交互体验", "nav.docs": "文档与下载", "language.switch": "Switch to English",
    "nav.primary": "主导航", skip: "跳到正文", footer: "本地配置，证据可追溯。",
    "home.title": "每条规则，都有迹可循。", "home.body": "发现配置，追溯来源，在本地计算机上解决冲突。",
    "explore.title": "合成交互演示", "explore.body": "探索虚构的配置证据。公开演示不执行扫描或上传。",
    "docs.title": "文档与下载", "docs.body": "从源码构建。阅读文档，了解本地运行方式及兼容性边界。",
    "explore.nojs": "交互演示需要 JavaScript。导航和文档仍可访问。",
    "field.label": "配置观测站", "field.body": "发现 / 追溯 / 解决",
  },
};
