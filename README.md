# Gdu 中文磁盘分析与清理预览

**简体中文** | [English](README.en.md)

*Disk usage visualization, graded cleanup review, project ownership and saved scans. [Read the English guide →](README.en.md)*

这是 [dundee/gdu](https://github.com/dundee/gdu) 的非官方中文扩展分支，基于 [v5.39.0](https://github.com/dundee/gdu/releases/tag/v5.39.0)。原项目由 Daniel Milde 和上游贡献者维护，原始版权声明和 [MIT 许可证](LICENSE.md) 保持不变。本分支的中文界面、空间总览、分级清理、归属分析和扫描记录扩展由 [Robin9161](https://github.com/Robin9161) 维护，同样按 MIT 许可证提供。

**当前新增功能属于 Windows 源码预览版。既有 Releases 的早期汉化包不包含这些新功能，本次没有发布新的安装包。** 可以在普通浏览器或 Codex 内置浏览器中使用。仓库说明提供中英文版本；应用网页界面目前为简体中文。

- 按文件类别显示横向条形图，按大小与修改时间显示可框选散点图。
- 按目录、项目或软件汇总占用；点击进入文件清单。
- A/B/C/D 四级筛选，显示分级理由、归属依据、清理影响和恢复方式。
- 勾选后显示合计占用，核对清单后通过 Windows 回收站处理。
- 选择盘符或文件夹重新扫描；保存并恢复最近一份扫描。
- 刷新恢复分组与筛选，清空勾选和散点框选。

## 从源码运行

需要 Windows、Go 1.26 或更新版本和 Node.js 22.12 或更高的受支持版本。在仓库根目录运行：

```powershell
npm --prefix webui/frontend ci
npm --prefix webui/frontend test
npm --prefix webui/frontend run build
go build -trimpath -o gdu-zh-cn-windows-amd64.exe ./cmd/gdu
.\gdu-zh-cn-windows-amd64.exe --web --web-open=false --web-recycle-only --web-listen 127.0.0.1:17854 "$env:USERPROFILE\Downloads"
```

打开终端打印的完整本地地址，或将其粘贴到 Codex 的内置浏览器。该地址含本次进程的操作令牌，请勿公开分享。页面中的“扫描范围”可切换到 C 盘或其他文件夹。

`--web-recycle-only` 启用本分支的 Windows 回收站预览流程，并禁用网页中的原有永久删除入口。只想分析时，再加 `--no-delete`。未使用 `--web-recycle-only` 时，上游原有删除模式仍然存在。

## 分级含义

| 等级 | 含义 | 选择与清理 |
| --- | --- | --- |
| A | 已识别的低风险生成缓存 | 可批量勾选，仍需核对清单 |
| B | 有条件可恢复的缓存或依赖 | 可批量勾选；清理可能影响软件运行或需要重新下载 |
| C | 用途或恢复条件不明确 | 只能逐项勾选 |
| D | 系统、凭据等受保护内容 | 服务端禁止通过此流程清理 |

分级和归属均基于规则，A 级也不是绝对安全承诺。项目依赖可能正在使用，工具不会自动确认运行状态。Windows 占用值是扫描估算，不能当作精确可释放空间。移入回收站本身不会释放对应磁盘空间；本分支不提供自动清空回收站。

## 扫描记录

点击“扫描记录”保存或恢复最近一份完整扫描，文件保存在可执行文件旁的 `state/latest-scan.json.gz`。再次保存会替换上次记录。该文件包含本机文件路径和元数据，应仅保留在本地。

如需启动时尝试恢复同一扫描范围，在启动命令前设置：

```powershell
$env:GDU_WEB_RESTORE_LATEST = '1'
```

恢复结果保留原扫描时间，清理前会重新核对文件。手动重新扫描始终读取当前磁盘。

已知限制、验证范围和后续方向见 [预览版说明](docs/WINDOWS-PREVIEW.md)。原始 Gdu 功能见 [上游 README](README.upstream.md)；其中的上游下载链接不包含本分支扩展。
