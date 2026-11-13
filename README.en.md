# Gdu Disk Usage & Cleanup Preview

[简体中文](README.md) | **English**

An unofficial Chinese-language extension of [dundee/gdu](https://github.com/dundee/gdu), based on [v5.39.0](https://github.com/dundee/gdu/releases/tag/v5.39.0). The original project is maintained by Daniel Milde and upstream contributors. Its copyright notice and [MIT license](LICENSE.md) are preserved. [Robin9161](https://github.com/Robin9161) maintains this fork's Chinese interface, disk overview, graded cleanup review, ownership analysis and saved scans under the same MIT license.

**The new features are a Windows source preview. Earlier localization packages in Releases do not include them; no new installer or binary package has been released for this preview.** Use it in a regular browser or the Codex in-app browser. Repository documentation is available in Chinese and English; the application's web interface is currently in Simplified Chinese.

- Explore file categories with horizontal bars and file size versus modification age with a brushable scatter plot.
- Group disk usage by directory, project or software, then open the matching file list.
- Filter by A/B/C/D grades and read classification reasons, ownership evidence, cleanup impact and recovery guidance.
- Review selected files and their total usage before confirming a move to the Windows Recycle Bin.
- Select a drive or folder to rescan, and save or restore the latest scan.
- Restore grouping and filters after a page refresh; file selections and scatter selections are cleared.

## Build and run

Requirements: Windows, Go 1.26 or later, and a supported Node.js version of 22.12 or later. Run from the repository root:

```powershell
npm --prefix webui/frontend ci
npm --prefix webui/frontend test
npm --prefix webui/frontend run build
go build -trimpath -o gdu-zh-cn-windows-amd64.exe ./cmd/gdu
.\gdu-zh-cn-windows-amd64.exe --web --web-open=false --web-recycle-only --web-listen 127.0.0.1:17854 "$env:USERPROFILE\Downloads"
```

Open the complete local URL printed in the terminal, or paste it into the Codex in-app browser. This URL contains an action token for the running process; do not share it publicly. Use **扫描范围** (Scan scope) to switch to the C drive or another folder.

`--web-recycle-only` enables this fork's Windows Recycle Bin review flow and disables the original permanent-delete controls in the web UI. Add `--no-delete` for analysis without deletion. Without `--web-recycle-only`, the upstream deletion modes remain available.

## Cleanup grades

| Grade | Meaning | Selection and cleanup |
| --- | --- | --- |
| A | Recognized generated caches with low expected risk | Bulk selection is available; review is still required |
| B | Caches or dependencies that may be recoverable under specific conditions | Bulk selection is available; removal may affect software or require downloads |
| C | Uncertain purpose or recovery conditions | Files must be selected individually |
| D | Protected content, such as system files or credentials | The server blocks cleanup through this flow |

Grades and ownership are rule-based; grade A is not an absolute safety guarantee. Dependencies may be in use, and the tool does not automatically detect running applications. Windows usage figures are scan estimates, not precise predictions of reclaimable space. Moving files to the Recycle Bin does not itself free their disk space. This fork does not automatically empty the Recycle Bin.

## Saved scans

Use **扫描记录** (Scan records) to save or restore the latest complete scan. It is stored as `state/latest-scan.json.gz` beside the executable. Saving again replaces the previous record. The file contains local paths and metadata and should remain private.

To attempt restoration at startup for the same scan scope, set this variable before running the launch command:

```powershell
$env:GDU_WEB_RESTORE_LATEST = '1'
```

Restored results retain the original scan time, and files are checked again before cleanup. An explicit rescan always reads the current disk.

See the [preview guide](docs/WINDOWS-PREVIEW.en.md) for validation coverage, limitations and planned improvements. The [upstream README](README.upstream.md) documents original Gdu features; upstream download links do not include this fork's extensions.
