package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const pelicanCheckLockID int64 = 734210093

const pelicanStreamLimit = exclusivePageLimit * 3

var errPelicanCheckInProgress = errors.New("pelican check is already in progress")

type pelicanCore interface {
	marketCore
	exclusiveRelay
	pelicanAccountPrompter
}

type pelicanCheckConfig struct {
	ReasoningEffort string     `json:"reasoning_effort,omitempty"`
	Model           string     `json:"model,omitempty"`
	Prompt          string     `json:"prompt,omitempty"`
	Enabled         bool       `json:"enabled"`
	GroupID         *int64     `json:"group_id"`
	GroupName       string     `json:"group_name"`
	APIKeyID        *int64     `json:"api_key_id"`
	APIKeyName      string     `json:"api_key_name"`
	UpdatedAt       time.Time  `json:"updated_at"`
	NextRunAt       *time.Time `json:"next_run_at"`
	SchedulerSeenAt *time.Time `json:"scheduler_seen_at"`
}

type pelicanTimelinePoint struct {
	ID         int64      `json:"id"`
	APIKeyID   int64      `json:"api_key_id"`
	APIKeyName string     `json:"api_key_name"`
	GroupName  string     `json:"group_name"`
	Attempt    int        `json:"attempt"`
	Progress   string     `json:"progress"`
	State      string     `json:"state"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	LatencyMs  int64      `json:"latency_ms"`
	Error      string     `json:"error,omitempty"`
}

type pelicanCheckItem struct {
	GroupID    int64                  `json:"group_id"`
	GroupName  string                 `json:"group_name"`
	APIKeyID   int64                  `json:"api_key_id"`
	APIKeyName string                 `json:"api_key_name"`
	State      string                 `json:"state"`
	StartedAt  *time.Time             `json:"started_at"`
	FinishedAt *time.Time             `json:"finished_at"`
	LatencyMs  int64                  `json:"latency_ms"`
	Error      string                 `json:"error,omitempty"`
	Attempt    int                    `json:"attempt"`
	Progress   string                 `json:"progress"`
	Timeline   []pelicanTimelinePoint `json:"timeline"`
}

type pelicanArtwork struct {
	ID          int64     `json:"id"`
	GroupID     int64     `json:"group_id"`
	GroupName   string    `json:"group_name"`
	APIKeyID    int64     `json:"api_key_id"`
	APIKeyName  string    `json:"api_key_name"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
	LatencyMs   int64     `json:"latency_ms"`
	HTML        string    `json:"html"`
	AccountID   *int64    `json:"account_id,omitempty"`
	AccountName string    `json:"account_name,omitempty"`
}

