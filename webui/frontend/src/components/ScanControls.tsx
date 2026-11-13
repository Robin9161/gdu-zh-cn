import { useEffect, useRef, useState } from 'react';
import { cancelScan, fetchScanDrives, fetchScanFolders, startScan, type ScanFolders } from '../api';
import type { Status } from '../types';
import './scan.css';

interface ScanControlsProps {
  status: Status;
  token: string;
  currentPath: string;
}

export function ScanControls({ status, token, currentPath }: ScanControlsProps) {
  const [open, setOpen] = useState(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function requestCancel() {
    setBusy(true);
    setError('');
    try {
      await cancelScan(token);
    } catch (err) {
      setError(err instanceof Error ? err.message : '取消扫描失败，请重试');
    } finally {
      setBusy(false);
    }
  }

  if (status.state === 'scanning') {
    return (
      <div className="scan-controls">
        <button type="button" className="toggle" disabled={!token || busy || status.scanCancelling || !!status.snapshotBusy}
          onClick={() => void requestCancel()}>{status.scanCancelling ? '正在取消…' : '取消扫描'}</button>
        {error && <span role="alert">{error}</span>}
      </div>
    );
  }

  return (
    <>
      <button type="button" className="toggle" disabled={!token || !!status.snapshotBusy}
        title={token ? '选择盘符或文件夹并重新扫描' : '请通过 Gdu 的本地启动地址启用扫描'}
        onClick={() => setOpen(true)}>扫描范围</button>
      {open && <ScanDialog token={token} currentPath={currentPath} onClose={() => setOpen(false)} />}
    </>
  );
}

function ScanDialog({ token, currentPath, onClose }: { token: string; currentPath: string; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [drives, setDrives] = useState<{ path: string }[] | null>(null);
  const [drivesError, setDrivesError] = useState('');
  const [path, setPath] = useState(currentPath);
  const [browsePath, setBrowsePath] = useState<string | null>(null);
  const [folders, setFolders] = useState<ScanFolders | null>(null);
  const [browseError, setBrowseError] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    const element = dialog.current;
    element?.showModal();
    return () => element?.close();
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    fetchScanDrives(token, controller.signal).then(setDrives).catch((err: unknown) => {
      if (!controller.signal.aborted) {
        setDrivesError(err instanceof Error ? err.message : '无法读取盘符');
      }
    });
    return () => controller.abort();
  }, [token]);

  useEffect(() => {
    if (browsePath === null) return;
    const controller = new AbortController();
    fetchScanFolders(browsePath, token, controller.signal).then((result) => {
      if (!controller.signal.aborted) {
        setFolders(result);
        setPath(result.path);
      }
    }).catch((err: unknown) => {
      if (!controller.signal.aborted) {
        setBrowseError(err instanceof Error ? err.message : '无法浏览此文件夹');
      }
    });
    return () => controller.abort();
  }, [browsePath, token]);

  function browse(next: string) {
    setFolders(null);
    setBrowseError('');
    setError('');
    setBrowsePath(next);
  }

  function clearBrowse() {
    setBrowsePath(null);
    setFolders(null);
    setBrowseError('');
    setError('');
  }

  async function submit() {
    setBusy(true);
    setError('');
    try {
      await startScan(path, token);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : '无法开始扫描，请重试');
    } finally {
      setBusy(false);
    }
  }

  return (
    <dialog ref={dialog} className="modal scan-dialog" aria-labelledby="scan-title"
      onCancel={(event) => { event.preventDefault(); if (!busy) onClose(); }}>
      <h2 id="scan-title">选择扫描范围</h2>
      <form onSubmit={(event) => { event.preventDefault(); if (!busy && path.trim()) void submit(); }}>
        <div className="scan-drive-picker" role="group" aria-label="本机盘符">
          {drives?.map((drive) => (
            <button key={drive.path} type="button" disabled={busy}
              aria-pressed={path.toUpperCase() === drive.path.toUpperCase()}
              onClick={() => {
                setPath(drive.path);
                if (browsePath !== null && browsePath !== drive.path) browse(drive.path);
                else if (browsePath === null) clearBrowse();
              }}>{drive.path}</button>
          ))}
          {drives === null && !drivesError && <span role="status">正在读取盘符…</span>}
          {drivesError && <span>{drivesError}</span>}
        </div>
        <label htmlFor="scan-path">文件夹路径</label>
        <div className="scan-path-entry">
          <input id="scan-path" value={path} disabled={busy} autoFocus spellCheck={false} autoComplete="off"
            onChange={(event) => { setPath(event.target.value); clearBrowse(); }} />
          <button type="button" disabled={busy || !path.trim() || path === browsePath}
            onClick={() => browse(path)}>浏览文件夹</button>
        </div>
        {browsePath === null && <p>也可从资源管理器地址栏复制路径。</p>}
        {browsePath !== null && !folders && !browseError && <p role="status">正在读取文件夹…</p>}
        {browseError && <p role="alert" className="scan-error">{browseError}</p>}
        {folders && (
          <div className="scan-folder-list" role="group" aria-label="子文件夹">
            {folders.parent && <button type="button" disabled={busy} onClick={() => browse(folders.parent)}>上一级</button>}
            {folders.folders.map((name) => (
              <button type="button" key={name} disabled={busy} onClick={() => browse(`${folders.path.replace(/\\$/, '')}\\${name}`)}>{name}</button>
            ))}
            {folders.folders.length === 0 && <p>没有子文件夹</p>}
          </div>
        )}
        {error && <p role="alert" className="scan-error">{error}</p>}
        <div className="modal-actions">
          <button type="button" onClick={onClose} disabled={busy}>返回</button>
          <button type="submit" disabled={busy || !path.trim() || (browsePath !== null && !folders && !browseError)}>{busy ? '正在启动…' : '开始扫描'}</button>
        </div>
      </form>
    </dialog>
  );
}
