# NetSetGo v{{version}}

适用于 Windows 10/11 x64，提供中文网卡状态查看、IPv4/DNS 方案管理、INI 导入和配置应用。

## 更新内容

- 在已保存方案列表中直接应用或删除方案，应用前仍会预览并确认。

## 下载与使用

- 安装版：下载 `{{installer}}` 并运行安装程序。
- 便携版：下载 `{{archive}}`，完整解压后运行 `NetSetGo.exe`。
- 无需安装 Go、Node.js 或 WebView。应用网络配置时需要管理员权限；便携版可右键以管理员身份运行，或使用 `start-admin.ps1`。

## 校验

使用 PowerShell 的 `Get-FileHash <文件路径> -Algorithm SHA256`，与 `SHA256SUMS.txt` 对照。

## 验证与限制

- 发布构建已通过 `go test ./...` 并完成 Windows amd64 打包。
- 仅管理 IPv4；修改网络可能中断连接，部分步骤失败不会自动回滚。
- 自动构建不代表已完成真实网卡切换验收。