type pelicanCoreKey struct {
	ID      int64  `json:"id"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	GroupID *int64 `json:"group_id"`
	Status  string `json:"status"`
}

type pelicanGroupOption struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

type pelicanKeyOption struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Preview string `json:"preview"`
	Status  string `json:"status"`
}

func loadPelicanCheckConfig(ctx context.Context, db *sql.DB) (pelicanCheckConfig, error) {
	return loadPelicanConfig(ctx, db)
}

func executePelicanCheck(ctx context.Context, db *sql.DB, core pelicanCore, logger *slog.Logger, config pelicanCheckConfig, runID int64, key pelicanCoreKey, requestErr error) error {
	started := time.Now()
	state, message, page := "failed", "无法读取检查用 Key，请重新保存配置", ""
	clientRequestID := ""
	var accountID *int64
	accountName := ""
	if requestErr == nil {
		message = ""
		lookupCtx, cancelLookup := context.WithTimeout(ctx, 30*time.Second)
		accounts, err := listPelicanAccounts(lookupCtx, core, *config.GroupID, runID, config.Model)
		cancelLookup()
		requestErr = err
		for index, account := range accounts {
			if index >= pelicanMaxAttempts || ctx.Err() != nil {
				break
			}
			attempt := index + 1
			if index > 0 {
				timer := time.NewTimer(time.Duration(index) * 2 * time.Second)
				select {
				case <-ctx.Done():
					timer.Stop()
					requestErr = ctx.Err()
				case <-timer.C:
				}
				if ctx.Err() != nil {
					break
				}
			}
			if _, err := db.ExecContext(ctx, `UPDATE portal_pelican_check_runs SET attempt=$1,progress=$2 WHERE id=$3 AND state='running'`, attempt, "正在生成作品", runID); err != nil {
				return err
			}
			attemptCtx, cancel := context.WithTimeout(ctx, pelicanAttemptTimeout)
			clientRequestID = fmt.Sprintf("portal-pelican-%d-%d-%d", runID, attempt, time.Now().UnixNano())
			page, requestErr = requestPelicanAccountPage(attemptCtx, core, account.ID, clientRequestID, config.Model, config.Prompt, config.ReasoningEffort)
			cancel()
			if requestErr == nil {
				state, message = "succeeded", ""
				id := account.ID
				accountID, accountName = &id, account.Name
				break
			}
			// Any failed probe (including SSE 429, timeout or invalid artwork)
			// advances to a different account. No partial artwork is published.
			page, message = "", requestErr.Error()
			logger.Warn("pelican account skipped", "run_id", runID, "account_id", account.ID, "attempt", attempt, "error", requestErr)
		}
		if ctx.Err() != nil && state != "succeeded" {
			requestErr = ctx.Err()
		}
	}
	if requestErr != nil {
		logger.Warn("pelican request failed", "group_id", *config.GroupID, "api_key_id", *config.APIKeyID, "error", requestErr)
		if message == "" {
			message = requestErr.Error()
		}
	}
	latency := time.Since(started).Milliseconds()
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer saveCancel()
	_, saveErr := db.ExecContext(saveCtx, `UPDATE portal_pelican_check_runs SET state=$1,finished_at=CURRENT_TIMESTAMP,latency_ms=$2,html=$3,error=$4,progress='',client_request_id=$5,account_id=$6,account_name=$7 WHERE id=$8 AND state='running'`, state, latency, page, message, clientRequestID, accountID, accountName, runID)
	if saveErr != nil {
		return saveErr
	}
	_, _ = db.ExecContext(saveCtx, `DELETE FROM portal_pelican_check_runs WHERE started_at < CURRENT_TIMESTAMP - INTERVAL '7 days'`)
	return requestErr
}

func requestPelicanPage(ctx context.Context, relay exclusiveRelay, key, clientRequestID string) (string, string, error) {
	body, err := json.Marshal(map[string]any{
		"model":        exclusivePageModel,
		"reasoning":    map[string]string{"effort": "high"},
		"input":        exclusivePagePrompt,
		"instructions": "仅返回完整、自包含的 HTML 页面，包含内联 SVG、CSS 和必要的 JavaScript。不要解释、不要 Markdown 代码围栏，不引用外部资源。",
		"stream":       true,
	})
	if err != nil {
		return "", "检查请求构造失败", err
	}
	request := (&http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/v1/responses"}, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body))}).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer "+key)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("X-Client-Request-ID", clientRequestID)
	response, err := relay.RelayRoundTrip(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", "检查请求超时", err
		}
		return "", "上游连接失败", &pelicanTransientError{err}
	}
	defer response.Body.Close()
	data, readErr := io.ReadAll(io.LimitReader(response.Body, pelicanStreamLimit+1))
	if len(data) > pelicanStreamLimit {
		return "", "Core 响应内容过大", errors.New("Core response exceeds the page size limit")
	}
	if readErr != nil {
		return "", "上游连接中断，未收到完整生成结果", &pelicanTransientError{readErr}
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message := pelicanCoreError(data, response.StatusCode)
		if response.StatusCode == 429 || response.StatusCode >= 500 {
			return "", message, &pelicanTransientError{errors.New(message)}
		}
		return "", message, errors.New(message)
	}
	page, err := parsePelicanStream(data)
	if err != nil {
		if pelicanRetryable(err) {
			return "", "上游生成连接中断，重试后仍未完成（" + err.Error() + "）", err
		}
		return "", err.Error(), err
	}
	return page, "", nil
}

// resolvePelicanAccount uses the administrator-only usage endpoint. The
// gateway records usage asynchronously, so briefly poll for the row associated
// with the unique client request ID rather than exposing account metadata in a
// public model response header.
func resolvePelicanAccount(ctx context.Context, core marketCore, clientRequestID string) (*int64, string) {
	if strings.TrimSpace(clientRequestID) == "" {
		return nil, ""
	}
	requestID := "client:" + clientRequestID
	path := "/api/v1/admin/usage?page=1&page_size=1&request_id=" + url.QueryEscape(requestID)
	for attempt := 0; attempt < 10; attempt++ {
		response, err := core.AdminMarketRequest(ctx, http.MethodGet, path, nil, "")
		if err == nil {
			var result struct {
				Items []struct {
					AccountID int64 `json:"account_id"`
					Account   *struct {
						ID   int64  `json:"id"`
						Name string `json:"name"`
					} `json:"account"`
				} `json:"items"`
			}
			if decodePelicanCoreData(response.Body, &result) == nil && len(result.Items) > 0 {
				id := result.Items[0].AccountID
				name := ""
				if result.Items[0].Account != nil {
					if result.Items[0].Account.ID > 0 {
						id = result.Items[0].Account.ID
					}
					name = result.Items[0].Account.Name
				}
				if id > 0 {
					return &id, name
				}
			}
		}
		if attempt < 9 {
			timer := time.NewTimer(200 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ""
			case <-timer.C:
			}
		}
	}
	return nil, ""
}

type pelicanUsageAccount struct {
	AccountID int64 `json:"account_id"`
	Account   *struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"account"`
	CreatedAt time.Time `json:"created_at"`
}

