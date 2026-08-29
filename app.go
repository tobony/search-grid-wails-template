package main

import (
	"context"
	"encoding/csv"
	"sort"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"search-grid-template/pbi"
	"search-grid-template/store"
)

// App struct
type App struct {
	ctx   context.Context
	store *store.Store
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	s, err := store.OpenDefault()
	if err != nil {
		log.Printf("sqlite open failed: %v", err)
		return
	}
	a.store = s
}

// shutdown closes the cache DB.
func (a *App) shutdown(ctx context.Context) {
	if a.store != nil {
		a.store.Close()
	}
}

// GetDocs returns the locally cached docs (fast, offline-capable).
func (a *App) GetDocs() ([]store.Doc, error) {
	if a.store == nil {
		return nil, fmt.Errorf("cache unavailable")
	}
	return a.store.ListDocs()
}

// SyncResult reports the outcome of a Power BI sync.
// SyncResult2 is the outcome of a sync attempt, with a machine-readable
// error code so the frontend can show actionable guidance.
type SyncResult2 struct {
	OK       bool   `json:"ok"`
	Count    int    `json:"count"`
	SyncedAt string `json:"syncedAt"`
	ErrCode  string `json:"errCode"` // "" | azNotFound | azNotLoggedIn | tokenError | apiError | cacheError
	ErrMsg   string `json:"errMsg"`
	ErrHint  string `json:"errHint"` // command to run / next action
	Message  string `json:"message"` // success message (AddDoc etc.)
}

