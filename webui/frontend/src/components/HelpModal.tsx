import { useGduModel } from '../model';
import { Modal } from './Modal';

export function HelpModal() {
  const { showHelp, setShowHelp, view, status } = useGduModel();
  if (!showHelp) {
    return null;
  }

  return (
    <Modal
      titleId="shortcuts-title"
      title="键盘快捷键"
      className="shortcuts-modal"
      onClose={() => setShowHelp(false)}
    >
      <dl className="shortcuts">
        {view === 'treemap' && (
          <>
            <dt>O</dt>
            <dd>在资源管理器中定位所选项目</dd>
            {status.deleteAllowed && (
              <>
                <dt>D / Delete</dt>
                <dd>删除所选项目</dd>
              </>
            )}
            <dt>单击</dt>
            <dd>选择树图中的项目</dd>
            <dt>双击</dt>
            <dd>进入目录</dd>
            <dt>P</dt>
            <dd>
              切换树图着色方式，可按目录、清理候选、修改时间或类别查看
            </dd>
          </>
        )}
        {view === 'donut' && (
          <>
            <dt>行内按钮</dt>
            <dd>{status.deleteAllowed ? '定位或删除列表中的项目' : '在资源管理器中定位列表中的项目'}</dd>
          </>
        )}
        <dt>?</dt>
        <dd>显示快捷键帮助</dd>
        <dt>Esc</dt>
        <dd>关闭对话框，或清除类别筛选</dd>
      </dl>
      <div className="modal-actions">
        <button type="button" autoFocus onClick={() => setShowHelp(false)}>
          关闭
        </button>
      </div>
    </Modal>
  );
}