func pelicanUsageAccountIdentity(item pelicanUsageAccount) (int64, string) {
	id := item.AccountID
	name := ""
	if item.Account != nil {
		if item.Account.ID > 0 {
			id = item.Account.ID
		}
		name = item.Account.Name
	}
	return id, name
}

// backfillPelicanArtworkAccounts covers artworks generated before request
// correlation was introduced. The check Key is dedicated and checks are spaced
// thirty minutes apart, so a narrow completion-time match identifies the usage
// row without guessing from account state.
func backfillPelicanArtworkAccounts(ctx context.Context, core marketCore, db *sql.DB, artworks []pelicanArtwork) {
	usageByKey := make(map[int64][]pelicanUsageAccount)
	loaded := make(map[int64]bool)
	for index := range artworks {
		artwork := &artworks[index]
		if artwork.AccountID != nil || artwork.APIKeyID <= 0 {
			continue
		}
		if !loaded[artwork.APIKeyID] {
			loaded[artwork.APIKeyID] = true
			path := fmt.Sprintf("/api/v1/admin/usage?page=1&page_size=1000&api_key_id=%d&model=%s", artwork.APIKeyID, url.QueryEscape(exclusivePageModel))
			response, err := core.AdminMarketRequest(ctx, http.MethodGet, path, nil, "")
			if err == nil {
				var result struct {
					Items []pelicanUsageAccount `json:"items"`
				}
				if decodePelicanCoreData(response.Body, &result) == nil {
					usageByKey[artwork.APIKeyID] = result.Items
				}
			}
		}
		var matched *pelicanUsageAccount
		bestDistance := 5 * time.Minute
		for usageIndex := range usageByKey[artwork.APIKeyID] {
			usage := &usageByKey[artwork.APIKeyID][usageIndex]
			id, _ := pelicanUsageAccountIdentity(*usage)
			if id <= 0 {
				continue
			}
			distance := usage.CreatedAt.Sub(artwork.FinishedAt)
			if distance < 0 {
				distance = -distance
			}
			if distance < bestDistance {
				bestDistance = distance
				matched = usage
			}
		}
		if matched == nil {
			continue
		}
		id, name := pelicanUsageAccountIdentity(*matched)
		artwork.AccountID = &id
		artwork.AccountName = name
		_, _ = db.ExecContext(ctx, `UPDATE portal_pelican_check_runs SET account_id=$1,account_name=$2 WHERE id=$3 AND account_id IS NULL`, id, name, artwork.ID)
	}
}

type pelicanTransientError struct{ error }

func pelicanRetryable(err error) bool {
	var transient *pelicanTransientError
	if errors.As(err, &transient) {
		return true
	}
	if err == nil {
		return false
	}
	for _, code := range []string{"stream_read_error", "server_error", "temporarily_unavailable", "rate_limit_exceeded", "upstream_error", "Core 流式响应未正常完成"} {
		if strings.Contains(err.Error(), code) {
			return true
		}
	}
	return false
}

