import { useEffect, useMemo, useState } from 'react';
import { formatCount, formatMtime, percent } from '../format';
import { formatOverviewSize } from './formatUsage';
import { useGduModel } from '../model';
import { categoryNames, fetchOverview, type OverviewFile, type OverviewResult, type PlotBox } from './api';
import { ScatterPlot } from './ScatterPlot';
import { RecycleDialog } from './RecycleDialog';
import { SafetyDetail } from './SafetyDetail';
import { DirectoryGroups } from './DirectoryGroups';
import { OwnerAdvice, OwnerGroups } from './OwnerGroups';
import { loadViewPreferences, saveViewPreferences, type Grouping, type Location } from './viewPreferences';
import { cleanupGrades, gradeLabels, type CleanupGradeFilter } from './safety';
import type { RecycleResult } from '../api';
import './overview.css';

export function OverviewView() {
  const { currentPath, status, effectiveApparent, useSIPrefix, actionToken } = useGduModel();
  const view = new URLSearchParams(window.location.search).get('view');
  return (
    <OverviewScope
      key={JSON.stringify([status.rootPath, currentPath, status.scanRevision])}
      path={currentPath}
      root={status.rootPath}
      apparent={effectiveApparent}
      useSIPrefix={useSIPrefix}
      revision={status.treeRevision ?? 0}
      classifying={status.classifying}
      recycleAllowed={status.recycleAllowed ?? false}
      actionToken={actionToken}
      initialGrouping={view === 'directories' ? 'directories' : view === 'owners' ? 'owners' : 'files'}
    />
  );
}

interface ScopeProps {
  path: string;
  root: string;
  apparent: boolean;
  useSIPrefix: boolean;
  revision: number;
  classifying: boolean;
  recycleAllowed: boolean;
  actionToken: string;
  initialGrouping: Grouping;
}

