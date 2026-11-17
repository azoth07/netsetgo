# NetSetGo

使用 MyGo 和 Lobe MyGo 原生 Go 组件构建的中文 Windows 网络设置工具。

## 功能

- 查看网卡、IPv4 地址、掩码、网关、DNS 和 DHCP 状态。
- 编辑并保存网络方案，切换自动获取或静态 IPv4，支持多地址及多个 DNS。
- 导入 `netsetup.bat` 使用的 `config.ini`，兼容逗号分隔 IP、掩码和网关。掩码不足时沿用最后一个掩码。
- 应用前校验参数、预览命令并确认，逐项展示执行结果；检测指定目标的连通性。

## 运行与构建

面向 Windows 10/11，需系统 PowerShell、NetTCPIP 模块和 netsh。开发环境使用 Go 1.27.3 或更高版本，不需要 Node.js、Bun 或 WebView。

```powershell
go run .
```

生成可执行文件和配套文件：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build.ps1
```

生成文件位于 `dist/NetSetGo.exe`。普通启动可读取状态、编辑和保存方案；应用网络配置需要管理员权限。可右键可执行文件选择「以管理员身份运行」，或手动执行 `dist/start-admin.ps1`，由 Windows 弹出 UAC。

应用图标使用 `resources/icon.png`，当前采用 A3 网络小路由器设计。

## GitHub Releases

在仓库的 Releases 页面下载 Windows x64 安装包，或下载便携 ZIP 并完整解压后运行。发布附件包含 `SHA256SUMS.txt`，可用 `Get-FileHash <文件路径> -Algorithm SHA256` 核对。

本地生成与发布流程相同的安装包、便携 ZIP、校验文件和中文发布说明：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/package-release.ps1
```

产物位于 `dist/release/v<版本号>/`。便携包包含程序、图标、示例配置、管理员启动脚本和 README。

发布时先更新 `mygo.json` 的三段式版本号，并提交源码及 `.github/workflows/release.yml`，然后推送对应标签，例如：

```powershell
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions 会检出标签对应代码，按 `go.mod` 安装 Go，执行测试和打包，再上传附件并发布 Release。标签必须与 `mygo.json` 版本一致。也可在 Actions 的 Release 工作流中手动输入已存在的标签。无需额外配置个人访问令牌。

已存在的 Release 不会被自动覆盖；若上传或发布失败，先在 Releases 页面检查并处理残留草稿，再重新运行工作流。

## 使用

1. 选择目标网卡，核对名称、当前地址和网卡编号。
2. 新建方案或加载已保存的方案。静态模式下填写 IPv4 地址与掩码；网关按列表顺序设置不同路由度量。静态模式 DNS 留空会清空 DNS，DHCP 模式自动获取地址与 DNS。
3. 可从 `config.example.ini` 创建自己的 INI 并导入，导入后确认目标网卡。
4. 保存方案，预览并确认应用。网络设置变更可能断开连接；不建议通过正在修改的网卡进行远程操作。
5. 检查操作结果和刷新后的网卡状态；需要时运行连通性检测。

方案存储在 `%APPDATA%\NetSetGo\profiles.json`，使用 UTF-8 无 BOM。保存前校验并通过同目录临时文件替换；文件损坏会报告错误。

当前版本只管理 IPv4。方案应用是依次执行系统命令，失败后停止，已经完成的步骤不会自动回滚。请在变更前保留原配置。连通性检测通过 ICMP，目标拒绝 ICMP 不代表网络一定不可用。

## 验证

```powershell
go test ./...
go vet ./...
go run ./cmd/netsetgo-inspect
```

最后一条命令只读取本机网卡并输出 JSON，不改变配置。单元测试使用纯命令计划或模拟执行器，不切换本机网络。真实 DHCP/静态切换需在可恢复的测试网卡上单独验收。

## 结构

| 位置 | 职责 |
| --- | --- |
| `main.go`、`desktop*.go` | MyGo 窗口、Lobe 组件、异步交互 |
| `internal/network` | 网卡枚举、输入校验、命令计划与执行 |
| `internal/profiles` | 本地方案与 INI 导入 |
| `cmd/netsetgo-inspect` | 只读诊断入口 |
| `docs/netsetman-research.md` | NetSetMan 静态研究与证据边界 |
| `netsetup.bat` | 用户提供的原始参考脚本 |

## 参考项目

- [egoist/mygo](https://github.com/egoist/mygo)：桌面应用架构。
- [lobehub/lobe-mygo](https://github.com/lobehub/lobe-mygo)：原生 UI 组件和主题。
- [dawncopper/windows-netsetup](https://github.com/dawncopper/windows-netsetup)：网络配置流程和 INI 格式。
- 本机 NetSetMan：仅用于功能行为研究，不复制其专有实现或资源。

依赖版本固定在 `go.mod` 和 `go.sum`。