func parsePelicanStream(body []byte) (string, error) {
	if len(body) > pelicanStreamLimit {
		return "", errors.New("Core 响应内容过大")
	}
	var streamed strings.Builder
	var doneText string
	var completed bool
	var completedResponse json.RawMessage
	scanner := bufio.NewScanner(bytes.NewReader(body))
	scanner.Buffer(make([]byte, 1024), pelicanStreamLimit+1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var event struct {
			Type     string          `json:"type"`
			Delta    string          `json:"delta"`
			Text     string          `json:"text"`
			Error    json.RawMessage `json:"error"`
			Response json.RawMessage `json:"response"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue
		}
		switch event.Type {
		case "response.output_text.delta":
			streamed.WriteString(event.Delta)
		case "response.output_text.done":
			if event.Text != "" {
				doneText = event.Text
			}
		case "response.completed", "response.done":
			completed = true
			completedResponse = event.Response
		case "response.failed", "response.incomplete", "error":
			return "", errors.New(pelicanEventError(event.Error, event.Response))
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("Core 流式响应读取失败: %w", err)
	}
	if !completed {
		return "", errors.New("Core 流式响应未正常完成")
	}
	if len(completedResponse) > 0 && string(completedResponse) != "null" {
		if page, err := parseExclusivePage(completedResponse); err == nil {
			return page, nil
		}
	}
	text := doneText
	if text == "" {
		text = streamed.String()
	}
	for _, candidate := range exclusivePageCandidates(text) {
		if page, ok := extractExclusiveDocument(candidate); ok {
			return page, nil
		}
	}
	return "", errors.New("回复中未找到可展示的 HTML 或 SVG 页面")
}

func pelicanCoreError(body []byte, status int) string {
	message := fmt.Sprintf("检查请求返回 HTTP %d", status)
	var envelope struct {
		Message string `json:"message"`
		Error   struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		if strings.TrimSpace(envelope.Error.Message) != "" {
			message = envelope.Error.Message
			if envelope.Error.Code != "" {
				message += " (" + envelope.Error.Code + ")"
			}
		} else if strings.TrimSpace(envelope.Message) != "" {
			message = envelope.Message
		}
	}
	return truncatePelicanError(message)
}

func pelicanEventError(eventError, response json.RawMessage) string {
	for _, body := range []json.RawMessage{eventError, response} {
		if len(body) == 0 || string(body) == "null" {
			continue
		}
		var detail struct {
			Code    string `json:"code"`
			Message string `json:"message"`
			Error   *struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &detail) != nil {
			continue
		}
		if detail.Error != nil {
			detail.Code, detail.Message = detail.Error.Code, detail.Error.Message
		}
		if strings.TrimSpace(detail.Message) != "" {
			if detail.Code != "" {
				detail.Message += " (" + detail.Code + ")"
			}
			return truncatePelicanError(detail.Message)
		}
	}
	return "Core 流式生成失败"
}

func truncatePelicanError(message string) string {
	const limit = 500
	characters := []rune(strings.TrimSpace(message))
	if len(characters) > limit {
		return string(characters[:limit]) + "..."
	}
	return string(characters)
}

func decodePelicanCoreData(body []byte, out any) error {
	var envelope struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 || len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return errors.New("Core returned an unsuccessful response")
	}
	return json.Unmarshal(envelope.Data, out)
}

func loadPelicanGroups(ctx context.Context, core marketCore) ([]rentalGroup, error) {
	response, err := core.AdminMarketRequest(ctx, http.MethodGet, "/api/v1/admin/groups/all?include_inactive=true", nil, "")
	if err != nil {
		return nil, err
	}
	var groups []rentalGroup
	if err := decodePelicanCoreData(response.Body, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

func loadPelicanGroupKeys(ctx context.Context, core marketCore, groupID int64) ([]pelicanCoreKey, error) {
	keys := []pelicanCoreKey{}
	for page := 1; page <= 100; page++ {
		path := fmt.Sprintf("/api/v1/admin/groups/%d/api-keys?page=%d&page_size=100", groupID, page)
		response, err := core.AdminMarketRequest(ctx, http.MethodGet, path, nil, "")
		if err != nil {
			return nil, err
		}
		var result struct {
			Items []pelicanCoreKey `json:"items"`
			Pages int              `json:"pages"`
		}
		if err := decodePelicanCoreData(response.Body, &result); err != nil {
			return nil, err
		}
		keys = append(keys, result.Items...)
		if result.Pages <= page || len(result.Items) == 0 {
			break
		}
	}
	return keys, nil
}

func findPelicanCoreKey(ctx context.Context, core marketCore, groupID, keyID int64) (pelicanCoreKey, error) {
	keys, err := loadPelicanGroupKeys(ctx, core, groupID)
	if err != nil {
		return pelicanCoreKey{}, err
	}
	for _, key := range keys {
		if key.ID == keyID && key.GroupID != nil && *key.GroupID == groupID {
			if key.Status != "active" || strings.TrimSpace(key.Key) == "" {
				return pelicanCoreKey{}, errors.New("configured API key is not active")
			}
			return key, nil
		}
	}
	return pelicanCoreKey{}, errors.New("configured API key no longer belongs to the selected group")
}

func (s *Server) requirePelicanAdmin(w http.ResponseWriter, r *http.Request) bool {
	if _, ok := s.requireActiveAdmin(w, r); !ok {
		return false
	}
	return s.requireMarketDB(w)
}

func (s *Server) requirePelicanViewer(w http.ResponseWriter, r *http.Request) (bool, bool) {
	token, ok := bearerToken(w, r)
	if !ok {
		return false, false
	}
	user, err := s.core.Profile(r.Context(), token)
	if err != nil {
		s.writeCoreProxyError(w, r, err)
		return false, false
	}
	if user.Status != "active" {
		writeAPIError(w, http.StatusForbidden, "USER_INACTIVE", "账号已停用")
		return false, false
	}
	return user.Role == "admin", s.requireMarketDB(w)
}

func (s *Server) pelicanError(w http.ResponseWriter, err error) {
	s.logger.Error("pelican check operation failed", "error", err)
	writeAPIError(w, http.StatusServiceUnavailable, "PELICAN_CHECK_UNAVAILABLE", "鹈鹕绘图检查暂时不可用")
}

func (s *Server) pelicanCheckOptions(w http.ResponseWriter, r *http.Request) {
	if !s.requirePelicanAdmin(w, r) {
		return
	}
	core, ok := s.core.(marketCore)
	if !ok {
		writeAPIError(w, http.StatusServiceUnavailable, "CORE_ADMIN_UNAVAILABLE", "Core 管理接口不可用")
		return
	}
	groups, err := loadPelicanGroups(r.Context(), core)
	if err != nil {
		s.pelicanError(w, err)
		return
	}
	groupOptions := []pelicanGroupOption{}
	for _, group := range groups {
		if group.Status == "active" && group.Platform == "openai" {
			groupOptions = append(groupOptions, pelicanGroupOption{ID: group.ID, Name: group.Name, Platform: group.Platform})
		}
	}
	modelOptions := []string{}
	keyOptions := []pelicanKeyOption{}
	if raw := strings.TrimSpace(r.URL.Query().Get("group_id")); raw != "" {
		groupID, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || groupID <= 0 {
			writeAPIError(w, http.StatusBadRequest, "INVALID_GROUP", "分组无效")
			return
		}
		if rawKey := r.URL.Query().Get("api_key_id"); rawKey != "" {
			keyID, parseErr := strconv.ParseInt(rawKey, 10, 64)
			if parseErr != nil || keyID <= 0 {
				writeAPIError(w, 400, "INVALID_PELICAN_KEY", "API Key 无效")
				return
			}
			key, keyErr := findPelicanCoreKey(r.Context(), core, groupID, keyID)
			if keyErr != nil {
				writeAPIError(w, 400, "INVALID_PELICAN_KEY", "所选 Key 不可用")
				return
			}
			relay, ok := s.core.(exclusiveRelay)
			if !ok {
				s.pelicanError(w, errors.New("model relay unavailable"))
				return
			}
			request := (&http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/v1/models"}, Header: make(http.Header)}).WithContext(r.Context())
			request.Header.Set("Authorization", "Bearer "+key.Key)
			modelCtx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
			defer cancel()
			response, requestErr := relay.RelayRoundTrip(request.WithContext(modelCtx))
			if requestErr != nil {
				s.pelicanError(w, requestErr)
				return
			}
			defer response.Body.Close()
			var catalog struct {
				Data []struct {
					ID string `json:"id"`
				} `json:"data"`
			}
			if response.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&catalog) != nil {
				s.pelicanError(w, errors.New("model catalog unavailable"))
				return
			}
			seen := map[string]bool{}
			for _, item := range catalog.Data {
				if item.ID != "" && !seen[item.ID] {
					modelOptions = append(modelOptions, item.ID)
					seen[item.ID] = true
				}
			}
		}
		keys, loadErr := loadPelicanGroupKeys(r.Context(), core, groupID)
		if loadErr != nil {
			s.pelicanError(w, loadErr)
			return
		}
		for _, key := range keys {
			if key.Status == "active" && key.GroupID != nil && *key.GroupID == groupID {
				preview := key.Key
				if len(preview) > 4 {
					preview = "****" + preview[len(preview)-4:]
				}
				keyOptions = append(keyOptions, pelicanKeyOption{ID: key.ID, Name: key.Name, Preview: preview, Status: key.Status})
			}
		}
	}
	writeSuccess(w, map[string]any{"groups": groupOptions, "keys": keyOptions, "models": modelOptions})
}

func (s *Server) updatePelicanCheckConfig(w http.ResponseWriter, r *http.Request) {
	if !s.requirePelicanAdmin(w, r) {
		return
	}
	var input struct {
		Enabled         bool    `json:"enabled"`
		GroupID         *int64  `json:"group_id"`
		APIKeyID        *int64  `json:"api_key_id"`
		Model           *string `json:"model"`
		Prompt          *string `json:"prompt"`
		ReasoningEffort *string `json:"reasoning_effort"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PELICAN_CONFIG", "检查配置无效")
		return
	}
	if input.ReasoningEffort != nil {
		switch *input.ReasoningEffort {
		case "none", "minimal", "low", "medium", "high", "xhigh":
		default:
			writeAPIError(w, http.StatusBadRequest, "INVALID_PELICAN_CONFIG", "不支持的思考深度")
			return
		}
	}
	for _, field := range []struct {
		value *string
		limit int
		label string
	}{{input.Model, 200, "模型"}, {input.Prompt, 10000, "检查题目"}} {
		if field.value != nil {
			*field.value = strings.TrimSpace(*field.value)
			if *field.value == "" || len([]rune(*field.value)) > field.limit {
				writeAPIError(w, http.StatusBadRequest, "INVALID_PELICAN_CONFIG", fmt.Sprintf("%s不能为空且不能超过 %d 个字符", field.label, field.limit))
				return
			}
		}
	}
	if !input.Enabled && input.GroupID == nil && input.APIKeyID == nil {
		if _, err := s.portalDB.ExecContext(r.Context(), `UPDATE portal_pelican_check_config SET enabled=FALSE,model=COALESCE($1,model),prompt=COALESCE($2,prompt),reasoning_effort=COALESCE($3,reasoning_effort),updated_at=CURRENT_TIMESTAMP WHERE id=1`, input.Model, input.Prompt, input.ReasoningEffort); err != nil {
			s.pelicanError(w, err)
			return
		}
		config, err := loadPelicanCheckConfig(r.Context(), s.portalDB)
		if err != nil {
			s.pelicanError(w, err)
			return
		}
		writeSuccess(w, config)
		return
	}
	if input.GroupID == nil || input.APIKeyID == nil || *input.GroupID <= 0 || *input.APIKeyID <= 0 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PELICAN_CONFIG", "请选择检查分组和 API Key")
		return
	}
	core, ok := s.core.(pelicanCore)
	if !ok {
		writeAPIError(w, http.StatusServiceUnavailable, "CORE_ADMIN_UNAVAILABLE", "Core 管理接口不可用")
		return
	}
	groups, err := loadPelicanGroups(r.Context(), core)
	if err != nil {
		s.pelicanError(w, err)
		return
	}
	groupName := ""
	for _, group := range groups {
		if group.ID == *input.GroupID && group.Status == "active" && group.Platform == "openai" {
			groupName = group.Name
			break
		}
	}
	if groupName == "" {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PELICAN_GROUP", "请选择启用中的 OpenAI 分组")
		return
	}
	key, err := findPelicanCoreKey(r.Context(), core, *input.GroupID, *input.APIKeyID)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "INVALID_PELICAN_KEY", "所选 Key 不可用或不属于该分组")
		return
	}
	_, err = s.portalDB.ExecContext(r.Context(), `UPDATE portal_pelican_check_config SET next_run_at=CASE WHEN NOT enabled OR group_id IS DISTINCT FROM $2 OR api_key_id IS DISTINCT FROM $4 THEN CURRENT_TIMESTAMP ELSE next_run_at END,enabled=$1,group_id=$2,group_name=$3,api_key_id=$4,api_key_name=$5,model=COALESCE($6,model),prompt=COALESCE($7,prompt),reasoning_effort=COALESCE($8,reasoning_effort),updated_at=CURRENT_TIMESTAMP WHERE id=1`, input.Enabled, *input.GroupID, groupName, *input.APIKeyID, key.Name, input.Model, input.Prompt, input.ReasoningEffort)
	if err != nil {
		s.pelicanError(w, err)
		return
	}
	config, err := loadPelicanCheckConfig(r.Context(), s.portalDB)
	if err != nil {
		s.pelicanError(w, err)
		return
	}
	writeSuccess(w, config)
}