function OverviewScope({ root, path, apparent: selectedApparent, useSIPrefix, revision, classifying, recycleAllowed, actionToken, initialGrouping }: ScopeProps) {
  const [initial] = useState(() => loadViewPreferences(root, path, initialGrouping));
  const [category, setCategory] = useState(initial.category);
  const [grade, setGrade] = useState(initial.grade);
  const [grouping, setGrouping] = useState(initial.grouping);
  const [directoryTrail, setDirectoryTrail] = useState(initial.directoryTrail);
  const [owner, setOwner] = useState(initial.owner);
  const [directoryOffset, setDirectoryOffset] = useState(0);
  const [ownerOffset, setOwnerOffset] = useState(0);
  const groupByDirectory = grouping === 'directories';
  const groupByOwner = grouping === 'owners';
  const folderPath = groupByDirectory ? directoryTrail.at(-1)?.path ?? path : null;
  const ownerPath = groupByOwner ? owner?.path ?? null : null;
  const apparent = grouping === 'files' && selectedApparent;
  const [brush, setBrush] = useState<{ box: PlotBox; apparent: boolean } | null>(null);
  const [offset, setOffset] = useState(0);
  const [attempt, setAttempt] = useState(0);
  const [response, setResponse] = useState<{ queryKey: string; chartKey: string; apparent: boolean; category: string | null; data: OverviewResult } | null>(null);
  const [failure, setFailure] = useState<{ queryKey: string; message: string } | null>(null);
  const [checked, setChecked] = useState<Map<string, OverviewFile>>(() => new Map());
  const [reviewPaths, setReviewPaths] = useState<string[] | null>(null);
  const activeBox = brush?.apparent === apparent ? brush.box : null;
  const chartKey = JSON.stringify([apparent, category, grade, folderPath, ownerPath, revision]);
  const queryKey = JSON.stringify([chartKey, activeBox, offset, directoryOffset, groupByOwner, ownerOffset, classifying, attempt]);

  useEffect(() => {
    saveViewPreferences(root, path, { grouping, category, grade, directoryTrail, owner });
  }, [root, path, grouping, category, grade, directoryTrail, owner]);

  useEffect(() => {
    if (classifying) return;
    const controller = new AbortController();
    const timeout = window.setTimeout(() => {
      controller.abort();
      setFailure({ queryKey, message: '载入文件分析超时，请重试。' });
    }, 120_000);

    const directory = folderPath === null ? undefined : { path: folderPath, offset: directoryOffset };
    const ownership = groupByOwner ? { path: ownerPath, offset: ownerOffset } : undefined;
    fetchOverview({ path, apparent, category, grade, box: activeBox, offset, directory, ownership }, controller.signal)
      .then((data) => {
        if (!controller.signal.aborted) {
          setResponse({ queryKey, chartKey, apparent, category, data });
          setFailure(null);
        }
      })
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          setFailure({ queryKey, message: error instanceof Error ? error.message : '请重试。' });
        }
      })
      .finally(() => window.clearTimeout(timeout));

    return () => {
      controller.abort();
      window.clearTimeout(timeout);
    };
  }, [path, apparent, category, grade, activeBox, offset, folderPath, directoryOffset, groupByOwner, ownerPath, ownerOffset, classifying, chartKey, queryKey]);

  // A brush belongs to the size metric in which it was drawn.
  useEffect(() => {
    setBrush((previous) => previous?.apparent === apparent ? previous : null);
    setOffset(0);
    setDirectoryOffset(0);
    setOwnerOffset(0);
  }, [apparent]);

  const current = response?.queryKey === queryKey ? response.data : null;
  const failed = failure?.queryKey === queryKey;
  const pending = !current && failure?.queryKey !== queryKey;
  const chartData = response?.apparent === apparent ? response.data : null;
  const gradesInCategory = response?.category === category ? response.data.grades : undefined;
  const gradeAB = gradesInCategory?.reduce((total, group) => {
    if (group.id === 'A' || group.id === 'B') {
      total.count += group.count;
      total.bytes += group.bytes;
    }
    return total;
  }, { count: 0, bytes: 0 });
  const chartIsCurrent = response?.chartKey === chartKey;
  const selectedBytes = useMemo(() => {
    let total = 0;
    for (const file of checked.values()) total += apparent ? file.size : file.usage;
    return total;
  }, [apparent, checked]);
  const selectedUsage = useMemo(() => {
    let total = 0;
    for (const file of checked.values()) total += file.usage;
    return total;
  }, [checked]);

  function resetPages() {
    setOffset(0);
    setDirectoryOffset(0);
    setOwnerOffset(0);
  }

  function chooseCategory(id: string | null) {
    setCategory(id === category ? null : id);
    setBrush(null);
    resetPages();
  }

  function chooseBox(next: PlotBox | null) {
    setBrush(next ? { box: next, apparent } : null);
    resetPages();
  }

  function chooseGrade(next: CleanupGradeFilter | null) {
    setGrade(next);
    resetPages();
  }

  function chooseGrouping(next: Grouping) {
    if (next === grouping) return;
    if (grouping === 'files' && next !== 'files' && grade === null) setGrade('AB');
    setGrouping(next);
    resetPages();
  }

  function chooseOwner(next: Location | null) {
    setOwner(next);
    setOffset(0);
  }

  function setFilesChecked(files: OverviewFile[], value: boolean) {
    setChecked((previous) => {
      const next = new Map(previous);
      for (const file of files) {
        if (value && file.grade !== 'D') next.set(file.path, file);
        else next.delete(file.path);
      }
      return next;
    });
  }

  function applyRecycleResults(results: RecycleResult[]) {
    setChecked((previous) => {
      const remaining = new Map(previous);
      for (const file of results) {
        if (!file.error) remaining.delete(file.path);
      }
      return remaining;
    });
    if (results.some((file) => !file.error)) setBrush(null);
    resetPages();
    setAttempt((value) => value + 1);
  }

  const files = current?.files ?? [];
  const bulkFiles = files.filter((file) => file.grade === 'A' || file.grade === 'B');
  const allChecked = bulkFiles.length > 0 && bulkFiles.every((file) => checked.has(file.path));
  const error = !current && failed ? failure.message : null;
  const unit = apparent ? '文件大小' : '实际占用';
  const formatted = (bytes: number) => formatOverviewSize(bytes, useSIPrefix);
  const leadingBytes = chartData?.categories[0]?.bytes ?? 0;

  return (
    <main className="overview" aria-label="空间总览">
      <div className="overview-intro">
        <h1>空间总览</h1>
        <p className="overview-scope" title={folderPath ?? path}>文件范围 <bdi>{folderPath ?? path}</bdi> · 含子目录</p>
      </div>

      {chartData && (
        <div className="overview-charts">
          <section className="overview-panel" aria-labelledby="category-title">
            <div className="overview-panel-heading">
              <h2 id="category-title">按文件类别</h2>
              <span className="overview-secondary">{formatted(chartData.fileBytes)} · {formatCount(chartData.fileCount)} 个文件</span>
            </div>
            <p className="overview-hint">按扩展名分类，点击查看对应文件。占用为扫描估算，不等于可释放空间。</p>
            {chartData.categories.length > 0 ? (
              <div className="category-bars">
                {chartData.categories.map((group) => (
                  <button type="button" className="category-bar" key={group.id}
                    aria-pressed={category === group.id}
                    onClick={() => chooseCategory(group.id)}>
                    <span className="category-bar-top">
                      <span>{categoryNames[group.id] ?? group.id}</span>
                      <span className="category-bar-value">{formatted(group.bytes)} <small>{percent(group.bytes, chartData.fileBytes).toFixed(1)}%</small></span>
                    </span>
                    <span className="category-bar-track" aria-hidden="true">
                      <span style={{ width: `${percent(group.bytes, leadingBytes)}%` }} />
                    </span>
                  </button>
                ))}
              </div>
            ) : <p className="overview-empty">当前目录没有文件。</p>}
          </section>

          {chartIsCurrent
            ? <ScatterPlot data={chartData} box={activeBox} onSelect={chooseBox} apparent={apparent} useSIPrefix={useSIPrefix} />
            : <section className="overview-panel scatter-panel" aria-busy={pending}>
                <div className="overview-panel-heading"><h2>大小 × 修改时间</h2></div>
                {pending && <p className="overview-empty">正在更新散点图…</p>}
              </section>}
        </div>
      )}

      <section className="overview-panel overview-files" aria-labelledby="file-list-title" aria-busy={pending}>
        <div className="overview-panel-heading">
          <h2 id="file-list-title">文件清单</h2>
          <div className="overview-view-modes" role="group" aria-label="文件清单视图">
            <button type="button" className="overview-button" aria-pressed={grouping === 'files'}
              onClick={() => chooseGrouping('files')}>按文件</button>
            <button type="button" className="overview-button" aria-pressed={groupByDirectory}
              onClick={() => chooseGrouping('directories')}>按目录分组</button>
            <button type="button" className="overview-button" aria-pressed={groupByOwner}
              onClick={() => chooseGrouping('owners')}>按项目或软件</button>
          </div>
          {(category || grade || activeBox) && (
            <button type="button" className="overview-button" onClick={() => {
              setCategory(null);
              setGrade(null);
              chooseBox(null);
            }}>清除筛选</button>
          )}
        </div>
        <div className="safety-filters" role="group" aria-label="清理安全等级">
          <button type="button" className="overview-button" aria-pressed={grade === null}
            onClick={() => chooseGrade(null)}>全部等级</button>
          <button type="button" className="overview-button" aria-pressed={grade === 'AB'}
            onClick={() => chooseGrade('AB')}>
            A + B 级
            {gradeAB && <small>{formatCount(gradeAB.count)} 个 · 实际占用 {formatted(gradeAB.bytes)}</small>}
          </button>
          {cleanupGrades.map((id) => {
            const summary = gradesInCategory?.find((group) => group.id === id);
            return (
              <button type="button" className="overview-button" aria-pressed={grade === id} key={id}
                onClick={() => chooseGrade(id)}>
                <span className="safety-grade" data-grade={id}>{id} 级 · {gradeLabels[id]}</span>
                {summary && <small>{formatCount(summary.count)} 个 · {formatted(summary.bytes)}</small>}
              </button>
            );
          })}
        </div>
        {(!groupByOwner || owner) && (
          <p className="overview-hint safety-hint">批量勾选仅适用于 A、B 级。C 级需逐项勾选，D 级禁止清理。</p>
        )}
        <div className="file-list-meta" aria-live="polite">
          <span>{category ? categoryNames[category] ?? category : '全部类别'}{grade ? ` · ${grade === 'AB' ? 'A + B' : grade} 级` : ''}{activeBox ? ' · 框选范围' : ''}</span>
          {current && <span>{formatCount(current.matchedCount)} 个文件 · {unit} {formatted(current.matchedBytes)}</span>}
        </div>
        {groupByDirectory && (
          <DirectoryGroups data={current} trail={directoryTrail} useSIPrefix={useSIPrefix}
            onPageChange={setDirectoryOffset}
            onNavigate={(trail) => {
              setDirectoryTrail(trail);
              setOffset(0);
              setDirectoryOffset(0);
            }} />
        )}
        {groupByOwner && <OwnerGroups data={current} selected={owner} useSIPrefix={useSIPrefix}
          onSelect={chooseOwner} onPageChange={setOwnerOffset} />}
        {pending && <p className="overview-empty" role="status">{classifying ? '正在识别清理安全等级…' : '正在载入文件分析…'}</p>}
        {error && (
          <div className="overview-error" role="alert">
            <span>无法载入文件分析。{error}</span>
            <button type="button" className="overview-button" onClick={() => setAttempt((value) => value + 1)}>重试</button>
            {groupByDirectory && directoryTrail.length > 1 && (
              <button type="button" className="overview-button" onClick={() => {
                setDirectoryTrail(directoryTrail.slice(0, 1));
                resetPages();
              }}>返回扫描目录</button>
            )}
          </div>
        )}
        {current && files.length === 0 && <p className="overview-empty">当前范围没有匹配的文件。</p>}
        {current && files.length > 0 && (!groupByOwner || owner) && (
          <>
            {groupByDirectory && <h3 className="directory-files-title">文件明细（含子目录）</h3>}
            {groupByOwner && <h3 className="directory-files-title">归属文件明细</h3>}
            <div className="overview-table">
              <table>
                <thead>
                  <tr>
                    <th scope="col"><input type="checkbox" aria-label="批量勾选本页 A、B 级文件" checked={allChecked}
                      disabled={bulkFiles.length === 0}
                      ref={(element) => {
                        if (element) element.indeterminate = !allChecked && bulkFiles.some((file) => checked.has(file.path));
                      }}
                      onChange={(event) => setFilesChecked(bulkFiles, event.target.checked)} /></th>
                    <th scope="col">文件和清理等级</th>
                    <th scope="col" aria-sort="descending">{unit}</th>
                    <th scope="col">修改时间</th>
                  </tr>
                </thead>
                <tbody>
                  {files.map((file) => (
                    <tr key={file.path} className={checked.has(file.path) ? 'is-checked' : ''}>
                      <td><input type="checkbox" aria-label={`勾选 ${file.name}`} checked={file.grade !== 'D' && checked.has(file.path)}
                        disabled={file.grade === 'D'}
                        onChange={(event) => setFilesChecked([file], event.target.checked)} /></td>
                      <td className="overview-file-name">
                        <span title={file.name}>{file.name}</span>
                        <small title={file.path}>{file.path}</small>
                        <SafetyDetail grade={file.grade} reason={file.reason} />
                        {file.owner && (
                          <details className="owner-details">
                            <summary title={file.owner.path}>{groupByOwner ? '依据、影响与恢复' : `归属 · ${file.owner.name}`}</summary>
                            <OwnerAdvice owner={file.owner} />
                          </details>
                        )}
                      </td>
                      <td className="overview-file-number">{formatted(apparent ? file.size : file.usage)}</td>
                      <td className="overview-file-date">{formatMtime(file.mtime > 0 ? file.mtime : 0) || '未知'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="overview-pagination">
              <span>{formatCount(current.offset + 1)}–{formatCount(current.offset + files.length)} / {formatCount(current.matchedCount)}</span>
              <div>
                <button type="button" className="overview-button" disabled={current.offset === 0}
                  onClick={() => setOffset(Math.max(0, current.offset - current.pageSize))}>上一页</button>
                <button type="button" className="overview-button" disabled={current.offset + files.length >= current.matchedCount}
                  onClick={() => setOffset(current.offset + current.pageSize)}>下一页</button>
              </div>
            </div>
          </>
        )}
      </section>

      <footer className="overview-selection" aria-live="polite">
        <div><strong>已勾选 {formatCount(checked.size)} 个文件</strong>
          <span>实际占用 {formatted(selectedUsage)}{apparent ? ` · 文件大小 ${formatted(selectedBytes)}` : ''}</span>
        </div>
        {checked.size > 0 && (
          <div className="overview-selection-actions">
            <button type="button" className="overview-button" onClick={() => setChecked(new Map())}>取消勾选</button>
            {recycleAllowed && (
              <button type="button" className="overview-button recycle-confirm"
                disabled={!actionToken || checked.size > 500}
                onClick={() => setReviewPaths([...checked.keys()])}>查看待清理清单</button>
            )}
          </div>
        )}
        {recycleAllowed && checked.size > 0 && !actionToken && <span>请从 Gdu 的本地启动地址打开本页以启用清理。</span>}
        {recycleAllowed && checked.size > 500 && <span>每批最多 500 个文件，请减少勾选数量。</span>}
      </footer>
      {reviewPaths && <RecycleDialog paths={reviewPaths} checked={checked} actionToken={actionToken} useSIPrefix={useSIPrefix}
        onClose={() => setReviewPaths(null)} onResult={applyRecycleResults} />}
    </main>
  );
}
