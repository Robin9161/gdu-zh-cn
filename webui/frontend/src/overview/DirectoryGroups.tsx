import { formatCount } from '../format';
import { formatOverviewSize } from './formatUsage';
import type { OverviewResult } from './api';
import { childDirectoryPath, type Location } from './viewPreferences';

interface Props {
  data: OverviewResult | null;
  trail: Location[];
  useSIPrefix: boolean;
  onNavigate: (trail: Location[]) => void;
  onPageChange: (offset: number) => void;
}

export function DirectoryGroups({ data, trail, useSIPrefix, onNavigate, onPageChange }: Props) {
  const folder = trail[trail.length - 1];
  const directories = data?.directories ?? [];

  return (
    <div className="directory-groups">
      <nav className="directory-trail" aria-label="目录分组路径">
        {trail.map(({ name, path }, index) => (
          <button type="button" key={path} className="overview-button" title={path}
            aria-current={index === trail.length - 1 ? 'location' : undefined}
            disabled={index === trail.length - 1}
            onClick={() => onNavigate(trail.slice(0, index + 1))}>{name}</button>
        ))}
      </nav>
      {data && directories.length > 0 && (
        <>
          <div className="directory-heading">
            <h3>子目录</h3>
            <span>按实际占用降序 · 含下级文件</span>
          </div>
          <ul className="directory-list" aria-label="按占用排列的子目录">
            {directories.map((group) => {
              const path = childDirectoryPath(folder.path, group.id);
              return (
                <li key={path}>
                  <button type="button" title={path} onClick={() => onNavigate([...trail, { name: group.id, path }])}>
                    <span className="directory-name">{group.id}</span>
                    <span className="directory-count">{formatCount(group.count)} 个</span>
                    <strong>{formatOverviewSize(group.bytes, useSIPrefix)}</strong>
                  </button>
                </li>
              );
            })}
          </ul>
          {data.directoryCount > data.pageSize && (
            <div className="overview-pagination">
              <span>{formatCount(data.directoryOffset + 1)}–{formatCount(data.directoryOffset + directories.length)} / {formatCount(data.directoryCount)} 个目录</span>
              <div>
                <button type="button" className="overview-button" disabled={data.directoryOffset === 0}
                  onClick={() => onPageChange(Math.max(0, data.directoryOffset - data.pageSize))}>上一页目录</button>
                <button type="button" className="overview-button"
                  disabled={data.directoryOffset + directories.length >= data.directoryCount}
                  onClick={() => onPageChange(data.directoryOffset + data.pageSize)}>下一页目录</button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
