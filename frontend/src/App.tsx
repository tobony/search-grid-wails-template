import { useCallback, useEffect, useMemo, useState } from 'react';
import './App.css';
import { GetDocs, SyncFromPowerBI, CheckEnv , ExportCSV } from '../wailsjs/go/main/App';
import { SetupGuideDialog, type SyncResult } from './SetupGuide';
import { NewDocDialog } from './NewDocDialog';
import { DeleteConfirmDialog } from './DeleteConfirmDialog';
import { DeleteDoc } from '../wailsjs/go/main/App';

interface Doc {
  id: number;
  title: string;
  author: string;
  category: string;
  updated: string;
  status: string;
}



const PAGE_SIZE = 8;
type SortKey = 'title' | 'author' | 'category' | 'updated' | 'status';

// 상태값 → 배지 매핑. NewDocDialog의 '계획/진행/완료/보류'와
// Power BI 쪽 'published/draft/archived'(한국어 라벨 포함)를 모두 지원.
const STATUS_BADGES: Record<string, { label: string; cls: string }> = {
  'published': { label: '게시됨', cls: 'published' },
  '게시됨': { label: '게시됨', cls: 'published' },
  'draft': { label: '초안', cls: 'draft' },
  '초안': { label: '초안', cls: 'draft' },
  'archived': { label: '보관됨', cls: 'archived' },
  '보관됨': { label: '보관됨', cls: 'archived' },
  '계획': { label: '계획', cls: 'plan' },
  '진행': { label: '진행', cls: 'progress' },
  '완료': { label: '완료', cls: 'done' },
  '보류': { label: '보류', cls: 'hold' },
};

// statusBadge는 상태 문자열에 대응하는 배지 라벨/클래스를 반환한다.
// 알 수 없는 값은 회색 톤으로 원본 문자열을 그대로 보여준다.
function statusBadge(status: string): { label: string; cls: string } {
  return STATUS_BADGES[status] ?? { label: status || '-', cls: 'archived' };
}

