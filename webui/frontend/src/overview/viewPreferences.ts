import { categoryNames } from './api';
import { cleanupGrades, type CleanupGradeFilter } from './safety';

export type Grouping = 'files' | 'directories' | 'owners';
export type Location = { name: string; path: string };

export interface ViewPreferences {
  grouping: Grouping;
  category: string | null;
  grade: CleanupGradeFilter | null;
  directoryTrail: Location[];
  owner: Location | null;
}

export function childDirectoryPath(path: string, name: string): string {
  const separator = path.includes('\\') ? '\\' : '/';
  return path.endsWith(separator) ? path + name : path + separator + name;
}

function storageKey(root: string, scope: string): string {
  return `gdu.overview.v1:${JSON.stringify([root, scope])}`;
}

function asRecord(value: unknown): Record<string, unknown> {
  return value !== null && typeof value === 'object' ? value as Record<string, unknown> : {};
}

export function loadViewPreferences(root: string, scope: string, initialGrouping: Grouping): ViewPreferences {
  let saved: Record<string, unknown> = {};
  try {
    saved = asRecord(JSON.parse(localStorage.getItem(storageKey(root, scope)) ?? 'null'));
  } catch {
    // Browsing still works when the browser blocks local storage.
  }
  const directoryTrail = [{ name: scope, path: scope }];
  if (Array.isArray(saved.directoryTrail)) {
    for (const name of saved.directoryTrail.slice(0, 128)) {
      if (typeof name !== 'string' || !name || name === '.' || name === '..' || /[\\/\0]/.test(name)) break;
      directoryTrail.push({ name, path: childDirectoryPath(directoryTrail.at(-1)!.path, name) });
    }
  }
  const grouping = saved.grouping === 'files' || saved.grouping === 'directories' || saved.grouping === 'owners'
    ? saved.grouping : initialGrouping;
  const savedOwner = asRecord(saved.owner);
  const owner = typeof savedOwner.path === 'string' && savedOwner.path.length > 0 && typeof savedOwner.name === 'string'
    ? { path: savedOwner.path, name: savedOwner.name } : null;
  return {
    grouping,
    category: typeof saved.category === 'string' && Object.hasOwn(categoryNames, saved.category) ? saved.category : null,
    grade: saved.grade === null || saved.grade === 'AB' || cleanupGrades.some((grade) => grade === saved.grade)
      ? saved.grade as CleanupGradeFilter | null : grouping === 'files' ? null : 'AB',
    directoryTrail,
    owner,
  };
}

export function saveViewPreferences(root: string, scope: string, preferences: ViewPreferences) {
  try {
    localStorage.setItem(storageKey(root, scope), JSON.stringify({
      ...preferences,
      directoryTrail: preferences.directoryTrail.slice(1).map(({ name }) => name),
    }));
  } catch {
    // Storage can be unavailable or full.
  }
}
