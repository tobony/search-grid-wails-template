import { useState } from 'react';
import { AddDoc } from '../wailsjs/go/main/App';

export function NewDocDialog({ onClose, onSaved }: {
  onClose: () => void;
  onSaved: (msg: string, isErr: boolean) => void;
}) {
  const [title, setTitle] = useState('');
  const [author, setAuthor] = useState('');
  const [category, setCategory] = useState('정책');
  const [status, setStatus] = useState('진행');
  const [saving, setSaving] = useState(false);
  const [err, setErr] = useState('');

  async function save() {
    if (!title.trim()) { setErr('제목을 입력하세요'); return; }
    setSaving(true); setErr('');
    try {
      const r = await AddDoc(title, author, category, status);
      if (!r.ok) {
        // 저장 실패: 다이얼로그를 유지하고 실패 사유를 표시한다.
        setErr(r.errMsg || '저장에 실패했습니다. 잠시 후 다시 시도해 주세요.');
        return;
      }
      onSaved(r.message ?? '저장되었습니다', !!r.errHint);
      onClose();
    } catch (e: any) {
      setErr('저장에 실패했습니다: ' + String(e?.message ?? e));
    } finally {
      setSaving(false);
    }
  }

  return (
    // 저장 중 backdrop 클릭으로 다이얼로그가 닫히면 응답을 놓치게 되므로 차단한다.
    <div className="dialog-backdrop" onClick={() => { if (!saving) onClose(); }}>
      <div className="dialog" onClick={(e) => e.stopPropagation()}>
        <div className="dialog-title">새 문서 추가</div>
        <div className="form-row"><label>제목 *</label>
          <input value={title} onChange={(e) => setTitle(e.target.value)} placeholder="문서 제목을 입력하세요" /></div>
        <div className="form-row"><label>작성자</label>
          <input value={author} onChange={(e) => setAuthor(e.target.value)} /></div>
        <div className="form-row"><label>카테고리</label>
          <select value={category} onChange={(e) => setCategory(e.target.value)}>
            <option>정책</option><option>가이드</option><option>보고서</option><option>기타</option>
          </select></div>
        <div className="form-row"><label>상태</label>
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option>계획</option><option>진행</option><option>완료</option><option>보류</option>
          </select></div>
        {err && <div className="form-err">{err}</div>}
        <div className="form-actions">
          <button className="btn-secondary" onClick={onClose} disabled={saving}>취소</button>
          <button className="btn-primary" onClick={save} disabled={saving || !title.trim()}>
            {saving ? '저장 중…' : '저장'}
          </button>
        </div>
        <p className="form-note">저장하면 로컬 DB와 Power BI 데이터셋 양쪽에 반영됩니다.</p>
      </div>
    </div>
  );
}
