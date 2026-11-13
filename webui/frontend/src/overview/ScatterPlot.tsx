import { useEffect, useRef, useState, type PointerEvent } from 'react';
import { formatCount, formatSize } from '../format';
import type { OverviewResult, PlotBox } from './api';

type Cell = readonly [x: number, y: number];

function boxBetween(a: Cell, b: Cell): PlotBox {
  return [Math.min(a[0], b[0]), Math.max(a[0], b[0]), Math.min(a[1], b[1]), Math.max(a[1], b[1])];
}

function axisLabels(maximum: number, format: (value: number) => string) {
  return [0, 0.25, 0.5, 0.75, 1].map((fraction) => format(Math.expm1(Math.log1p(maximum) * fraction)));
}

interface ScatterPlotProps {
  data: OverviewResult;
  box: PlotBox | null;
  onSelect: (box: PlotBox | null) => void;
  apparent: boolean;
  useSIPrefix: boolean;
}

export function ScatterPlot({ data, box, onSelect, apparent, useSIPrefix }: ScatterPlotProps) {
  const canvas = useRef<HTMLCanvasElement>(null);
  const dragStart = useRef<Cell | null>(null);
  const [preview, setPreview] = useState<PlotBox | null>(null);
  const [cursor, setCursor] = useState<Cell | null>(null);
  const [anchor, setAnchor] = useState<Cell | null>(null);
  const [hover, setHover] = useState<Cell | null>(null);

  const selected = preview ?? (anchor && cursor ? boxBetween(anchor, cursor) : box);
  const { columns, rows, points } = data;

  useEffect(() => {
    const element = canvas.current;
    if (!element) return;

    const draw = () => {
      const bounds = element.getBoundingClientRect();
      if (!bounds.width || !bounds.height) return;
      const ratio = window.devicePixelRatio || 1;
      element.width = Math.round(bounds.width * ratio);
      element.height = Math.round(bounds.height * ratio);
      const context = element.getContext('2d');
      if (!context) return;
      context.scale(ratio, ratio);

      for (const point of points) {
        const x = (point.x + 0.5) * bounds.width / columns;
        const y = bounds.height - (point.y + 0.5) * bounds.height / rows;
        const inside = !selected || (
          point.x >= selected[0] && point.x <= selected[1] &&
          point.y >= selected[2] && point.y <= selected[3]
        );
        context.globalAlpha = inside ? 0.85 : 0.2;
        context.fillStyle = point.count > 1 ? '#6fdfc3' : '#78aefb';
        context.beginPath();
        context.arc(x, y, Math.min(6, 2.5 + Math.log2(point.count) * 0.6), 0, 2 * Math.PI);
        context.fill();
      }
    };

    draw();
    const observer = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(draw);
    observer?.observe(element);
    return () => observer?.disconnect();
  }, [points, columns, rows, selected]);

  function pointerCell(event: PointerEvent<HTMLDivElement>): Cell {
    const rect = event.currentTarget.getBoundingClientRect();
    const x = Math.floor((event.clientX - rect.left) / Math.max(rect.width, 1) * columns);
    const y = Math.floor((rect.bottom - event.clientY) / Math.max(rect.height, 1) * rows);
    return [Math.max(0, Math.min(columns - 1, x)), Math.max(0, Math.min(rows - 1, y))];
  }

  const sizeTicks = axisLabels(data.maxBytes, (size) => formatSize(size, useSIPrefix)).reverse();
  const timeTicks = axisLabels(data.maxAgeDays, (days) => `${Math.round(days)} 天`);
  const hoveredCount = hover
    ? points.find(({ x, y }) => x === hover[0] && y === hover[1])?.count
    : undefined;

  return (
    <section className="overview-panel scatter-panel" aria-labelledby="scatter-title">
      <div className="overview-panel-heading">
        <h2 id="scatter-title">大小 × 修改时间</h2>
        {box && <button type="button" className="overview-button" onClick={() => onSelect(null)}>清除框选</button>}
      </div>
      <p className="overview-hint" id="scatter-help">
        向右越久未修改，向上占用越大。拖动框选可筛选文件，重叠点按密度显示。
      </p>
      {points.length > 0 ? (
        <>
          <div className="scatter-axis-title">{apparent ? '文件大小' : '实际占用'} · 对数刻度</div>
          <div className="scatter-grid">
            <div className="scatter-y" aria-hidden="true">
              {sizeTicks.map((label, i) => <span key={i}>{label}</span>)}
            </div>
            <div
              className="scatter-plot"
              role="group"
              aria-label="文件大小和修改时间散点图"
              aria-describedby="scatter-help"
              tabIndex={0}
              title="也可用方向键移动，按空格开始、Enter 确认，Esc 清除"
              onBlur={() => { setCursor(null); setAnchor(null); }}
              onPointerDown={(event) => {
                if (event.button !== 0 || !event.isPrimary) return;
                event.currentTarget.focus({ preventScroll: true });
                event.currentTarget.setPointerCapture(event.pointerId);
                dragStart.current = pointerCell(event);
                setPreview(boxBetween(dragStart.current, dragStart.current));
                setAnchor(null);
                setCursor(null);
                setHover(null);
              }}
              onPointerMove={(event) => {
                const cell = pointerCell(event);
                if (dragStart.current) setPreview(boxBetween(dragStart.current, cell));
                else setHover(cell);
              }}
              onPointerLeave={() => setHover(null)}
              onPointerUp={(event) => {
                if (!dragStart.current) return;
                onSelect(boxBetween(dragStart.current, pointerCell(event)));
                dragStart.current = null;
                setPreview(null);
              }}
              onPointerCancel={() => { dragStart.current = null; setPreview(null); }}
              onLostPointerCapture={() => { dragStart.current = null; setPreview(null); }}
              onKeyDown={(event) => {
                const [x, y] = cursor ?? [Math.floor(columns / 2), Math.floor(rows / 2)];
                const step = event.shiftKey ? 8 : 1;
                switch (event.key) {
                  case 'ArrowLeft': setCursor([Math.max(0, x - step), y]); break;
                  case 'ArrowRight': setCursor([Math.min(columns - 1, x + step), y]); break;
                  case 'ArrowDown': setCursor([x, Math.max(0, y - step)]); break;
                  case 'ArrowUp': setCursor([x, Math.min(rows - 1, y + step)]); break;
                  case ' ': setAnchor([x, y]); setCursor([x, y]); break;
                  case 'Enter': onSelect(boxBetween(anchor ?? [x, y], [x, y])); setAnchor(null); break;
                  case 'Escape': onSelect(null); setAnchor(null); break;
                  default: return;
                }
                event.preventDefault();
              }}
            >
              <canvas ref={canvas} aria-hidden="true" />
              {selected && (
                <div className="scatter-selection" aria-hidden="true" style={{
                  left: `${selected[0] / columns * 100}%`,
                  bottom: `${selected[2] / rows * 100}%`,
                  width: `${(selected[1] - selected[0] + 1) / columns * 100}%`,
                  height: `${(selected[3] - selected[2] + 1) / rows * 100}%`,
                }} />
              )}
              {cursor && (
                <span className="scatter-cursor" aria-hidden="true" style={{
                  left: `${(cursor[0] + 0.5) / columns * 100}%`,
                  bottom: `${(cursor[1] + 0.5) / rows * 100}%`,
                }} />
              )}
              {hoveredCount && !preview && <span className="scatter-tooltip">{formatCount(hoveredCount)} 个文件</span>}
            </div>
            <div className="scatter-x" aria-hidden="true">
              {timeTicks.map((label, i) => <span key={i}>{label}</span>)}
            </div>
          </div>
          <div className="scatter-caption">距上次修改 · 对数刻度</div>
        </>
      ) : <p className="overview-empty">当前范围没有带修改时间的文件。</p>}
      {data.undatedCount > 0 && (
        <p className="overview-hint">{formatCount(data.undatedCount)} 个文件没有修改时间，可在下方清单查看。</p>
      )}
    </section>
  );
}
