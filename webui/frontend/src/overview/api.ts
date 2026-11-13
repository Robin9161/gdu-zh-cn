import { getJSON } from '../api';
import type { CleanupGrade, CleanupGradeFilter, FileSafety } from './safety';

export type PlotBox = readonly [minX: number, maxX: number, minY: number, maxY: number];

export interface FileOwner {
  path: string;
  name: string;
  kind: 'project' | 'software' | 'folder' | 'environment' | 'cache';
  evidence: string;
  impact: string;
  recovery: string;
}

export interface OwnerSummary extends FileOwner {
  bytes: number;
  count: number;
  aBytes: number;
  bBytes: number;
}

export interface OverviewFile extends FileSafety {
  path: string;
  name: string;
  category: string;
  usage: number;
  size: number;
  mtime: number;
  owner?: FileOwner;
}

export interface OverviewResult {
  directories?: { id: string; bytes: number; count: number }[];
  directoryCount: number;
  directoryOffset: number;
  owners?: OwnerSummary[];
  ownerCount?: number;
  ownerOffset?: number;
  categories: { id: string; bytes: number; count: number }[];
  grades: { id: CleanupGrade; bytes: number; count: number }[];
  files: OverviewFile[];
  points: { x: number; y: number; count: number }[];
  fileCount: number;
  fileBytes: number;
  matchedCount: number;
  matchedBytes: number;
  undatedCount: number;
  offset: number;
  pageSize: number;
  columns: number;
  rows: number;
  maxAgeDays: number;
  maxBytes: number;
}

export interface OverviewQuery {
  path: string;
  apparent: boolean;
  category: string | null;
  grade: CleanupGradeFilter | null;
  box: PlotBox | null;
  offset: number;
  directory?: { path: string; offset: number };
  ownership?: { path: string | null; offset: number };
}

export function fetchOverview(query: OverviewQuery, signal: AbortSignal): Promise<OverviewResult> {
  const params = new URLSearchParams({
    path: query.path,
    metric: query.apparent ? 'size' : 'usage',
    offset: String(query.offset),
  });
  if (query.category) params.set('category', query.category);
  if (query.grade) params.set('grade', query.grade);
  if (query.box) params.set('box', query.box.join(','));
  if (query.directory) {
    params.set('group', 'directory');
    params.set('folder', query.directory.path);
    params.set('directoryOffset', String(query.directory.offset));
  }
  if (query.ownership) {
    params.set('group', 'owner');
    if (query.ownership.path) params.set('owner', query.ownership.path);
    params.set('ownerOffset', String(query.ownership.offset));
  }
  return getJSON<OverviewResult>(`api/v1/overview?${params}`, signal);
}

export const categoryNames: Record<string, string> = {
  video: '视频',
  audio: '音频',
  image: '图片',
  model: 'AI 模型',
  installer: '安装包和镜像',
  archive: '压缩包',
  document: '文档和字幕',
  code: '代码和配置',
  project: '工程文件',
  database: '数据库',
  virtual: '虚拟机和磁盘',
  other: '其他',
};
