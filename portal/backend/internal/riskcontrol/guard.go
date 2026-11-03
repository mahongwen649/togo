// Package riskcontrol enforces user bans through Core's official admin APIs.
// All persistent state belongs to Portal, never to the Core database.
package riskcontrol

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

const Message = "账号因触发风控已被禁止使用，请联系管理员。"

var errUserGone = errors.New("Core resource no longer exists")

type Core interface {
	AdminMarketRequest(context.Context, string, string, any, string) (coreclient.Response, error)
}

type Guard struct {
	DB     *sql.DB
	Core   Core
	Logger *slog.Logger
}

type event struct {
	ID     int64  `json:"id"`
	UserID int64  `json:"user_id"`
	Type   string `json:"type"`
	Phase  string `json:"phase"`
}

func (e event) actionable() bool {
	return e.ID > 0 && e.UserID > 0 && e.Phase == "request" && (e.Type == "cyber_policy" || e.Type == "cyber_policy_session_blocked")
}

func keyHash(key string) string { sum := sha256.Sum256([]byte(key)); return hex.EncodeToString(sum[:]) }

func credential(r *http.Request) string {
	parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && strings.TrimSpace(parts[1]) != "" {
		return strings.TrimSpace(parts[1])
	}
	if key := r.Header.Get("X-Api-Key"); key != "" {
		return key
	}
	return r.Header.Get("X-Goog-Api-Key")
}

// Middleware also serves Nginx auth_request. It never accepts a user ID or a
// claimed ban reason from the caller; only stored credential hashes are used.
func (g *Guard) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := credential(r)
		if key != "" {
			var banned bool
			err := g.DB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM portal_risk_keys WHERE key_hash=$1)`, keyHash(key)).Scan(&banned)
			if err != nil {
				writeError(w, 503, "RISK_CHECK_UNAVAILABLE", "风控状态暂时无法确认，请稍后重试")
				return
			}
			if banned {
				writeError(w, 403, "ACCOUNT_RISK_BANNED", Message)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"type": "permission_error", "code": code, "message": message}})
}

func (g *Guard) request(ctx context.Context, method, path string, body, out any) error {
	response, err := g.Core.AdminMarketRequest(ctx, method, path, body, "")
	if response.StatusCode == http.StatusNotFound {
		return errUserGone
	}
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Core risk API HTTP %d", response.StatusCode)
	}
	if out == nil {
		return nil
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(response.Body, &envelope); err != nil {
		return err
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return fmt.Errorf("Core risk API missing data")
	}
	return json.Unmarshal(envelope.Data, out)
}

func (g *Guard) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if err := g.tick(ctx, time.Now().UTC()); err != nil && ctx.Err() == nil {
			g.Logger.Error("risk ban scan failed; will retry", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (g *Guard) tick(ctx context.Context, now time.Time) error {
	// One scanner across replicas. Connection ownership is required by PostgreSQL
	// session advisory locks; do not acquire/release through pooled DB.Exec.
	conn, err := g.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var locked bool
	if err = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(728194028)`).Scan(&locked); err != nil {
		return err
	}
	if !locked {
		return nil
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(c, `SELECT pg_advisory_unlock(728194028)`)
	}()
	if err = g.releaseReenabled(ctx); err != nil {
		return err
	}
	var start time.Time
	err = g.DB.QueryRowContext(ctx, `SELECT checked_until FROM portal_risk_cursor WHERE id=1`).Scan(&start)
	if err == sql.ErrNoRows {
		start = now.Add(-24 * time.Hour)
	} else if err != nil {
		return err
	}
	// Fixed windows and pagination avoid starvation by large unrelated error volumes.
	// Replay five minutes for asynchronous log writes; event IDs make it idempotent.
	end := now
	if end.Sub(start) > time.Hour {
		end = start.Add(time.Hour)
	}
	for page := 1; ; page++ {
		q := url.Values{"view": {"all"}, "start_time": {start.Add(-5 * time.Minute).Format(time.RFC3339Nano)}, "end_time": {end.Format(time.RFC3339Nano)}, "phase": {"request"}, "q": {"cyber_policy"}, "page": {fmt.Sprint(page)}, "page_size": {"500"}, "sort_order": {"asc"}, "sort_by": {"created_at"}}
		var result struct {
			Items []event `json:"items"`
			Total int     `json:"total"`
		}
		if err = g.request(ctx, "GET", "/api/v1/admin/ops/request-errors?"+q.Encode(), nil, &result); err != nil {
			return err
		}
		for _, e := range result.Items {
			if e.actionable() {
				if err = g.ban(ctx, e); err != nil {
					return err
				}
			}
		}
		if page*500 >= result.Total {
			break
		}
		if len(result.Items) == 0 {
			return fmt.Errorf("Core risk pagination incomplete")
		}
	}
	_, err = g.DB.ExecContext(ctx, `INSERT INTO portal_risk_cursor(id,checked_until) VALUES(1,$1) ON CONFLICT(id) DO UPDATE SET checked_until=EXCLUDED.checked_until`, end)
	return err
}

