# Project ownership and saved scans

Status: accepted for the local Windows preview (2026-11-13).

## Purpose

Help users understand which project or tool uses A/B candidates before choosing
files for review. Preserve a completed scan so returning to the preview does not
always require reading the disk again. This is not a release or a cleanup run.

## Ownership

`webui/ownership.go` derives ownership from the existing scanned tree. Project
markers identify roots; nested packages inside node_modules stay attached to
their owning project. Known software folders and package caches have explicit
evidence. Unrecognized files fall back to their parent directory, labeled as
unconfirmed. It does not inspect running processes or promise dependency recovery.

The overview inventory shares owner records per owner/impact and aggregates
actual disk usage with its existing hard-link accounting. Owner groups show A/B
subtotals and the strongest applicable impact. Mixed groups containing protected
files say so without labeling every file protected. Grade counters respect the
selected category, directory and owner, before grade and scatter filters.

The recycle preview uses the same resolver, so its reasons and impact come from
the server. A means low risk, not an unconditional guarantee. C still requires
individual selection and D is blocked by the server.

## Saved scan format and lifecycle

`webui/snapshots.go` writes `state/latest-scan.json.gz` beside the preview binary.
Gzip extra metadata carries the version, absolute root, original scan time,
save time and filtering flag; the body is the existing Gdu JSON report. A temp
file in the same directory is flushed and closed before replacement. There is
one latest record; saving replaces it. Archive and virtual multi-root scans are
not supported by this first version.

Save/restore requests return immediately and report task status. A shared action
lock prevents overlapping scan/recycle/snapshot mutations, returning conflict
instead of silently queueing a later cleanup. Invalid or truncated reports do
not replace the current tree. Report import consumes directories as a stream to
avoid holding a second complete JSON object graph for a large disk.

`GDU_WEB_RESTORE_LATEST=1` enables startup restore only for the same requested
root and compatible filtering mode. A failed startup restore falls back to a
normal scan. Explicit web rescans always scan. Restoring invalidates old recycle
plans, caches and selection, reclassifies the restored tree, and shows its time.
The existing live file checks remain mandatory before recycling.

## Browsing preferences

The overview stores grouping, directory, owner, category and grade locally per
scan root and browsing scope. Selection and scatter coordinates are never
restored: new scans can change the plot axes. Browser storage failures do not
prevent browsing. This is not a cross-device preference store.

## Validation and limits

Frontend interaction tests cover owner drill-down and preference recovery.
Windows backend tests cover owner aggregation, nested dependencies, safety
reasons, snapshot replacement, corrupt input, C-drive paths, original timestamps,
startup restore, explicit rescanning, action conflicts and stale-file rejection.
Manual browser checks use disposable fixtures for save/restore and review; no
real files are recycled during acceptance.

Full scans still require memory proportional to their tree and inventory.
Restoring while an old tree is active temporarily requires both trees. Ownership
is evidence-based inference; cache contents may need downloads and dependencies
may be required by active projects. The latest scan is historical data, not live
disk state. Moving files to the Windows Recycle Bin does not itself release their
disk space.
