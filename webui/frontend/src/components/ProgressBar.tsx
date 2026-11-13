import type { Progress } from '../types';
import { formatCount, formatSize } from '../format';

interface ProgressBarProps {
  progress: Progress;
  useSIPrefix: boolean;
  target?: string;
}

export function ProgressBar({ progress, useSIPrefix, target }: ProgressBarProps) {
  return (
    <div className="scanning">
      <div className="spinner" />
      <h2>正在扫描…</h2>
      {target && <p className="scan-target">{target}</p>}
      <div className="scan-stats">
        <span>{formatCount(progress.itemCount)} 项</span>
        <span>{formatSize(progress.totalUsage, useSIPrefix)}</span>
      </div>
      <div className="scan-current" title={progress.currentItem}>
        {progress.currentItem}
      </div>
    </div>
  );
}
