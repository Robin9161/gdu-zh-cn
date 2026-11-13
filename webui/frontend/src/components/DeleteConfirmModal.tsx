import { useGduModel } from '../model';
import { formatSize } from '../format';
import type { ItemActions } from '../useItemActions';
import { Modal } from './Modal';

// Confirmation dialog for the delete workflow in useItemActions. Renders
// nothing until a delete has been requested.
export function DeleteConfirmModal({ actions }: { actions: ItemActions }) {
  const { effectiveApparent, useSIPrefix, setSkipDeleteConfirm } = useGduModel();
  const {
    deleteCandidate,
    skipDeleteChoice,
    setSkipDeleteChoice,
    actionPending,
    cancelDelete,
    performDelete,
  } = actions;

  if (!deleteCandidate) {
    return null;
  }

  return (
    <Modal titleId="delete-title" title="删除项目" onClose={cancelDelete}>
      <p>选择将此项目移入回收站，或立即永久删除。</p>
      <code className="modal-path">{deleteCandidate.path}</code>
      <span className="muted">
        {formatSize(effectiveApparent ? deleteCandidate.size : deleteCandidate.usage, useSIPrefix)}
      </span>
      <label className="modal-checkbox">
        <input
          type="checkbox"
          checked={skipDeleteChoice}
          onChange={(event) => setSkipDeleteChoice(event.target.checked)}
        />
        本次会话不再确认
      </label>
      <div className="modal-actions">
        <button type="button" autoFocus onClick={cancelDelete}>
          取消
        </button>
        <button
          type="button"
          disabled={actionPending}
          onClick={() => {
            if (skipDeleteChoice) {
              setSkipDeleteConfirm('trash');
            }
            void performDelete(deleteCandidate, 'trash');
          }}
        >
          移入回收站
        </button>
        <button
          type="button"
          className="danger"
          disabled={actionPending}
          onClick={() => {
            if (skipDeleteChoice) {
              setSkipDeleteConfirm('permanent');
            }
            void performDelete(deleteCandidate, 'permanent');
          }}
        >
          永久删除
        </button>
      </div>
    </Modal>
  );
}
