// Package pbi talks to the Power BI REST API using a token acquired
// through the Azure CLI (az account get-access-token). No MSAL SDK,
// no client secrets: the user logs in once with `az login` and this
// package borrows that token.
package pbi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	apiBase  = "https://api.powerbi.com/v1.0/myorg"
	resource = "https://analysis.windows.net/powerbi/api"
)

// datasetID returns the Power BI push dataset ID.
// Configure via the SEARCH_GRID_DATASET_ID environment variable (or see
// AGENTS.private.example.md). The default is empty: the app shows a setup
// guide instead of silently pointing at a workspace.
func datasetID() string {
	return os.Getenv("SEARCH_GRID_DATASET_ID")
}

// Doc mirrors the Docs table in the dataset (and frontend Doc type).
type Doc struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Author   string `json:"author"`
	Category string `json:"category"`
	Updated  string `json:"updated"`
	Status   string `json:"status"`
}

// Token holds a Power BI access token and its expiry.
type Token struct {
	Value     string
	ExpiresAt time.Time
}

// GetToken acquires a Power BI access token via the Azure CLI.
// `az login` must have been performed once by the user beforehand.
func GetToken(ctx context.Context) (*Token, error) {
	c, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(c, "az", "account", "get-access-token",
		"--resource", resource,
		"--output", "tsv",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("az account get-access-token failed: %w (did you run `az login --use-device-code`?)", err)
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 2 {
		return nil, fmt.Errorf("unexpected az output (%d fields)", len(fields))
	}
	// tsv: <token> <expiresOn...> — token is the first field
	return &Token{Value: fields[0], ExpiresAt: time.Now().Add(55 * time.Minute)}, nil
}

// Client is a minimal Power BI REST client.
type Client struct {
	token *Token
	http  *httpClient
}

// NewClient returns a client using the given token.
func NewClient(t *Token) *Client {
	return &Client{token: t, http: &httpClient{}}
}

// newAPIError builds an apiError from an HTTP response, parsing
// Retry-After (seconds) when present.
func newAPIError(resp *http.Response, body string) *apiError {
	ae := &apiError{
		status: resp.StatusCode,
		msg:    fmt.Sprintf("powerbi api %s (%.300s)", resp.Status, body),
	}
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(ra)); err == nil && secs > 0 {
			ae.retryAfter = time.Duration(secs) * time.Second
		}
	}
	return ae
}

