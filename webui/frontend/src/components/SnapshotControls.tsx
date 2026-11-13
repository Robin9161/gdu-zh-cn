import { useEffect, useRef, useState } from 'react';
import type { ScanState } from '../types';
import './snapshot.css';

type SnapshotAction = 'save' | 'restore';

interface SnapshotState {
  busy?: SnapshotAction;
  message?: string;
  error?: string;
  savedError?: string;
  canSave: boolean;
  saved?: {
    rootPath: string;
    scanTime: string;
    savedAt: string;
  };
}

async function snapshotRequest(token: string, signal: AbortSignal, action?: SnapshotAction): Promise<SnapshotState> {
  const headers = new Headers({ 'X-GDU-Action': token });
  if (action) headers.set('Content-Type', 'application/json');
  const response = await fetch('api/v1/snapshot', {
    method: action ? 'POST' : 'GET',
    headers,
    body: action ? JSON.stringify({ action }) : undefined,
    signal,
  });
  if (!response.ok) {
    const data = await response.json().catch(() => null) as { error?: string } | null;
    throw new Error(data?.error ?? `无法读取扫描记录（HTTP ${response.status}）`);
  }
  return response.json();
}

function snapshotDate(value: string): string {
  return new Date(value).toLocaleString('zh-CN', { dateStyle: 'medium', timeStyle: 'short' });
}

export function SnapshotControls({ token, scanState }: { token: string; scanState: ScanState }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button type="button" className="toggle" disabled={!token || scanState === 'scanning'}
        onClick={() => setOpen(true)}>扫描记录</button>
      {open && <SnapshotDialog token={token} onClose={() => setOpen(false)} />}
    </>
  );
}

function SnapshotDialog({ token, onClose }: { token: string; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const controller = useRef<AbortController | null>(null);
  const [status, setStatus] = useState<SnapshotState | null>(null);
  const [error, setError] = useState('');
  const [pending, setPending] = useState<SnapshotAction | null>(null);
  const [revision, setRevision] = useState(0);

  useEffect(() => {
    const element = dialog.current;
    element?.showModal();
    return () => element?.close();
  }, []);

  useEffect(() => {
    const request = new AbortController();
    controller.current = request;
    let timer: number | undefined;

    async function update() {
      try {
        const next = await snapshotRequest(token, request.signal);
        if (request.signal.aborted) return;
        setStatus((previous) => next.busy ? { ...next, saved: previous?.saved } : next);
        setError('');
        if (next.busy) timer = window.setTimeout(() => void update(), 800);
      } catch (err) {
        if (!request.signal.aborted) setError(err instanceof Error ? err.message : '无法读取扫描记录');
      }
    }

    void update();
    return () => {
      request.abort();
      window.clearTimeout(timer);
    };
  }, [token, revision]);

  async function run(action: SnapshotAction) {
    const signal = controller.current?.signal;
    if (!signal || signal.aborted || pending || status?.busy) return;
    setPending(action);
    setError('');
    try {
      const started = await snapshotRequest(token, signal, action);
      if (!signal.aborted) {
        setStatus((previous) => ({ ...previous, ...started, message: undefined, error: undefined }));
        setRevision((previous) => previous + 1);
      }
    } catch (err) {
      if (!signal.aborted) setError(err instanceof Error ? err.message : '操作失败，请重试');
    } finally {
      if (!signal.aborted) setPending(null);
    }
  }

  const busy = pending ?? status?.busy;
  return (
    <dialog ref={dialog} className="modal snapshot-dialog" aria-labelledby="snapshot-title"
      onCancel={(event) => { event.preventDefault(); onClose(); }}>
      <h2 id="snapshot-title">扫描记录</h2>
      {status?.saved ? (
        <dl className="snapshot-details">
          <dt>范围</dt><dd>{status.saved.rootPath}</dd>
          <dt>扫描时间</dt><dd>{snapshotDate(status.saved.scanTime)}</dd>
          <dt>保存时间</dt><dd>{snapshotDate(status.saved.savedAt)}</dd>
        </dl>
      ) : (
        <p>{status?.savedError || (status ? '暂无已保存的扫描' : error ? '扫描记录暂不可用' : '正在读取扫描记录…')}</p>
      )}
      <p className="snapshot-note">保存会更新最近一份记录。恢复后可浏览当时的结果，清理前会核对当前文件。</p>
      {(error || status?.error) && <p role="alert" className="snapshot-error">{error || status?.error}</p>}
      {busy ? (
        <p role="status">{busy === 'save' ? '正在保存扫描…' : '正在恢复扫描…'}</p>
      ) : status?.message && <p role="status">{status.message}</p>}
      <div className="modal-actions">
        <button type="button" autoFocus onClick={onClose}>关闭</button>
        {error && <button type="button" disabled={Boolean(pending)}
          onClick={() => { setError(''); setRevision((previous) => previous + 1); }}>刷新记录</button>}
        <button type="button" disabled={Boolean(busy) || !status?.saved}
          onClick={() => void run('restore')}>恢复扫描</button>
        <button type="button" disabled={Boolean(busy) || !status?.canSave}
          onClick={() => void run('save')}>保存当前扫描</button>
      </div>
    </dialog>
  );
}