func (s *Server) runPelicanCheckNow(w http.ResponseWriter, r *http.Request) {
	if !s.requirePelicanAdmin(w, r) {
		return
	}
	config, err := loadPelicanCheckConfig(r.Context(), s.portalDB)
	if err != nil {
		s.pelicanError(w, err)
		return
	}
	if !config.Enabled || config.GroupID == nil || config.APIKeyID == nil {
		writeAPIError(w, http.StatusConflict, "PELICAN_CHECK_DISABLED", "请先保存配置并开启检测")
		return
	}
	core, ok := s.core.(pelicanCore)
	if !ok {
		writeAPIError(w, http.StatusServiceUnavailable, "CORE_ADMIN_UNAVAILABLE", "Core 管理接口不可用")
		return
	}
	_, err = findPelicanCoreKey(r.Context(), core, *config.GroupID, *config.APIKeyID)
	if err != nil {
		writeAPIError(w, http.StatusConflict, "PELICAN_KEY_INVALID", "当前 API Key 已失效或已移动分组，请重新选择并保存配置")
		return
	}
	if s.pelicanRunner == nil {
		writeAPIError(w, http.StatusServiceUnavailable, "PELICAN_WORKER_UNAVAILABLE", "检查服务未启动")
		return
	}
	runID, err := s.pelicanRunner.start(r.Context(), true)
	if errors.Is(err, errPelicanCheckInProgress) {
		writeAPIError(w, http.StatusConflict, "PELICAN_CHECK_IN_PROGRESS", "鹈鹕绘图检查正在进行中")
		return
	}
	if err != nil {
		s.pelicanError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"state": "running", "run_id": runID})
}