// CheckEnv reports whether the machine is ready to sync from Power BI
// (Azure CLI installed + logged in). Returns nil-diagnostic equivalent
// when everything is fine.
func (a *App) CheckEnv() SyncResult2 {
	if a.store == nil {
		return SyncResult2{ErrCode: "cacheError", ErrMsg: "로컬 캐시를 사용할 수 없습니다. 앱을 다시 시작해 주세요."}
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	if d := pbi.Diagnose(ctx); d != nil {
		return SyncResult2{ErrCode: d.Code, ErrMsg: d.Title + " " + d.Detail, ErrHint: d.CmdHint}
	}
	return SyncResult2{OK: true}
}

// SyncFromPowerBI pulls the Docs table from the Power BI dataset and
// refreshes the local SQLite cache. Always returns a SyncResult2 (no
// bare error) so the frontend can render guidance per error code.
func (a *App) SyncFromPowerBI() SyncResult2 {
	if a.store == nil {
		return SyncResult2{ErrCode: "cacheError", ErrMsg: "로컬 캐시를 사용할 수 없습니다. 앱을 다시 시작해 주세요."}
	}
	ctx, cancel := context.WithTimeout(a.ctx, 90*time.Second)
	defer cancel()
	if d := pbi.Diagnose(ctx); d != nil {
		return SyncResult2{ErrCode: d.Code, ErrMsg: d.Title, ErrHint: d.CmdHint}
	}
	tok, err := pbi.GetToken(ctx)
	if err != nil {
		return SyncResult2{ErrCode: "tokenError", ErrMsg: "로그인 세션이 만료되었습니다. 다시 로그인해 주세요.", ErrHint: "az login"}
	}
	client := pbi.NewClient(tok)
	// Rows whose Power BI push previously failed go up first; otherwise
	// the cache replace below would leave them stranded local-only.
	pending, err := a.store.PendingDocs()
	if err != nil {
		return SyncResult2{ErrCode: "cacheError", ErrMsg: "재전송 대기 문서 조회 실패: " + err.Error()}
	}
	if len(pending) > 0 {
		pdocs := make([]pbi.Doc, 0, len(pending))
		ids := make([]int64, 0, len(pending))
		for i := range pending {
			d := pending[i]
			pdocs = append(pdocs, pbi.Doc{ID: d.ID, Title: d.Title, Author: d.Author, Category: d.Category, Updated: d.Updated, Status: d.Status})
			ids = append(ids, d.ID)
		}
		if err := client.AddRows(ctx, pdocs); err != nil {
			// Keep the rows flagged; ReplaceDocs preserves pending rows
			// and the next sync retries the push.
			log.Printf("pending re-push failed, %d rows kept flagged: %v", len(pending), err)
		} else if err := a.store.ClearPending(ids); err != nil {
			return SyncResult2{ErrCode: "cacheError", ErrMsg: "재전송 완료 후 상태 갱신 실패: " + err.Error()}
		}
	}
	docs, err := client.QueryDocs(ctx)
	if err != nil {
		switch pbi.StatusCode(err) {
		case 403:
			return SyncResult2{ErrCode: "forbiddenError", ErrMsg: "이 데이터셋에 대한 읽기 권한이 없습니다.", ErrHint: "Power BI 관리자에게 'Power BI Release Plan' 데이터셋 접근 권한을 요청하세요."}
		case 429:
			return SyncResult2{ErrCode: "apiError", ErrMsg: "요청이 많아 잠시 대기가 필요합니다. 잠시 후 다시 시도해 주세요.", ErrHint: "동시 접속자가 많으면 이 메시지가 나올 수 있습니다."}
		}
		return SyncResult2{ErrCode: "apiError", ErrMsg: "Power BI 데이터를 가져오지 못했습니다. " + err.Error(), ErrHint: "네트워크/권한을 확인한 뒤 다시 시도해 주세요."}
	}
	sdocs := make([]store.Doc, len(docs))
	for i, d := range docs {
		sdocs[i] = store.Doc(d)
	}
	if err := a.store.ReplaceDocs(sdocs); err != nil {
		return SyncResult2{ErrCode: "cacheError", ErrMsg: "캐시 저장 실패: " + err.Error()}
	}
	last, err := a.store.LastSync()
	if err != nil {
		last = time.Now().Format(time.RFC3339)
	}
	return SyncResult2{OK: true, Count: len(docs), SyncedAt: last}
}

// AddDoc inserts a new doc into the local cache and pushes it to the
// Power BI dataset. Local insert always wins; Power BI failure is
// reported via ErrHint.
func (a *App) AddDoc(title, author, category, status string) SyncResult2 {
	if strings.TrimSpace(title) == "" {
		return SyncResult2{OK: false, ErrCode: "inputError", ErrMsg: "제목은 필수입니다"}
	}
	if a.store == nil {
		return SyncResult2{OK: false, ErrCode: "cacheError", ErrMsg: "로컬 저장소가 준비되지 않았습니다"}
	}
	ldoc := store.Doc{Title: title, Author: author, Category: category, Updated: time.Now().Format("2006-01-02"), Status: status}
	if err := a.store.InsertDoc(&ldoc); err != nil {
		return SyncResult2{OK: false, ErrCode: "cacheError", ErrMsg: "로컬 저장 실패: " + err.Error()}
	}
	pbiErr := ""
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()
	tok, err := pbi.GetToken(ctx)
	if err != nil {
		pbiErr = err.Error()
	} else {
		client := pbi.NewClient(tok)
		pdoc := pbi.Doc{ID: ldoc.ID, Title: ldoc.Title, Author: ldoc.Author, Category: ldoc.Category, Updated: ldoc.Updated, Status: ldoc.Status}
		if err := client.AddRow(ctx, &pdoc); err != nil {
			pbiErr = err.Error()
		}
	}
	if pbiErr != "" {
		// Flag the row so the next sync re-pushes it instead of
		// dropping it when the cache is replaced from Power BI.
		if err := a.store.MarkPending(ldoc.ID, true); err != nil {
			pbiErr += " (pending 표시 실패: " + err.Error() + ")"
		}
	}
	msg := "새 문서가 저장되었습니다 (id: " + fmt.Sprintf("%d", ldoc.ID) + ")"
	if pbiErr != "" {
		msg += " — Power BI 반영 실패 (다음 동기화 때 자동 재전송됩니다)"
	}
	return SyncResult2{OK: true, Count: 1, Message: msg, ErrHint: pbiErr}
}

// DeleteDoc removes the document from the local cache and re-syncs
// the Power BI dataset (push datasets only allow whole-table clears,
// so a delete means: clear table, push remaining rows).
func (a *App) DeleteDoc(id int64) SyncResult2 {
	if a.store == nil {
		return SyncResult2{OK: false, ErrCode: "cacheError", ErrMsg: "로컬 저장소가 준비되지 않았습니다"}
	}
	if err := a.store.DeleteDoc(id); err != nil {
		return SyncResult2{OK: false, ErrCode: "cacheError", ErrMsg: "로컬 삭제 실패: " + err.Error()}
	}
	pbiErr := ""
	ctx, cancel := context.WithTimeout(a.ctx, 120*time.Second)
	defer cancel()
	tok, err := pbi.GetToken(ctx)
	if err != nil {
		pbiErr = err.Error()
	} else {
		client := pbi.NewClient(tok)
		if err := client.ClearDocs(ctx); err != nil {
			pbiErr = "클리어 실패: " + err.Error()
		} else {
			docs, lerr := a.store.ListDocs()
			if lerr != nil {
				pbiErr = "로컬 조회 실패: " + lerr.Error()
			} else if len(docs) > 0 {
				pdocs := make([]pbi.Doc, 0, len(docs))
				for i := range docs {
					d := docs[i]
					pdocs = append(pdocs, pbi.Doc{ID: d.ID, Title: d.Title, Author: d.Author, Category: d.Category, Updated: d.Updated, Status: d.Status})
				}
				if err := client.AddRows(ctx, pdocs); err != nil {
					pbiErr = "재업로드 실패: " + err.Error()
				}
			}
		}
	}
	msg := "문서가 삭제되었습니다 (id: " + fmt.Sprintf("%d", id) + ")"
	if pbiErr != "" {
		msg += " — Power BI 재동기화 실패: " + pbiErr
	}
	return SyncResult2{OK: true, Count: 1, Message: msg, ErrHint: pbiErr}
}
// ExportCSV는 검색·정렬이 적용된 문서 전체를 CSV 파일로 저장한다 (UTF-8 BOM).
// path가 빈 문자열이면 사용자가 저장 대화상자를 취소한 것이다.
func (a *App) ExportCSV(query, sortKey string, sortAsc bool) SyncResult2 {
	if a.store == nil {
		return SyncResult2{OK: false, ErrCode: "cacheError", ErrMsg: "로컬 캐시를 사용할 수 없습니다"}
	}
	docs, err := a.store.ListDocs()
	if err != nil {
		return SyncResult2{OK: false, ErrCode: "cacheError", ErrMsg: "데이터 조회 실패: " + err.Error()}
	}

	// 프론트엔드와 동일한 필터·정렬 로직
	q := strings.ToLower(strings.TrimSpace(query))
	filtered := make([]store.Doc, 0, len(docs))
	for _, d := range docs {
		// 검색 범위는 화면과 동일하게 제목/작성자/분류만 (상태 제외)
		hay := strings.ToLower(d.Title + " " + d.Author + " " + d.Category)
		if q == "" || strings.Contains(hay, q) {
			filtered = append(filtered, d)
		}
	}
	sortDocsByKey(filtered, sortKey, sortAsc)

	path, err := wruntime.SaveFileDialog(a.ctx, wruntime.SaveDialogOptions{
		DefaultFilename: "문서목록.csv",
		Title:           "CSV 내보내기",
	})
	if err != nil {
		return SyncResult2{OK: false, ErrCode: "apiError", ErrMsg: "파일 대화상자 오류: " + err.Error()}
	}
	if path == "" {
		return SyncResult2{OK: true, Count: 0, Message: ""} // 취소
	}
	if !strings.HasSuffix(strings.ToLower(path), ".csv") {
		path += ".csv"
	}

	f, err := os.Create(path)
	if err != nil {
		return SyncResult2{OK: false, ErrCode: "apiError", ErrMsg: "파일 생성 실패: " + err.Error()}
	}
	defer f.Close()

	// UTF-8 BOM: 엑셀에서 한글이 깨지지 않도록
	f.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(f)
	w.Write([]string{"ID", "제목", "작성자", "분류", "수정일", "상태"})
	for _, d := range filtered {
		w.Write([]string{fmt.Sprintf("%d", d.ID), csvSafe(d.Title), csvSafe(d.Author), csvSafe(d.Category), csvSafe(d.Updated), csvSafe(d.Status)})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return SyncResult2{OK: false, ErrCode: "apiError", ErrMsg: "CSV 쓰기 실패: " + err.Error()}
	}
	return SyncResult2{OK: true, Count: len(filtered), Message: fmt.Sprintf("%d건을 저장했습니다: %s", len(filtered), path)}
}

// csvSafe는 CSV 수식 인젝션 방어: 값이 =, +, -, @, 탭, 캐리지리턴으로
// 시작하면 엑셀이 수식으로 해석하지 못하도록 작은따옴표를 접두한다
// (OWASP 권장 방식).
func csvSafe(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

// sortDocsByKey는 프론트엔드 정렬과 동일한 기준(오름차순, !asc면 반전)으로 정렬한다.
func sortDocsByKey(docs []store.Doc, key string, asc bool) {
	sort.Slice(docs, func(i, j int) bool {
		li, lj := docs[i], docs[j]
		var cmp bool
		switch key {
		case "title":
			cmp = li.Title < lj.Title
		case "author":
			cmp = li.Author < lj.Author
		case "category":
			cmp = li.Category < lj.Category
		case "status":
			cmp = li.Status < lj.Status
		default: // updated
			cmp = li.Updated < lj.Updated
		}
		if !asc {
			return !cmp && li.Updated != lj.Updated || key != "updated" && !cmp && docKey(li, key) != docKey(lj, key)
		}
		return cmp
	})
}

func docKey(d store.Doc, key string) string {
	switch key {
	case "title":
		return d.Title
	case "author":
		return d.Author
	case "category":
		return d.Category
	case "status":
		return d.Status
	}
	return d.Updated
}