// do runs fn, retrying once on 401 (fresh token) and up to 2 times on
// 429 honoring Retry-After. This implements the Power BI API error
// handling guidance (401: refresh token; 429: back off then retry).
func (c *Client) do(ctx context.Context, fn func(token string) error) error {
	for attempt := 0; ; attempt++ {
		err := fn(c.token.Value)
		if err == nil {
			return nil
		}
		st := statusOf(err)
		switch {
		case st == 401:
			// Token rejected (expired/revoked): acquire a fresh one and
			// retry a single time.
			if attempt > 0 {
				return fmt.Errorf("Power BI 인증이 거부되었습니다(401). az login으로 다시 로그인해 주세요")
			}
			tok, terr := GetToken(ctx)
			if terr != nil {
				return fmt.Errorf("토큰 갱신 실패: %w", terr)
			}
			c.token = tok
		case st == 429:
			// Throttled: wait Retry-After (default 10s) and retry, max 2 times.
			if attempt >= 2 {
				return fmt.Errorf("Power BI 요청 한도 초과(429). 잠시 후 다시 시도해 주세요")
			}
			wait := retryAfterOf(err)
			if wait <= 0 {
				wait = 10 * time.Second
			}
			if wait > 30*time.Second {
				wait = 30 * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
		default:
			return err
		}
	}
}

// QueryDocs runs an EVALUATE DAX query against the Docs table and
// returns all rows.
func (c *Client) QueryDocs(ctx context.Context) ([]Doc, error) {
	body := map[string]interface{}{
		"queries": []map[string]interface{}{
			{
				"query": "EVALUATE Docs",
				"serializerSettings": map[string]interface{}{
					"includeNulls": true,
				},
			},
		},
	}
	var raw []byte
	err := c.do(ctx, func(token string) error {
		r, e := c.http.postJSON(ctx,
			apiBase+"/datasets/"+datasetID()+"/executeQueries",
			token, body)
		if e != nil {
			return e
		}
		raw = r
		return nil
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Results []struct {
			Tables []struct {
				Rows []map[string]interface{} `json:"rows"`
			} `json:"tables"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parse executeQueries response: %w (body: %.200s)", err, string(raw))
	}
	docs := []Doc{}
	if len(resp.Results) > 0 && len(resp.Results[0].Tables) > 0 {
		for _, r := range resp.Results[0].Tables[0].Rows {
			docs = append(docs, rowToDoc(r))
		}
	}
	return docs, nil
}

// rowToDoc normalizes Power BI's typed row objects ("Docs[title]" keys)
// into a Doc.
func rowToDoc(r map[string]interface{}) Doc {
	str := func(v interface{}) string {
		switch t := v.(type) {
		case nil:
			return ""
		case string:
			return t
		case float64:
			return fmt.Sprintf("%d", int64(t))
		default:
			return fmt.Sprintf("%v", t)
		}
	}
	get := func(short string) string {
		if v, ok := r["Docs["+short+"]"]; ok {
			return str(v)
		}
		return str(r[short])
	}
	d := Doc{}
	if f, ok := r["Docs[id]"].(float64); ok {
		d.ID = int64(f)
	} else if f, ok := r["id"].(float64); ok {
		d.ID = int64(f)
	}
	d.Title = get("title")
	d.Author = get("author")
	d.Category = get("category")
	d.Updated = get("updated")
	d.Status = get("status")
	return d
}

// AddRow pushes one row into the Docs table via the push-dataset
// rows API. Returns nil on success (200 with empty body).
func (c *Client) AddRow(ctx context.Context, d *Doc) error {
	body := map[string]interface{}{
		"rows": []map[string]interface{}{{
			"id":       d.ID,
			"title":    d.Title,
			"author":   d.Author,
			"category": d.Category,
			"updated":  d.Updated,
			"status":   d.Status,
		}},
	}
	return c.do(ctx, func(token string) error {
		_, e := c.http.postJSON(ctx,
			apiBase+"/datasets/"+datasetID()+"/tables/Docs/rows",
			token, body)
		return e
	})
}

// ClearDocs removes ALL rows from the Power BI Docs table.
// (Push datasets only support whole-table row deletion.)
func (c *Client) ClearDocs(ctx context.Context) error {
	return c.do(ctx, func(token string) error {
		return c.http.deleteJSON(ctx, apiBase+"/datasets/"+datasetID()+"/tables/Docs/rows", token)
	})
}

// addRowsBatch is the single-call row push used by AddRows.
func (c *Client) addRowsBatch(ctx context.Context, docs []Doc) error {
	rows := make([]map[string]interface{}, 0, len(docs))
	for i := range docs {
		d := docs[i]
		rows = append(rows, map[string]interface{}{
			"id":       d.ID,
			"title":    d.Title,
			"author":   d.Author,
			"category": d.Category,
			"updated":  d.Updated,
			"status":   d.Status,
		})
	}
	body := map[string]interface{}{"rows": rows}
	return c.do(ctx, func(token string) error {
		_, e := c.http.postJSON(ctx,
			apiBase+"/datasets/"+datasetID()+"/tables/Docs/rows",
			token, body)
		return e
	})
}

// batchSize keeps each rows push well below the push-dataset limits
// (10,000 rows / 1 MB per request) and bounds the blast radius of a
// failed batch.
const pushBatchSize = 100

// AddRows pushes multiple rows in batches of pushBatchSize, failing
// with the batch index that failed (already retried for 401/429 by do).
func (c *Client) AddRows(ctx context.Context, docs []Doc) error {
	for start := 0; start < len(docs); start += pushBatchSize {
		end := start + pushBatchSize
		if end > len(docs) {
			end = len(docs)
		}
		if err := c.addRowsBatch(ctx, docs[start:end]); err != nil {
			return fmt.Errorf("배치 %d-%d 전송 실패: %w", start+1, end, err)
		}
	}
	return nil
}
