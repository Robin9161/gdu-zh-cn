import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { OverviewView } from './OverviewView';
import { fetchOverview, type FileOwner, type OverviewResult, type PlotBox } from './api';

vi.mock('../model', () => ({
  useGduModel: () => ({
    currentPath: 'C:\\',
    status: { rootPath: 'C:\\', scanRevision: 1 },
    effectiveApparent: false,
    useSIPrefix: false,
    actionToken: '',
  }),
}));
vi.mock('./api', async (importOriginal) => ({
  ...await importOriginal<typeof import('./api')>(),
  fetchOverview: vi.fn(),
}));
vi.mock('./ScatterPlot', () => ({
  ScatterPlot: ({ onSelect }: { onSelect: (box: PlotBox) => void }) => (
    <button type="button" onClick={() => onSelect([0, 1, 0, 1])}>框选样本</button>
  ),
}));

afterEach(() => {
  cleanup();
  localStorage.clear();
  vi.clearAllMocks();
});

it('restores owner and grade filters after reopening while clearing selection and chart brush', async () => {
  const owner: FileOwner = {
    name: 'demo',
    path: 'C:\\demo',
    kind: 'project',
    evidence: '发现 package.json',
    impact: '项目可能需要重新安装依赖',
    recovery: '使用项目的包管理器恢复',
  };
  const file = {
    path: 'C:\\demo\\node_modules\\dependency.js',
    name: 'dependency.js',
    category: 'code',
    grade: 'B' as const,
    reason: '项目依赖',
    owner,
    mtime: 0,
    size: 80,
    usage: 100,
  };
  const data: OverviewResult = {
    directoryCount: 0,
    directoryOffset: 0,
    categories: [],
    grades: [{ id: 'B', bytes: 100, count: 1 }],
    files: [file],
    points: [],
    fileCount: 1,
    fileBytes: 100,
    matchedCount: 1,
    matchedBytes: 100,
    undatedCount: 1,
    offset: 0,
    pageSize: 50,
    columns: 96,
    rows: 48,
    maxAgeDays: 1,
    maxBytes: 100,
    owners: [{ ...owner, bytes: 100, count: 1, aBytes: 0, bBytes: 100 }],
    ownerCount: 1,
    ownerOffset: 0,
  };
  vi.mocked(fetchOverview).mockResolvedValue(data);
  const first = render(<OverviewView />);
  fireEvent.click(screen.getByRole('button', { name: '按项目或软件' }));
  fireEvent.click(await screen.findByRole('button', { name: /^demo 项目/ }));
  fireEvent.click(await screen.findByRole('button', { name: '框选样本' }));
  expect(vi.mocked(fetchOverview).mock.lastCall?.[0].box).toEqual([0, 1, 0, 1]);
  const checkbox = await screen.findByRole('checkbox', { name: '勾选 dependency.js' });
  fireEvent.click(checkbox);
  expect((checkbox as HTMLInputElement).checked).toBe(true);
  first.unmount();

  const storageKey = localStorage.key(0)!;
  const saved = JSON.parse(localStorage.getItem(storageKey)!);
  expect(saved).not.toHaveProperty('brush');
  localStorage.setItem(storageKey, JSON.stringify({ ...saved, brush: { box: [0, 1, 0, 1], apparent: false } }));

  render(<OverviewView />);
  const restoredCheckbox = await screen.findByRole('checkbox', { name: '勾选 dependency.js' });
  expect((restoredCheckbox as HTMLInputElement).checked).toBe(false);
  expect(vi.mocked(fetchOverview).mock.lastCall?.[0]).toMatchObject({
    grade: 'AB',
    box: null,
    ownership: { path: owner.path, offset: 0 },
  });
  fireEvent.click(screen.getByRole('button', { name: '全部归属' }));
  expect(await screen.findByRole('button', { name: /^demo 项目/ })).toBeTruthy();
});