func (g *Guard) ban(ctx context.Context, e event) error {
	var done bool
	if err := g.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM portal_risk_events WHERE event_id=$1)`, e.ID).Scan(&done); err != nil {
		return err
	}
	if done {
		return nil
	}
	var user struct {
		ID     int64  `json:"id"`
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	path := fmt.Sprintf("/api/v1/admin/users/%d", e.UserID)
	if err := g.request(ctx, "GET", path, nil, &user); err != nil {
		if errors.Is(err, errUserGone) {
			return g.record(ctx, e)
		}
		return err
	}
	if user.ID != e.UserID {
		return fmt.Errorf("Core risk user identity mismatch")
	}
	// Core explicitly prohibits disabling administrators.
	if user.Role == "admin" {
		g.Logger.Warn("risk event on administrator requires manual review", "user_id", e.UserID, "event_id", e.ID)
		return g.record(ctx, e)
	}
	var hashes []string
	for page := 1; ; page++ {
		var keys struct {
			Items []coreclient.APIKey `json:"items"`
			Total int                 `json:"total"`
		}
		if err := g.request(ctx, "GET", fmt.Sprintf("%s/api-keys?page=%d&page_size=100", path, page), nil, &keys); err != nil {
			return err
		}
		for _, key := range keys.Items {
			if key.Key == "" || strings.Contains(key.Key, "*") {
				return fmt.Errorf("Core risk API did not return full key")
			}
			hashes = append(hashes, keyHash(key.Key))
		}
		if page*100 >= keys.Total {
			break
		}
		if len(keys.Items) == 0 {
			return fmt.Errorf("Core key pagination incomplete")
		}
	}
	// Persist the ingress block before disabling Core, so failures and restarts
	// cannot lose the user-facing reason. A failed Core write remains retryable.
	tx, err := g.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, hash := range hashes {
		if _, err = tx.ExecContext(ctx, `INSERT INTO portal_risk_keys(key_hash,user_id,event_id) VALUES($1,$2,$3) ON CONFLICT(key_hash) DO UPDATE SET user_id=EXCLUDED.user_id,event_id=EXCLUDED.event_id`, hash, e.UserID, e.ID); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if user.Status != "disabled" {
		if err = g.request(ctx, "PUT", path, map[string]string{"status": "disabled"}, nil); err != nil {
			return err
		}
	}
	if err = g.record(ctx, e); err != nil {
		return err
	}
	g.Logger.Warn("user disabled by cyber risk policy", "user_id", e.UserID, "event_id", e.ID, "event_type", e.Type)
	return nil
}

func (g *Guard) record(ctx context.Context, e event) error {
	_, err := g.DB.ExecContext(ctx, `INSERT INTO portal_risk_events(event_id,user_id,event_type) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, e.ID, e.UserID, e.Type)
	return err
}

func (g *Guard) releaseReenabled(ctx context.Context) error {
	// Only completed bans can be lifted. Pending Core writes must keep blocking.
	rows, err := g.DB.QueryContext(ctx, `SELECT DISTINCT k.user_id FROM portal_risk_keys k JOIN portal_risk_events e ON e.event_id=k.event_id`)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		var user struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		}
		if err = g.request(ctx, "GET", fmt.Sprintf("/api/v1/admin/users/%d", id), nil, &user); err != nil {
			if !errors.Is(err, errUserGone) {
				return err
			}
		}
		if errors.Is(err, errUserGone) || (user.ID == id && user.Status == "active") {
			if _, err = g.DB.ExecContext(ctx, `DELETE FROM portal_risk_keys WHERE user_id=$1`, id); err != nil {
				return err
			}
		}
	}
	return nil
}