function App() {
  const [query, setQuery] = useState('');
  const [exporting, setExporting] = useState(false);
  const [sortKey, setSortKey] = useState<SortKey>('updated');
  const [sortAsc, setSortAsc] = useState(false);
  const [page, setPage] = useState(1);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [docs, setDocs] = useState<Doc[]>([]);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState(false);
  const [syncMsg, setSyncMsg] = useState<string | null>(null);
  const [syncMsgIsErr, setSyncMsgIsErr] = useState(false);
  const [setup, setSetup] = useState<SyncResult | null>(null);
  const [env, setEnv] = useState<SyncResult | null>(null);
  const [showNewDoc, setShowNewDoc] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Doc | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [deleteErr, setDeleteErr] = useState('');

  const envBannerText = env?.errCode === 'azNotFound'
    ? 'Power BI 동기화를 사용하려면 Azure CLI 설치가 필요합니다.'
    : 'Power BI 동기화를 사용하려면 로그인이 필요합니다.';

  // showMsg는 상태바 메시지를 갱신한다. 성공 메시지는 5초 후 자동으로
  // 사라지고, 실패 메시지는 사용자가 확인할 수 있도록 유지된다.
  const showMsg = useCallback((text: string | null, isErr = false) => {
    setSyncMsg(text);
    setSyncMsgIsErr(isErr);
  }, []);

  // 성공 메시지 자동 소멸 타이머. 실패 메시지는 타이머를 걸지 않는다.
  useEffect(() => {
    if (!syncMsg || syncMsgIsErr) return;
    const timer = setTimeout(() => setSyncMsg(null), 5000);
    return () => clearTimeout(timer);
  }, [syncMsg, syncMsgIsErr]);

  const refresh = useCallback(async () => {
    try {
      const rows = await GetDocs();
      setDocs(rows ?? []);
    } catch (err) {
      console.error('GetDocs failed:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  const handleSync = useCallback(async () => {
    setSyncing(true);
    showMsg(null);
    try {
      const r: SyncResult = await SyncFromPowerBI();
      if (r.ok) {
        // 0건이면 "완료"가 아니라 안내 문구로 표시한다.
        showMsg(r.count === 0 ? '가져올 데이터가 없습니다' : `동기화 완료: ${r.count}건 · ${r.syncedAt}`);
        const rows = await GetDocs();
        setDocs(rows ?? []);
      } else {
        showMsg(null);
        setSetup(r);
      }
    } catch (e) {
      showMsg(null);
      setSetup({ ok: false, errCode: 'apiError', errMsg: String(e) });
    } finally {
      setSyncing(false);
    }
  }, [showMsg]);

  // 시작 시 1회: 캐시를 먼저 읽고(오프라인 우선 — 캐시가 있으면 네트워크를
  // 쓰지 않는다), 캐시가 비어 있을 때만 Power BI 자동 동기화를 시도한다.
  // az 미설치/미로그인이면 동기화 대신 배너만 띄워 시작을 막지 않는다.
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const rows = await GetDocs().catch(() => null);
      if (cancelled) return;
      setDocs(rows ?? []);
      setLoading(false);
      if ((rows ?? []).length > 0) {
        void CheckEnv().then((r) => {
          if (!cancelled && !r.ok) setEnv(r);
        });
        return;
      }
      const envr = await CheckEnv();
      if (cancelled) return;
      if (!envr.ok) {
        setEnv(envr);
        return;
      }
      await handleSync();
    })();
    return () => {
      cancelled = true;
    };
  }, [handleSync]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    let rows = docs.filter(
      (d) => !q || d.title.toLowerCase().includes(q) || d.author.toLowerCase().includes(q) || d.category.toLowerCase().includes(q),
    );
    rows = [...rows].sort((a, b) => {
      const av = a[sortKey];
      const bv = b[sortKey];
      const cmp = typeof av === 'number' && typeof bv === 'number' ? av - bv : String(av).localeCompare(String(bv), 'ko');
      return sortAsc ? cmp : -cmp;
    });
    return rows;
  }, [docs, query, sortKey, sortAsc]);

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));

  async function exportCsv() {
    if (exporting) return;
    setExporting(true);
    try {
      const r = await ExportCSV(query, sortKey, sortAsc);
      if (r.count === 0 && !r.errMsg) return; // 취소 무음
      showMsg(r.ok ? r.message : ('내보내기 실패: ' + r.errMsg), !r.ok);
    } catch (e: any) {
      showMsg('내보내기 실패: ' + String(e?.message ?? e), true);
    } finally {
      setExporting(false);
    }
  }
  const safePage = Math.min(page, totalPages);
  const pageRows = filtered.slice((safePage - 1) * PAGE_SIZE, safePage * PAGE_SIZE);

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) setSortAsc(!sortAsc);
    else {
      setSortKey(key);
      setSortAsc(true);
    }
  };

  const arrow = (key: SortKey) => (sortKey === key ? (sortAsc ? ' ▲' : ' ▼') : '');

  return (
    <div className="app">
      {setup && (
        <SetupGuideDialog code={setup.errCode || 'apiError'} msg={setup.errMsg} hint={setup.errHint} onClose={() => setSetup(null)} />
      )}
      {env && (
        <div className="envbanner">
          <span>⚠️ {envBannerText}</span>
          <button className="linklike" onClick={() => {
            void CheckEnv().then((r) => {
              if (r.ok) {
                setEnv(null);
                setSetup({ ok: true, errCode: 'envOk', errMsg: '', errHint: '' });
              } else {
                setEnv(r); setSetup(r);
              }
            }).catch(() => {
              setSetup({ ok: false, errCode: 'apiError', errMsg: '환경 진단에 실패했습니다.', errHint: '잠시 후 다시 시도해 주세요.' });
            });
          }}>설정 안내 보기</button>
        </div>
      )}
      <div className="commandbar">
        <button className="cmdbtn primary" disabled={syncing} onClick={() => setShowNewDoc(true)}>
          <span className="cmdbtn-icon">＋</span> 새 문서
        </button>
        <div className="cmdbar-sep" />
        <button className="cmdbtn" disabled={syncing} onClick={() => { void handleSync(); }}>
          <span className="cmdbtn-icon">⟳</span> {syncing ? '동기화 중…' : 'Power BI 동기화'}
        </button>
        <button onClick={exportCsv} disabled={exporting} className="cmdbtn">
          <span className="cmdbtn-icon">⭳</span> 내보내기
        </button>
      </div>

      <div className="content">
        <div className="searchbox-row">
          <div className="searchbox">
            <span className="search-icon">🔍</span>
            <input
              placeholder="제목, 작성자, 분류로 검색…"
              value={query}
              onChange={(e) => {
                setQuery(e.target.value);
                setPage(1);
              }}
            />
          </div>
          <span className="result-count">
            {loading ? '불러오는 중…' : `${filtered.length}개 결과`}
            {syncMsg ? ` · ${syncMsg}` : ''}
          </span>
        </div>

        <div className="grid-card">
          <table className="grid">
            <thead>
              <tr>
                <th onClick={() => toggleSort('title')}>제목{arrow('title')}</th>
                <th onClick={() => toggleSort('author')}>작성자{arrow('author')}</th>
                <th onClick={() => toggleSort('category')}>분류{arrow('category')}</th>
                <th onClick={() => toggleSort('updated')}>수정일{arrow('updated')}</th>
                <th onClick={() => toggleSort('status')}>상태{arrow('status')}</th>
                <th className="th-actions">관리</th>
              </tr>
            </thead>
            <tbody>
              {pageRows.length === 0 ? (
                <tr>
                  <td colSpan={6}>
                    <div className="empty">
                      <div className="empty-icon">📄</div>
                      <div className="empty-title">결과가 없습니다</div>
                      <div className="empty-sub">{loading ? '불러오는 중…' : 'Power BI 동기화 후 다시 시도해 보세요.'}</div>
                    </div>
                  </td>
                </tr>
              ) : (
                pageRows.map((d) => (
                  <tr
                    key={d.id}
                    className={selectedId === d.id ? 'selected' : ''}
                    onClick={() => setSelectedId(d.id)}
                  >
                    <td className="cell-title">{d.title}</td>
                    <td>{d.author}</td>
                    <td>{d.category}</td>
                    <td>{d.updated}</td>
                    <td>
                      <span className={`badge ${statusBadge(d.status).cls}`}>
                        {statusBadge(d.status).label}
                      </span>
                    </td>
                    <td className="cell-actions" onClick={(e) => e.stopPropagation()}>
                      <button
                        className="del-btn"
                        title="문서 삭제"
                        disabled={syncing}
                        onClick={() => setDeleteTarget(d)}
                      >
                        🗑
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>

          <div className="grid-footer">
            <span>
              전체 {filtered.length}건 중 {(safePage - 1) * PAGE_SIZE + 1}–{Math.min(safePage * PAGE_SIZE, filtered.length)} 표시
            </span>
            <div className="pager">
              <button disabled={safePage === 1} onClick={() => setPage(safePage - 1)}>‹</button>
              {Array.from({ length: totalPages }, (_, i) => (
                <button
                  key={i}
                  className={safePage === i + 1 ? 'active' : ''}
                  onClick={() => setPage(i + 1)}
                >
                  {i + 1}
                </button>
              ))}
              <button disabled={safePage === totalPages} onClick={() => setPage(safePage + 1)}>›</button>
            </div>
          </div>
        </div>
      </div>
      {showNewDoc && (
        <NewDocDialog
          onClose={() => setShowNewDoc(false)}
          onSaved={(msg, isErr) => { void refresh(); showMsg(msg, isErr); }}
        />
      )}
      {deleteTarget && (
        <DeleteConfirmDialog
          doc={deleteTarget}
          busy={deleting}
          error={deleteErr}
          onClose={() => { setDeleteErr(''); setDeleteTarget(null); }}
          onConfirm={async () => {
            if (!deleteTarget) return;
            setDeleting(true);
            setDeleteErr('');
            try {
              const r = await DeleteDoc(deleteTarget.id);
              if (!r.ok) {
                // 삭제 실패: 그리드를 갱신하지 않고 다이얼로그에 사유를 표시한다.
                setDeleteErr(r.errMsg || '삭제에 실패했습니다. 잠시 후 다시 시도해 주세요.');
                return;
              }
              showMsg(r.message + (r.errHint ? ' (Power BI 재동기화 실패)' : ''), !!r.errHint);
              await refresh();
              setDeleteTarget(null);
            } catch (e: any) {
              setDeleteErr('삭제에 실패했습니다: ' + String(e?.message ?? e));
            } finally {
              setDeleting(false);
            }
          }}
        />
      )}
    </div>
  );
}

export default App;
