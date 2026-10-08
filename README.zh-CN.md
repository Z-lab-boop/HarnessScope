# HarnessScope

HarnessScope 是一个本地、离线的编码助手配置体检工具，用来回答三个问题：实际发现了哪些配置；某条规则、MCP、Hook 或 Skill 从哪里来；哪些冲突、失效路径和重复上下文值得处理。

当前版本对 Codex `0.162.0-alpha.2`、Claude Code `2.1.259` 提供 `VERIFIED` 适配器，对 Cursor、OpenCode 提供保守的 `PREVIEW` 适配器。预览适配器只读取官方文档支持的本地来源，不把未经验证的覆盖关系包装成确定事实。

## 五分钟上手

从 GitHub Releases 下载与你的系统和架构匹配的压缩包，并先用同一发布页的 `SHA256SUMS` 校验：

```sh
shasum -a 256 -c SHA256SUMS
tar -xzf harnessscope_v0.1.0_darwin_arm64.tar.gz
./hscope scan . --open
```

Linux 可使用 `sha256sum -c SHA256SUMS`。在首个正式标签发布前，也可用 Go 1.24 或更高版本从源码构建：

```sh
go build -trimpath -o bin/hscope ./cmd/hscope
./bin/hscope scan . --open
```

默认情况下，JSON 与单文件离线 HTML 报告写入系统应用数据目录，不污染被扫描的仓库。只想看终端结果时可加 `--no-report`。

## 常用命令

```text
hscope scan [path] [--client all|codex|claude|cursor|opencode]
hscope explain <规范化名称> [--client ...] [--path ...]
hscope compare <客户端A> <客户端B> [--path ...]
hscope report --from report.json --html report.html
hscope fix [修复ID...]              # 默认仅预览
hscope fix [修复ID...] --apply      # 只应用 SAFE 修复
hscope rollback <备份ID>
```

仓库内置合成演示：

```sh
./demo/run.sh
```

演示只使用虚构配置，并自动检查凭据金丝雀不会进入终端、JSON 或 HTML。其摘要会显示 Codex/Claude 为 `VERIFIED`，Cursor/OpenCode 为 `PREVIEW`，并稳定触发重复 MCP、失效路径和重复上下文三类发现。

## 隐私与修复安全

- 扫描在本地完成，HTML 不引用外部脚本、字体或样式，也不发起网络请求。
- 秘密字段与常见凭据模式在解析后立即脱敏。
- 报告中扫描根目录显示为 `.`，用户主目录显示为 `~`。
- `fix` 默认 dry-run；只有显式 `--apply` 才会修改。
- 自动修复仅限三类 `SAFE` 操作：同一来源中的字节级重复行、已有 shebang Hook 的执行位、指向同一文件对象的规范路径。
- 应用前校验源文件哈希；随后创建仅用户可读的备份，重扫验证；验证失败自动回滚。

秘密检测不可能覆盖所有私有格式。公开报告前仍应人工快速检查，尤其是包含内部编号或专有标识符的配置。

## 支持边界

| 客户端 | 等级 | v0.1 边界 |
|---|---|---|
| Codex | VERIFIED | 精确验证版本为 `0.162.0-alpha.2`；其他版本显示兼容性未知。 |
| Claude Code | VERIFIED | 精确验证版本为 `2.1.259`；其他版本显示兼容性未知。 |
| Cursor | PREVIEW | 检查官方文档声明的本地 Rules 与 MCP 来源，不声明确定的有效覆盖关系。 |
| OpenCode | PREVIEW | 检查本地 JSON/JSONC 与指令来源；不拉取远程配置，也不声明确定的合并优先级。 |

HarnessScope 不启动 MCP 服务、不访问远程配置端点、不猜测客户端未公开的内部行为。上下文 token 数是保守区间估计，不是供应商账单值。

后续路线包括更多版本化夹具、Schema 迁移工具、更强的图谱交互，以及更多编码助手的可验证适配器。详细设计见 [架构文档](docs/architecture.md)，安全反馈见 [SECURITY.md](SECURITY.md)。项目采用 Apache-2.0 许可证。
