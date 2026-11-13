import { GduModelProvider, useGduModelState } from './model';
import { Header } from './components/Header';
import { DonutView } from './components/DonutView';
import { FileTableView } from './components/FileTableView';
import { TreeMapView } from './components/TreeMapView';
import { HelpModal } from './components/HelpModal';
import { ProgressBar } from './components/ProgressBar';
import { ScanControls } from './components/ScanControls';
import { SnapshotControls } from './components/SnapshotControls';
import { localizeError } from './zh-CN';
import { OverviewView } from './overview/OverviewView';
import type { ChartView } from './model';

export function App({ initialView }: { initialView?: ChartView }) {
  const model = useGduModelState(initialView);
  const { status, currentPath, useSIPrefix, loadError, view, actionToken } = model;

  if (!status) {
    return <div className="loading">正在连接…</div>;
  }

  if (status.state === 'error') {
    return (
      <div className="error-screen">
        <h2>扫描失败</h2>
        <p>{status.error ? localizeError(status.error) : '未知错误'}</p>
        {status.scanAllowed && <ScanControls status={status} token={actionToken} currentPath={status.rootPath} />}
        {status.snapshotAllowed && <SnapshotControls token={actionToken} scanState={status.state} />}
      </div>
    );
  }

  if (status.state === 'scanning' || currentPath === null) {
    return (
      <div className="app">
        {status.scanAllowed && status.scanTarget && (
          <header className="app-header">
            <span className="logo">gdu</span>
            <ScanControls status={status} token={actionToken} currentPath={status.rootPath} />
          </header>
        )}
        {status.snapshotBusy === 'restore' ? (
          <div className="loading" role="status">正在恢复保存的扫描…</div>
        ) : <ProgressBar progress={status.progress} useSIPrefix={useSIPrefix} target={status.scanTarget} />}
      </div>
    );
  }

  return (
    <GduModelProvider model={{ ...model, status, currentPath }}>
      <div className="app">
        <Header />
        {status.scanTime && (
          <div className="scan-snapshot-time" role="status">
            {status.restoredSnapshot ? '已恢复保存的扫描' : '扫描完成'} · {new Date(status.scanTime).toLocaleString('zh-CN')}
            {status.restoredSnapshot && ' · 文件可能已有变化，清理前会重新核对'}
          </div>
        )}

        {loadError && <div className="banner error">{loadError}</div>}

        {view === 'overview' ? <OverviewView /> : (
          <main className={`content ${view === 'treemap' ? 'treemap-view' : ''}`}>
            {view === 'donut' ? (
              <>
                <DonutView />
                <FileTableView />
              </>
            ) : <TreeMapView />}
          </main>
        )}

        <HelpModal />
      </div>
    </GduModelProvider>
  );
}
