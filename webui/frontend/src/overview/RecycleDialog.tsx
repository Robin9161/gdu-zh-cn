import { useEffect, useRef, useState } from 'react';
import { formatCount } from '../format';
import { formatOverviewSize } from './formatUsage';
import { confirmRecycle, previewRecycle, type RecyclePlan, type RecycleResult } from '../api';
import { SafetyDetail } from './SafetyDetail';
import { OwnerAdvice } from './OwnerGroups';
import type { OverviewFile } from './api';

interface Props {
  paths: string[];
  checked: ReadonlyMap<string, OverviewFile>;
  actionToken: string;
  useSIPrefix: boolean;
  onClose: () => void;
  onResult: (results: RecycleResult[]) => void;
}

export function RecycleDialog({ paths, checked, actionToken, useSIPrefix, onClose, onResult }: Props) {
  const dialog = useRef<HTMLDialogElement>(null);
  const submitting = useRef(false);
  const [plan, setPlan] = useState<RecyclePlan | null>(null);
  const [results, setResults] = useState<RecycleResult[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    const element = dialog.current;
    const trigger = document.activeElement;
    element?.showModal();
    return () => {
      element?.close();
      if (trigger instanceof HTMLElement && trigger.isConnected) trigger.focus();
    };
  }, []);

  useEffect(() => {
    const request = new AbortController();
    const timeout = window.setTimeout(() => {
      request.abort();
      setError('核对清单超时，请重试。');
    }, 45_000);
    previewRecycle(paths, actionToken, request.signal)
      .then((data) => { if (!request.signal.aborted) setPlan(data); })
      .catch((failure: unknown) => {
        if (!request.signal.aborted) setError(failure instanceof Error ? failure.message : '无法核对清单');
      })
      .finally(() => window.clearTimeout(timeout));
    return () => {
      request.abort();
      window.clearTimeout(timeout);
    };
  }, [paths, actionToken, attempt]);

  async function confirm() {
    if (!plan || submitting.current) return;
    submitting.current = true;
    setBusy(true);
    setError(null);
    try {
      const report = await confirmRecycle(plan.id, actionToken);
      setResults(report);
      onResult(report);
    } catch (failure: unknown) {
      const detail = failure instanceof Error ? failure.message : '回收操作未返回结果';
      setError(`${detail}。请核对回收站后再重新确认。`);
      setPlan(null);
      onResult([]);
    } finally {
      submitting.current = false;
      setBusy(false);
    }
  }

  const valid = plan?.files.filter((file) => !file.error).length ?? 0;
  const reportByPath = new Map(results?.map((item) => [item.path, item]));
  const successful = results?.filter((item) => !item.error).length ?? 0;

  return (
    <dialog ref={dialog} className="modal recycle-dialog" aria-labelledby="recycle-title"
      onCancel={(event) => { event.preventDefault(); if (!submitting.current) onClose(); }}>
      <h2 id="recycle-title">{results ? '清理结果' : '待清理清单'}</h2>
      {results ? (
        <p role="status">已移入回收站 {formatCount(successful)} 个文件。未移入 {formatCount(results.length - successful)} 个。</p>
      ) : plan ? (
        <>
          <p><strong>{formatCount(valid)} 个可移入文件</strong> · 合计占用 <strong>{formatOverviewSize(plan.usage, useSIPrefix)}</strong></p>
          <p className="overview-hint">移入后可在 Windows 回收站恢复。清空回收站后才会释放空间。</p>
        </>
      ) : !error && <p role="status">正在核对 {formatCount(paths.length)} 个文件…</p>}
      {plan && (
        <ul className="recycle-items" tabIndex={0} aria-label="待清理文件">
          {plan.files.map((file) => {
            const result = reportByPath.get(file.path);
            const reason = result?.error ?? file.error;
            const scanned = checked.get(file.path);
            const owner = file.owner ?? (scanned?.grade === file.grade ? scanned.owner : undefined);
            return (
              <li key={file.path}>
                <div><strong>{file.name}</strong><span>{reason ? '未移入' : result ? '已移入' : formatOverviewSize(file.usage, useSIPrefix)}</span></div>
                <small>{file.path}</small>
                <SafetyDetail grade={file.grade} reason={file.reason} />
                {!results && !reason && owner && (
                  <details className="owner-details">
                    <summary>归属 · {owner.name}</summary>
                    <OwnerAdvice owner={owner} />
                  </details>
                )}
                {reason && <p className="recycle-error">{reason}</p>}
              </li>
            );
          })}
        </ul>
      )}
      {busy && <p role="status">正在移入 Windows 回收站…</p>}
      {error && <p className="recycle-error" role="alert">{error}</p>}
      <div className="modal-actions">
        <button type="button" autoFocus disabled={busy} onClick={onClose}>{results ? '完成' : '返回'}</button>
        {error && !results && (
          <button type="button" onClick={() => { setError(null); setAttempt((value) => value + 1); }}>重新核对</button>
        )}
        {plan && !results && (
          <button type="button" className="recycle-confirm" disabled={busy || valid === 0} onClick={() => void confirm()}>
            确认移入回收站
          </button>
        )}
      </div>
    </dialog>
  );
}
