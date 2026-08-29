interface Doc {
  id: number;
  title: string;
}

interface Props {
  doc: Doc;
  busy: boolean;
  error?: string; // 삭제 실패 사유 (있으면 다이얼로그 안에 표시)
  onClose: () => void;
  onConfirm: () => void;
}

export function DeleteConfirmDialog({ doc, busy, error, onClose, onConfirm }: Props) {
  return (
    <div className="dialog-backdrop" onClick={busy ? undefined : onClose}>
      <div className="dialog" onClick={(e) => e.stopPropagation()}>
        <h2>문서 삭제</h2>
        <p className="delete-msg">
          <strong>{doc.title}</strong> 문서를 삭제할까요?
        </p>
        <p className="delete-note">
          로컬 DB에서 삭제되고, Power BI 데이터셋에도 반영됩니다. 이 작업은 되돌릴 수 없습니다.
        </p>
        {error && <div className="form-err">{error}</div>}
        <div className="form-actions">
          <button className="btn-secondary" onClick={onClose} disabled={busy}>
            취소
          </button>
          <button className="btn-danger" onClick={onConfirm} disabled={busy}>
            {busy ? '삭제 중…' : '삭제'}
          </button>
        </div>
      </div>
    </div>
  );
}
