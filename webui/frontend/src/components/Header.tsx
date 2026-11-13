import { freedBytes, useGduModel } from '../model';
import { formatSize } from '../format';
import { Breadcrumbs } from './Breadcrumbs';
import { PROJECTIONS, type Projection } from '../cleanup';

export function Header() {
  const {
    status,
    nodeResp,
    total,
    useSIPrefix,
    view,
    toggleView,
    projection,
    setProjection,
    effectiveApparent,
    toggleApparent,
    handleSelect,
    deletionsInFlight,
    deletions,
  } = useGduModel();
  const freed = freedBytes(deletions, effectiveApparent);

  return (
    <header className="app-header">
      <div className="app-title">
        <span className="logo">gdu</span>
        {nodeResp && <Breadcrumbs breadcrumbs={nodeResp.breadcrumbs} onNavigate={handleSelect} />}
      </div>
      <div className="app-actions">
        <span className="total">{formatSize(total, useSIPrefix)}</span>
        <button
          type="button"
          className="toggle"
          onClick={toggleView}
          title="在环形图和矩形树图之间切换"
        >
          {view === 'donut' ? '矩形树图' : '环形图'}
        </button>
        {view === 'treemap' && (
          <select
            className="toggle"
            aria-label="树图着色方式"
            title="按目录、清理候选或修改时间着色，也可按类别分组（P）"
            value={projection}
            onChange={(event) => setProjection(event.target.value as Projection)}
          >
            {PROJECTIONS.map((p) => (
              <option key={p.id} value={p.id}>
                {p.label}
              </option>
            ))}
          </select>
        )}
        <button
          type="button"
          className="toggle"
          onClick={toggleApparent}
          title="在磁盘实际占用和文件大小之间切换"
        >
          {effectiveApparent ? '文件大小' : '实际占用'}
        </button>
        {status.state === 'scanning' && <span className="scanning-badge">正在扫描…</span>}
        {status.classifying && (
          <span
            className="scanning-badge"
            role="status"
            title="正在识别清理候选，完成后会更新树图"
          >
            正在分类…
          </span>
        )}
        {deletionsInFlight > 0 && (
          <span
            className="scanning-badge"
            role="status"
            title="正在后台删除，可以继续浏览"
          >
            正在删除 {deletionsInFlight} 项…
          </span>
        )}
        {freed > 0 && (
          <span
            className="freed-badge"
            title="当前目录已释放的空间。打开其他目录时会更新占用。"
          >
            已释放 {formatSize(freed, useSIPrefix)}
          </span>
        )}
      </div>
    </header>
  );
}
