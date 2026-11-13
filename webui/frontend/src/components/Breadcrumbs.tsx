import type { Node } from '../types';

interface BreadcrumbsProps {
  breadcrumbs: Node[];
  onNavigate: (node: Node) => void;
}

export function Breadcrumbs({ breadcrumbs, onNavigate }: BreadcrumbsProps) {
  return (
    <nav className="breadcrumbs" aria-label="目录路径">
      {breadcrumbs.map((node, i) => {
        const isLast = i === breadcrumbs.length - 1;
        const label = i === 0 && node.path === '(multiple)' && node.name === '(multiple)'
          ? '扫描目录总览'
          : node.name || node.path;
        return (
          <span key={node.path} className="crumb">
            {isLast ? (
              <span className="crumb-current">{label}</span>
            ) : (
              <button type="button" className="crumb-link" onClick={() => onNavigate(node)}>
                {label}
              </button>
            )}
            {!isLast && <span className="crumb-sep">/</span>}
          </span>
        );
      })}
    </nav>
  );
}
