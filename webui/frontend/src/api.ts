import { localizeError } from './zh-CN';
import type { FileSafety } from './overview/safety';
import type { FileOwner } from './overview/api';
import type { CategoriesResponse, NodeResponse, SortKey, SortOrder, Status, TreeNode } from './types';

async function responseError(res: Response): Promise<Error & { status: number }> {
  let message = `请求失败（HTTP ${res.status}）`;
  try {
    const body = (await res.json()) as { error?: string };
    if (body.error) {
      message = localizeError(body.error);
    }
  } catch {
    // ignore non-JSON error bodies
  }
  return Object.assign(new Error(message), { status: res.status });
}

export async function getJSON<T>(url: string, signal?: AbortSignal): Promise<T> {
  const res = await fetch(url, signal ? { signal } : undefined);
  if (!res.ok) {
    throw await responseError(res);
  }
  return (await res.json()) as T;
}

export function fetchStatus(): Promise<Status> {
  return getJSON<Status>('api/v1/status');
}

export function fetchNode(
  path: string,
  sort: SortKey,
  order: SortOrder,
): Promise<NodeResponse> {
  const params = new URLSearchParams({ path, sort, order });
  return getJSON<NodeResponse>(`api/v1/nodes?${params.toString()}`);
}

// fetchTree loads the recursive tree below path. With byCategory, it is the
// tree of the "By category" projection: one synthetic node per cleanup
// category (its path starts with CATEGORY_NODE_PREFIX, see cleanup.ts)
// holding the candidates with their real paths.
export function fetchTree(path: string, byCategory = false): Promise<TreeNode> {
  const params = new URLSearchParams({ path });
  if (byCategory) {
    params.set('projection', 'category');
  }
  return getJSON<TreeNode>(`api/v1/tree?${params.toString()}`);
}

export function fetchCategories(path: string): Promise<CategoriesResponse> {
  const params = new URLSearchParams({ path });
  return getJSON<CategoriesResponse>(`api/v1/categories?${params.toString()}`);
}

// requestAction sends a mutating request carrying the per-process action
// token the server printed into this page's own URL (see
// GduModel.actionToken in model.tsx). The server rejects the request unless
// this exact token is echoed back, which keeps a page on another origin (or
// another browser tab, or another local user who never saw this URL) from
// triggering a delete/reveal even though it may still be able to make the
// browser send a request: it does not know the token, since the token is
// never served over any API response.
async function requestAction(url: string, token: string, init: RequestInit): Promise<Response> {
  const res = await fetch(url, {
    ...init,
    headers: {
      'X-GDU-Action': token,
      ...init.headers,
    },
  });
  if (res.ok) {
    return res;
  }
  throw await responseError(res);
}

export async function revealNode(path: string, token: string): Promise<void> {
  await requestAction('api/v1/reveal', token, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  });
}

export async function fetchScanDrives(token: string, signal: AbortSignal): Promise<{ path: string }[]> {
  const response = await requestAction('api/v1/scan/drives', token, { signal });
  return response.json();
}

export interface ScanFolders {
  path: string;
  parent: string;
  folders: string[];
}

export async function fetchScanFolders(path: string, token: string, signal: AbortSignal): Promise<ScanFolders> {
  const params = new URLSearchParams({ path });
  const response = await requestAction(`api/v1/scan/folders?${params}`, token, { signal });
  return response.json();
}

export async function startScan(path: string, token: string): Promise<void> {
  await requestAction('api/v1/scan', token, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  });
}

export async function cancelScan(token: string): Promise<void> {
  await requestAction('api/v1/scan/cancel', token, { method: 'POST' });
}

export type DeleteMode = 'trash' | 'permanent';

export async function deleteNode(path: string, token: string, mode: DeleteMode): Promise<void> {
  const params = new URLSearchParams({ path });
  if (mode === 'trash') {
    params.set('mode', 'trash');
  }
  await requestAction(`api/v1/nodes?${params.toString()}`, token, { method: 'DELETE' });
}

export interface RecycleResult {
  path: string;
  error?: string;
}

export interface RecyclePlan {
  id: string;
  files: (RecycleResult & FileSafety & { name: string; usage: number; owner?: FileOwner })[];
  usage: number;
}

async function recycleRequest<T>(route: string, data: object, token: string, signal?: AbortSignal): Promise<T> {
  const response = await requestAction(route, token, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
    signal,
  });
  return response.json() as Promise<T>;
}

export function previewRecycle(paths: string[], token: string, signal: AbortSignal) {
  return recycleRequest<RecyclePlan>('api/v1/recycle/preview', { paths }, token, signal);
}

export function confirmRecycle(id: string, token: string) {
  return recycleRequest<RecycleResult[]>('api/v1/recycle/confirm', { id }, token);
}

// subscribeStatus opens an SSE connection that yields status updates. Returns
// an unsubscribe function.
export function subscribeStatus(onStatus: (status: Status) => void): () => void {
  const source = new EventSource('api/v1/events');
  source.onmessage = (event: MessageEvent<string>) => {
    try {
      onStatus(JSON.parse(event.data) as Status);
    } catch {
      // ignore malformed frames
    }
  };
  return () => source.close();
}
