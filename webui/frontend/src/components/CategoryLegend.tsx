import type { CategoriesResponse, Category } from '../types';
import { formatSize, percent } from '../format';
import { categoryLabel } from '../zh-CN';
import {
  AGE_BUCKETS,
  CATEGORY_COLORS,
  REVIEW_COLOR,
  SAFE_COLOR,
  amountValue,
  type Projection,
} from '../cleanup';

interface CategoryLegendProps {
  projection: Projection;
  categories: CategoriesResponse | null;
  apparent: boolean;
  useSIPrefix: boolean;
  filter: Category | null;
  onFilter: (category: Category | null) => void;
}

function Swatch({ color }: { color: string }) {
  return <span className="legend-swatch" style={{ background: color }} />;
}

// CategoryLegend explains the active projection and, for the cleanup ones,
// lists the space each category takes within the current directory. Clicking
// a category highlights only the tiles holding it.
export function CategoryLegend({
  projection,
  categories,
  apparent,
  useSIPrefix,
  filter,
  onFilter,
}: CategoryLegendProps) {
  if (projection === 'location') {
    return null;
  }
  if (projection === 'age') {
    return (
      <div className="legend" aria-label="修改时间图例">
        <span className="legend-title">最近修改</span>
        {AGE_BUCKETS.map((b) => (
          <span key={b.label} className="legend-key">
            <Swatch color={b.color} />
            {b.label}
          </span>
        ))}
      </div>
    );
  }
  if (!categories) {
    return <div className="legend legend-loading">正在分类…</div>;
  }

  const total = amountValue(categories.total, apparent);
  const present = categories.categories
    .map((c) => ({ ...c, bytes: amountValue(c, apparent) }))
    .filter((c) => c.bytes > 0);
  const reclaimable = present.reduce((sum, c) => sum + c.bytes, 0);

  return (
    <div className="legend" aria-label="清理候选图例">
      {present.length === 0 ? (
        <span className="legend-title">此目录没有清理候选</span>
      ) : (
        <span className="legend-title">
          清理候选 {formatSize(reclaimable, useSIPrefix)} ({percent(reclaimable, total).toFixed(0)}%)
        </span>
      )}
      {projection === 'cleanup' && present.length > 0 && (
        <>
          <span className="legend-key">
            <Swatch color={SAFE_COLOR} />
            通常可重新生成
          </span>
          <span className="legend-key">
            <Swatch color={REVIEW_COLOR} />
            需先核查
          </span>
        </>
      )}
      {present.map((c) => {
        const active = filter === c.id;
        const color = CATEGORY_COLORS[c.id];
        const label = categoryLabel(c.id, c.label);
        return (
          <button
            key={c.id}
            type="button"
            className={`legend-chip ${active ? 'active' : ''}`}
            aria-pressed={active}
            title={active ? '显示所有类别（Esc）' : `突出显示${label}`}
            style={active ? { borderColor: color, boxShadow: `inset 0 0 0 1px ${color}` } : undefined}
            onClick={() => onFilter(active ? null : c.id)}
          >
            <Swatch color={color} />
            {label} · {formatSize(c.bytes, useSIPrefix)} · {percent(c.bytes, total).toFixed(0)}%
          </button>
        );
      })}
      {filter && (
        <button type="button" className="legend-clear" onClick={() => onFilter(null)}>
          显示全部
        </button>
      )}
    </div>
  );
}
