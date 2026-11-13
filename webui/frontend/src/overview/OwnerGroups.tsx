import { formatCount } from '../format';
import { formatOverviewSize } from './formatUsage';
import type { FileOwner, OverviewResult } from './api';
import type { Location } from './viewPreferences';

const ownerKinds: Record<FileOwner['kind'], string> = {
  project: '项目',
  software: '软件',
  folder: '待确认归属',
  environment: '运行环境',
  cache: '软件包缓存',
};

export function OwnerAdvice({ owner }: { owner: FileOwner }) {
  return (
    <dl className="owner-advice">
      <dt>归属依据</dt><dd>{owner.evidence}</dd>
      <dt>清理影响</dt><dd>{owner.impact}</dd>
      <dt>恢复方式</dt><dd>{owner.recovery}</dd>
    </dl>
  );
}

interface Props {
  data: OverviewResult | null;
  selected: Location | null;
  useSIPrefix: boolean;
  onSelect: (owner: Location | null) => void;
  onPageChange: (offset: number) => void;
}

export function OwnerGroups({ data, selected, useSIPrefix, onSelect, onPageChange }: Props) {
  const formatBytes = (bytes: number) => formatOverviewSize(bytes, useSIPrefix);
  const owners = data?.owners ?? [];
  const offset = data?.ownerOffset ?? 0;
  const count = data?.ownerCount ?? 0;

  if (selected) {
    return (
      <div className="owner-groups">
        <nav className="directory-trail" aria-label="项目和软件归属路径">
          <button type="button" className="overview-button" onClick={() => onSelect(null)}>全部归属</button>
          <span className="owner-selection" title={selected.path}>{selected.name}</span>
        </nav>
        <p className="owner-path">{selected.path}</p>
      </div>
    );
  }

  if (!data || owners.length === 0) return null;
  return (
    <div className="owner-groups">
      <div className="directory-heading">
        <h3>项目与软件归属</h3>
        <span>按占用降序</span>
      </div>
      <ul className="owner-list" aria-label="按占用排列的项目与软件">
        {owners.map((owner) => (
          <li key={owner.path}>
            <button className="owner-row" type="button" title={owner.path} onClick={() => onSelect({ name: owner.name, path: owner.path })}>
              <span className="owner-label">
                <span><strong>{owner.name}</strong> <small>{ownerKinds[owner.kind]}</small></span>
                <small>{owner.path}</small>
                <small>A 级 {formatBytes(owner.aBytes)} · B 级 {formatBytes(owner.bBytes)} · {formatCount(owner.count)} 个文件</small>
              </span>
              <strong>{formatBytes(owner.bytes)}</strong>
            </button>
            <details className="owner-details">
              <summary>依据、影响与恢复</summary>
              <OwnerAdvice owner={owner} />
            </details>
          </li>
        ))}
      </ul>
      {count > data.pageSize && (
        <div className="overview-pagination">
          <span>{formatCount(offset + 1)}–{formatCount(offset + owners.length)} / {formatCount(count)} 组</span>
          <div>
            <button type="button" className="overview-button" disabled={offset === 0}
              onClick={() => onPageChange(Math.max(0, offset - data.pageSize))}>上一页归属</button>
            <button type="button" className="overview-button" disabled={offset + owners.length >= count}
              onClick={() => onPageChange(offset + data.pageSize)}>下一页归属</button>
          </div>
        </div>
      )}
    </div>
  );
}