func (s *Server) deletePelicanArtwork(w http.ResponseWriter, r *http.Request) {
	if !s.requirePelicanAdmin(w, r) {
		return
	}
	artworkID, ok := pathID(w, r, "artworkID")
	if !ok {
		return
	}
	result, err := s.portalDB.ExecContext(r.Context(), `DELETE FROM portal_pelican_check_runs WHERE id=$1 AND state='succeeded' AND html<>''`, artworkID)
	if err != nil {
		s.pelicanError(w, err)
		return
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		s.pelicanError(w, err)
		return
	}
	if deleted == 0 {
		writeAPIError(w, http.StatusNotFound, "PELICAN_ARTWORK_NOT_FOUND", "作品不存在或已删除")
		return
	}
	writeSuccess(w, map[string]any{"id": artworkID})
}

func (s *Server) pelicanChecks(w http.ResponseWriter, r *http.Request) {
	isAdmin, ok := s.requirePelicanViewer(w, r)
	if !ok {
		return
	}
	config, err := loadPelicanCheckConfig(r.Context(), s.portalDB)
	if err != nil {
		s.pelicanError(w, err)
		return
	}
	var latestRunID, activeRunID int64
	var lastRunAt *time.Time
	var artwork *pelicanArtwork
	artworks := []pelicanArtwork{}
	err = s.portalDB.QueryRowContext(r.Context(), `SELECT coalesce(max(id),0),coalesce(max(id) FILTER (WHERE state='running'),0),max(started_at) FROM portal_pelican_check_runs`).Scan(&latestRunID, &activeRunID, &lastRunAt)
	if err != nil {
		s.pelicanError(w, err)
		return
	}
	running := 0
	if activeRunID > 0 {
		running = 1
	}
	rows, queryErr := s.portalDB.QueryContext(r.Context(), `SELECT id,group_id,group_name,api_key_id,api_key_name,started_at,finished_at,latency_ms,html,account_id,account_name FROM portal_pelican_check_runs WHERE state='succeeded' AND html<>'' ORDER BY finished_at DESC LIMIT 6`)
	if queryErr != nil {
		s.pelicanError(w, queryErr)
		return
	}
	for rows.Next() {
		var item pelicanArtwork
		var accountID sql.NullInt64
		var accountName string
		if err = rows.Scan(&item.ID, &item.GroupID, &item.GroupName, &item.APIKeyID, &item.APIKeyName, &item.StartedAt, &item.FinishedAt, &item.LatencyMs, &item.HTML, &accountID, &accountName); err != nil {
			rows.Close()
			s.pelicanError(w, err)
			return
		}
		if isAdmin {
			if accountID.Valid && accountID.Int64 > 0 {
				value := accountID.Int64
				item.AccountID = &value
			}
			item.AccountName = accountName
		}
		artworks = append(artworks, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		s.pelicanError(w, err)
		return
	}
	if err = rows.Close(); err != nil {
		s.pelicanError(w, err)
		return
	}
	if isAdmin {
		if core, ok := s.core.(marketCore); ok {
			backfillPelicanArtworkAccounts(r.Context(), core, s.portalDB, artworks)
		}
	}
	if len(artworks) > 0 {
		artwork = &artworks[0]
	}
	var nextRunAt *time.Time
	if config.Enabled {
		nextRunAt = config.NextRunAt
	}
	schedulerHealthy := config.SchedulerSeenAt != nil && time.Since(*config.SchedulerSeenAt) < time.Minute
	if !isAdmin {
		config.Model, config.Prompt, config.ReasoningEffort = "", "", ""
	}
	w.Header().Set("Cache-Control", "no-store")
	writeSuccess(w, map[string]any{
		"interval_minutes":  30,
		"generated_at":      time.Now().UTC(),
		"last_run_at":       lastRunAt,
		"next_run_at":       nextRunAt,
		"scheduler_healthy": schedulerHealthy,
		"prompt":            config.Prompt,
		"config":            config,
		"summary":           map[string]any{"running": running},
		"latest_run_id":     latestRunID,
		"active_run_id":     activeRunID,
		"artworks":          artworks,
		"latest_artwork":    artwork,
	})
}

func (s *Server) pelicanCheckItem(ctx context.Context, config pelicanCheckConfig) (pelicanCheckItem, error) {
	item := pelicanCheckItem{GroupID: *config.GroupID, GroupName: config.GroupName, APIKeyID: *config.APIKeyID, APIKeyName: config.APIKeyName, State: "none", Timeline: []pelicanTimelinePoint{}}
	var state, runError sql.NullString
	var startedAt, finishedAt sql.NullTime
	var latency sql.NullInt64
	err := s.portalDB.QueryRowContext(ctx, `SELECT state,started_at,finished_at,latency_ms,error,attempt,progress FROM portal_pelican_check_runs WHERE group_id=$1 AND api_key_id=$2 ORDER BY started_at DESC,id DESC LIMIT 1`, item.GroupID, item.APIKeyID).Scan(&state, &startedAt, &finishedAt, &latency, &runError, &item.Attempt, &item.Progress)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return item, err
	}
	if state.Valid {
		item.State = state.String
	}
	if startedAt.Valid {
		value := startedAt.Time
		item.StartedAt = &value
	}
	if finishedAt.Valid {
		value := finishedAt.Time
		item.FinishedAt = &value
	}
	item.LatencyMs = latency.Int64
	item.Error = runError.String
	// The summary above spans all Keys, so the 24-hour timeline must use the
	// same scope. Keep the headline state tied to the currently configured Key,
	// but retain historical points when an administrator rotates that Key.
	rows, err := s.portalDB.QueryContext(ctx, `SELECT id,coalesce(api_key_id,0),api_key_name,group_name,state,started_at,finished_at,latency_ms,error,attempt,progress FROM portal_pelican_check_runs WHERE started_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours' ORDER BY started_at,id`)
	if err != nil {
		return item, err
	}
	defer rows.Close()
	for rows.Next() {
		var point pelicanTimelinePoint
		var finished sql.NullTime
		if err = rows.Scan(&point.ID, &point.APIKeyID, &point.APIKeyName, &point.GroupName, &point.State, &point.StartedAt, &finished, &point.LatencyMs, &point.Error, &point.Attempt, &point.Progress); err != nil {
			return item, err
		}
		if finished.Valid {
			value := finished.Time
			point.FinishedAt = &value
		}
		item.Timeline = append(item.Timeline, point)
	}
	return item, rows.Err()
}
