package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

func (s *Server) isExclusiveGroup(ctx context.Context, id int64) (bool, error) {
	if s.portalDB == nil {
		return false, nil
	}
	var found bool
	err := s.portalDB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM portal_exclusive_channels WHERE core_group_id=$1)`, id).Scan(&found)
	return found, err
}

// Called before generic user key management. Provisioning invokes Core internally
// and does not traverse these public handlers.
func (s *Server) protectExclusiveUserAPI(w http.ResponseWriter, r *http.Request, token string) bool {
	if s.portalDB == nil && !s.exclusive.Enabled {
		return true
	}
	if !strings.HasPrefix(r.URL.Path, "/api/v1/keys") {
		return true
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/keys/") {
		id, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/api/v1/keys/"), 10, 64)
		if err != nil {
			return true
		}
		if s.portalDB != nil {
			var managed bool
			err = s.portalDB.QueryRowContext(r.Context(), `SELECT EXISTS(SELECT 1 FROM portal_exclusive_leases WHERE core_key_id=$1)`, id).Scan(&managed)
			if err != nil {
				s.exclusiveError(w, err)
				return false
			}
			if managed {
				writeAPIError(w, 403, "MANAGED_CHANNEL_KEY", "独享通道凭据由 Portal 管理")
				return false
			}
		}
		// Covers a Core key created just before its local ID checkpoint was saved.
		resp, err := s.core.UserAPIRequest(r.Context(), coreclient.CoreRequest{AccessToken: token, Method: "GET", Path: r.URL.Path})
		if err != nil {
			s.writeCoreProxyError(w, r, err)
			return false
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var data struct {
				Data coreclient.APIKey `json:"data"`
			}
			if json.Unmarshal(resp.Body, &data) != nil {
				writeAPIError(w, 502, "CORE_INVALID_RESPONSE", "Core 凭据校验失败")
				return false
			}
			protected, e := s.isExclusiveGroup(r.Context(), pointerID(data.Data.GroupID))
			if e != nil {
				s.exclusiveError(w, e)
				return false
			}
			if strings.HasPrefix(data.Data.Key, exclusiveUpstreamPrefix) || protected {
				writeAPIError(w, 403, "MANAGED_CHANNEL_KEY", "独享通道凭据由 Portal 管理")
				return false
			}
		}
	}
	if r.Method == "POST" || r.Method == "PUT" {
		b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeAPIError(w, 413, "REQUEST_TOO_LARGE", "请求体过大")
			return false
		}
		r.Body = io.NopCloser(bytes.NewReader(b))
		var p struct {
			GroupID   int64  `json:"group_id"`
			CustomKey string `json:"custom_key"`
		}
		if json.Unmarshal(b, &p) != nil {
			writeAPIError(w, 400, "INVALID_REQUEST", "Key 配置无效")
			return false
		}
		protected, err := s.isExclusiveGroup(r.Context(), p.GroupID)
		if err != nil {
			s.exclusiveError(w, err)
			return false
		}
		if protected || strings.HasPrefix(p.CustomKey, exclusiveUpstreamPrefix) || strings.HasPrefix(p.CustomKey, exclusiveTokenPrefix) {
			writeAPIError(w, 403, "MANAGED_CHANNEL_GROUP", "请通过通道大厅领取独享通道")
			return false
		}
	}
	return true
}
func pointerID(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}
func (s *Server) filterExclusiveUserResponse(ctx context.Context, path string, response coreclient.Response) (coreclient.Response, error) {
	if s.portalDB == nil && !bytes.Contains(response.Body, []byte(exclusiveUpstreamPrefix)) {
		return response, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response, nil
	}
	if path != "/api/v1/keys" && path != "/api/v1/groups/available" && path != "/api/v1/groups/rates" {
		return response, nil
	}
	var envelope map[string]any
	decoder := json.NewDecoder(bytes.NewReader(response.Body))
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		return response, err
	}
	protected := map[int64]bool{}
	if s.portalDB != nil {
		rows, err := s.portalDB.QueryContext(ctx, `SELECT core_group_id FROM portal_exclusive_channels WHERE core_group_id IS NOT NULL`)
		if err != nil {
			return response, err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				return response, err
			}
			protected[id] = true
		}
		if err = rows.Err(); err != nil {
			return response, err
		}
	}
	idOf := func(v any) int64 {
		n, ok := v.(json.Number)
		if !ok {
			return 0
		}
		id, _ := n.Int64()
		return id
	}
	filter := func(items []any) []any {
		out := []any{}
		for _, v := range items {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			key, _ := m["key"].(string)
			groupID := idOf(m["id"])
			if path == "/api/v1/keys" {
				groupID = idOf(m["group_id"])
			}
			if !strings.HasPrefix(key, exclusiveUpstreamPrefix) && !protected[groupID] {
				out = append(out, v)
			}
		}
		return out
	}
	switch data := envelope["data"].(type) {
	case []any:
		envelope["data"] = filter(data)
	case map[string]any:
		if items, ok := data["items"].([]any); ok {
			data["items"] = filter(items)
		}
		if path == "/api/v1/groups/rates" {
			for key := range data {
				id, _ := strconv.ParseInt(key, 10, 64)
				if protected[id] {
					delete(data, key)
				}
			}
		}
	default:
		return response, errors.New("unexpected Core key/group response shape")
	}
	b, err := json.Marshal(envelope)
	response.Body = b
	return response, err
}

// Covers nested key DTOs in usage, dashboards and auth responses as well as key
// lists, including partially provisioned keys not yet checkpointed in Portal.
func redactExclusiveCredentials(body []byte) []byte {
	if !bytes.Contains(body, []byte(exclusiveUpstreamPrefix)) && !bytes.Contains(body, []byte(`\u`)) {
		return body
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return []byte(`{"code":"REDACTED_RESPONSE","message":"Protected response unavailable"}`)
	}
	var redact func(any) any
	redact = func(v any) any {
		switch x := v.(type) {
		case string:
			for {
				start := strings.Index(x, exclusiveUpstreamPrefix)
				if start < 0 {
					break
				}
				end := start + len(exclusiveUpstreamPrefix)
				for end < len(x) && ((x[end] >= 'a' && x[end] <= 'f') || (x[end] >= '0' && x[end] <= '9')) {
					end++
				}
				x = x[:start] + "[Portal managed credential]" + x[end:]
			}
			return x
		case []any:
			for i := range x {
				x[i] = redact(x[i])
			}
		case map[string]any:
			for k := range x {
				v := redact(x[k])
				cleanKey := redact(k).(string)
				if cleanKey != k {
					delete(x, k)
				}
				x[cleanKey] = v
			}
		}
		return v
	}
	b, err := json.Marshal(redact(value))
	if err != nil {
		return []byte(`{"code":"REDACTED_RESPONSE"}`)
	}
	return b
}
func (s *Server) registerExclusive(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/exclusive-channels", s.exclusiveList)
	mux.HandleFunc("GET /api/v1/exclusive-channels/mine", s.exclusiveMine)
	mux.HandleFunc("GET /api/v1/exclusive-channels/history", s.exclusiveHistory)
	mux.HandleFunc("GET /api/v1/exclusive-channels/usage", s.exclusiveUsageRecords(s.accountMarketUsageRecords))
	mux.HandleFunc("GET /api/v1/exclusive-channels/usage/stats", s.exclusiveUsageRecords(s.accountMarketUsageStats))
	mux.HandleFunc("GET /api/v1/exclusive-channels/usage/dashboard/models", s.exclusiveUsageRecords(s.accountMarketUsageModels))
	mux.HandleFunc("GET /api/v1/exclusive-channels/usage/dashboard/snapshot-v2", s.exclusiveUsageRecords(s.accountMarketUsageSnapshot))
	mux.HandleFunc("GET /api/v1/exclusive-channels/usage/errors", s.exclusiveUsageRecords(s.accountMarketUsageErrors))
	mux.HandleFunc("GET /api/v1/exclusive-channels/usage/errors/{errorID}", s.exclusiveUsageRecords(s.accountMarketUsageErrorDetail))
	mux.HandleFunc("POST /api/v1/exclusive-channels/{channelID}/accounts/{accountID}/refresh-usage", s.exclusiveRefreshUsage)
	mux.HandleFunc("GET /api/v1/exclusive-channels/{channelID}/accounts/{accountID}/downgrade-test", s.exclusiveAccountTestView)
	mux.HandleFunc("POST /api/v1/exclusive-channels/{channelID}/accounts/{accountID}/downgrade-test", s.exclusiveAccountTestStart)
	mux.HandleFunc("GET /api/v1/exclusive-channels/{channelID}/page-test", s.exclusivePageTestView)
	mux.HandleFunc("POST /api/v1/exclusive-channels/{channelID}/page-test", s.exclusivePageTestStart)
	mux.HandleFunc("POST /api/v1/exclusive-channels/{channelID}/claim", s.exclusiveClaim)
	mux.HandleFunc("POST /api/v1/exclusive-channels/{channelID}/release", s.exclusiveReturn)
	mux.HandleFunc("GET /api/v1/admin/exclusive-channels", s.exclusiveList)
	mux.HandleFunc("GET /api/v1/admin/exclusive-channels/accounts", s.exclusiveCandidates)
	mux.HandleFunc("GET /api/v1/admin/exclusive-channels/events", s.exclusiveEvents)
	mux.HandleFunc("POST /api/v1/admin/exclusive-channels", s.exclusivePublishChannel)
	mux.HandleFunc("PUT /api/v1/admin/exclusive-channels/{channelID}", s.exclusiveUpdateChannel)
	mux.HandleFunc("DELETE /api/v1/admin/exclusive-channels/{channelID}", s.exclusiveDeleteChannel)
	mux.HandleFunc("PUT /api/v1/admin/exclusive-channels/{channelID}/availability", s.exclusiveAvailability)
	mux.HandleFunc("POST /api/v1/admin/exclusive-channels/{channelID}/retry", s.exclusiveRetryChannel)
	mux.HandleFunc("POST /api/v1/admin/exclusive-channels/{channelID}/release", s.exclusiveReturn)
	mux.HandleFunc("GET /api/v1/admin/pelican-checks", s.pelicanChecks)
	mux.HandleFunc("GET /api/v1/admin/pelican-checks/options", s.pelicanCheckOptions)
	mux.HandleFunc("PUT /api/v1/admin/pelican-checks/config", s.updatePelicanCheckConfig)
	mux.HandleFunc("POST /api/v1/admin/pelican-checks/run", s.runPelicanCheckNow)
	mux.HandleFunc("DELETE /api/v1/admin/pelican-checks/artworks/{artworkID}", s.deletePelicanArtwork)
	mux.HandleFunc("/exclusive/", s.exclusiveGateway)
	mux.HandleFunc("/core-ingress/", s.exclusiveCoreIngress)
}
