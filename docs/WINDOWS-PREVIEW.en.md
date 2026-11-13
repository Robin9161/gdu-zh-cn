# Windows Preview Guide

[简体中文](WINDOWS-PREVIEW.md) | **English** · [Home](../README.en.md)

This fork is a source preview for local disk analysis and manually reviewed cleanup. It is not an unattended cleaner and does not automatically empty the Recycle Bin. The new recycling, scan selection and saved-scan features are available only on Windows in `--web-recycle-only` mode with a loopback listener.

## Extensions

- Category bars, file size versus modification age scatter plots, linked file lists and selection totals.
- A/B/C/D grades combined with category filters; individual selection for C and server-side rejection for D.
- Directory or project/software grouping ordered by disk usage, with ownership evidence and impact explanations.
- A review step before recycling, with checks of file identity, size, modification time, resolved paths and protection rules.
- Local drive/folder selection, rescanning and cancellation; usable previous results are retained on failure.
- Save/restore of the latest scan, original scan timestamps and browsing preferences.

See [ADR 0004](adr/0004-cleanup-ownership-and-scan-snapshots.md) for architectural decisions. The upstream Go module path is retained for internal imports and existing build workflows. This fork is not endorsed by upstream.

## Validation coverage

Windows checks cover frontend interaction, grouping and filtering, cleanup-preview protections, scan switching, snapshot replacement, corrupt-snapshot recovery, startup restoration and explicit rescanning. A real large-disk scan was saved and restored after a process restart, with matching file counts, usage totals and ownership group counts. User files were not used as cleanup test targets.

Automated recycling tests use a test double to check review plans, identity changes and protection rules. Before releasing a downloadable cleanup build, real recycling, restoration and unsupported-volume behavior still need acceptance testing in an isolated Windows environment. This preview does not claim validation across all Windows storage devices.

`preview-checks.yml` runs frontend checks, Linux Go tests and relevant Windows tests for the Chinese branch. The Windows checks explicitly exclude three issues reproduced with upstream code:

- `TestReadAnalysis`: expects Unix path separators.
- `TestAnalyzePathWithSummarizeFromSqliteStorage` and `TestAnalyzePathWithDepthFromSqliteStorage`: cleanup encounters SQLite temporary files that are still open.

These exclusions do not skip the new protection and snapshot tests. They also mean that the complete upstream Windows suite is not claimed to pass.

## Known limitations

- The upstream Windows scanner generally estimates ordinary-file usage from file size. Hard-link deduplication and allocation-unit accounting have limitations. Displayed usage is not a precise measure of reclaimable space.
- File checks use identity, size and modification time rather than content hashes. Scan timestamp precision and path-based Shell operations have limitations; not every concurrent change can be detected.
- Memory use grows with the number of scanned files. Restoring a different record in a running process may temporarily retain two trees.
- Inaccessible directories may be skipped; totals describe the visible scan scope.
- Ownership is inferred from paths and project markers. It does not detect running applications or prove that dependencies can be downloaded again.
- Only the latest record is retained. Restored results are historical, and the interface displays their scan time.
- Directory grouping and ownership grouping are separate views. Check the displayed file scope after switching.
- Recycle Bin availability depends on volume type, system settings and file state. Unsupported operations return an error without falling back to permanent deletion.
- Keep scan records, startup tokens, screenshots containing real paths and logs local; do not publish them in public issues or repositories.

## Priorities

1. **Isolated recycling and restoration acceptance tests:** cover Windows storage boundaries before a formal binary release.
2. **Scan efficiency:** incremental updates, lower peak memory, and progress/cancellation for large save and restore operations.
3. **Historical comparisons:** show new usage and fast-growing directories, with multiple snapshots and retention policies.
4. **Ownership accuracy:** improve virtual-environment, installation-directory and project rules, and allow user corrections.
5. **Targeted cleanup:** saved/excluded folders, combined age and size filters, and duplicate-file review without automatically deleting unique copies.
