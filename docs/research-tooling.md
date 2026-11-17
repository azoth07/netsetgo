# REA 工具发现与限制

日期：2026-11-17。

## 已执行路径

1. 读取指定 `reverse-engineer-anything/SKILL.md`（version 62）、`references/connection-and-recovery.md` 与 `references/native-and-artifacts.md` 的相关章节。
2. 工具发现查询 `+rea open_binary inspect_pe_resources`，返回 0 个工具。
3. shell PATH 中未发现 `rea`；发现 Node 和 npm。搜索现有 npm 缓存找到 `rea-agents` 6.9.0，因此没有下载软件或修改 MCP 注册。
4. 使用现有入口 `C:\Users\Administrator\AppData\Local\npm-cache\_npx\af752b90495482d7\node_modules\rea-agents\scripts\rea.mjs`，通过 Node v26.13.0 读取 CLI 帮助。
5. 运行只读 `doctor --client codex --provider ghidra --format json`；关键结果记录如下。
6. 使用无需原生分析引擎的 `inspect-pe-resources` 完成用户指定 EXE 的静态资源检查，保存完整 JSON Evidence。
7. 使用 PowerShell 文件读取、版本元数据、SHA-256 和有限字符串提取补充证据。这些结果明确不标作 REA Evidence。

## doctor 实际结果摘要

- `healthy: false`、`environment_healthy: false`。
- Node 检查健康，版本 `26.13.0`。
- Ghidra：`missing_analysis_engine`，`GHIDRA_INSTALL_DIR is not set`。提示配置已解压的 Ghidra 12.x（>=12.1），没有按此提示安装新引擎。
- Codex 注册：`config_drift`，identity 中显示 registration state `missing`，路径 `C:\Users\Administrator\.codex\config.toml`。
- IDA：未配置 IDA MCP registration。
- Kuna：未配置 `REA_KUNA_COMMAND`。
- Hopper：缺少分析引擎。
- JADX bridge 显示可用，但它不适用于本次原生 Windows EXE，未使用。
- shared skill 身份检查显示 `.agents/skills` 下缺失；本任务明确指定的 `.codex/skills/reverse-engineer-anything/SKILL.md` 已成功读取，不能把两个位置混为一谈。
- `live_server.state: unknown`；doctor 不证明本会话已有 MCP 连接。

这是一份本轮工具输出的摘录记录，不是伪造的原始 doctor JSON，也不是 REA Evidence。完整的 PE 检查输出独立保存在 [pe-resources.json](research-evidence/pe-resources.json)。

## 命令范围与一次无效探测

曾查询 `inspect-native-imports --help`；该版本未列出此命令，返回通用 CLI 帮助，因此没有取得 REA 导入表证据。后续以实际存在的 `inspect-pe-resources` 为准。未将字符串中的 API 名称误称为已解析导入或实际调用。

读取 `inspect-artifact --help` 确认其他静态入口存在，但本轮无需另建重复清单。未创建活动原生会话；CLI 一次性调用已退出，因此没有需要 `close_binary` 或 `reset_investigation` 清理的持久 MCP 会话。完整 Evidence 已保存到文件。

## 复现

从项目根目录调用：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\docs\research-collect.ps1
```

脚本仅读取目标 EXE 与本地英文帮助，并在 `docs/research-evidence` 写入结果；它不会启动目标程序。默认 CLI 路径是本机现有缓存；路径变化时传入 `-ReaCli`。脚本重新运行会覆盖该目录中同名证据文件，若需保留原始研究身份，应先另存本轮证据。

字符串提取只扫描 ASCII 与偶数字节对齐 UTF-16LE 中列出的固定标记，结果不完整；未命中不证明实现缺失。PE 资源检查对除图标组关系以外的载荷保持不透明，不解码窗体行为。
