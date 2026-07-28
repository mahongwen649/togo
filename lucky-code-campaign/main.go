package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed web/*
var webFiles embed.FS

const basePath = "/gift"

const deviceCookieName = "gift_device"

type App struct {
	store         *Store
	adminPassword string
	secureCookies bool
}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

func main() {
	addr := envOr("ADDR", ":8090")
	dbPath := envOr("DB_PATH", filepath.Join("data", "activity.db"))
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		log.Fatal("必须设置 ADMIN_PASSWORD 环境变量")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}
	store, err := OpenStore(dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer store.Close()

	app := &App{
		store:         store,
		adminPassword: adminPassword,
		secureCookies: strings.EqualFold(os.Getenv("COOKIE_SECURE"), "true"),
	}
	server := &http.Server{
		Addr:              addr,
		Handler:           app.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("活动服务已启动: http://localhost%s", addr)
	log.Fatal(server.ListenAndServe())
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	webRoot, _ := fs.Sub(webFiles, "web")
	mux.Handle("GET "+basePath+"/assets/", http.StripPrefix(basePath+"/assets/", http.FileServer(http.FS(webRoot))))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, basePath+"/", http.StatusTemporaryRedirect)
	})
	mux.HandleFunc("GET "+basePath, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, basePath+"/", http.StatusPermanentRedirect)
	})
	mux.HandleFunc("GET "+basePath+"/{$}", serveEmbedded("web/index.html"))
	mux.HandleFunc("GET "+basePath+"/admin", serveEmbedded("web/admin.html"))
	mux.HandleFunc("GET "+basePath+"/api/status", a.handleStatus)
	mux.HandleFunc("POST "+basePath+"/api/claim", a.handleClaim)
	mux.HandleFunc("POST "+basePath+"/api/admin/login", a.handleAdminLogin)
	mux.HandleFunc("POST "+basePath+"/api/admin/logout", a.handleAdminLogout)
	mux.Handle("GET "+basePath+"/api/admin/stats", a.requireAdmin(http.HandlerFunc(a.handleAdminStats)))
	mux.Handle("POST "+basePath+"/api/admin/codes/import", a.requireAdmin(http.HandlerFunc(a.handleCodeImport)))
	mux.Handle("GET "+basePath+"/api/admin/claims", a.requireAdmin(http.HandlerFunc(a.handleClaims)))
	mux.Handle("GET "+basePath+"/api/admin/claims/export", a.requireAdmin(http.HandlerFunc(a.handleClaimsExport)))
	return securityHeaders(mux)
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	stats, err := a.store.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "暂时无法读取奖池", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"remaining": stats.Remaining, "available": stats.Remaining > 0})
}

func (a *App) handleClaim(w http.ResponseWriter, r *http.Request) {
	var input struct{}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式不正确", "bad_request")
		return
	}
	deviceID, err := a.deviceID(w, r)
	if err != nil {
		log.Printf("创建设备标识失败: %v", err)
		writeError(w, http.StatusInternalServerError, "领取失败，请稍后再试", "internal_error")
		return
	}
	claim, _, err := a.store.ClaimRandom(r.Context(), deviceID)
	if errors.Is(err, ErrPoolEmpty) {
		writeError(w, http.StatusGone, "本轮兑换码已全部领完", "pool_empty")
		return
	}
	var cooldownErr *ClaimCooldownError
	if errors.As(err, &cooldownErr) {
		retryAfter := int(cooldownErr.RetryAfter.Round(time.Second).Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error": "每台设备 1 分钟只能抢一次，请稍后再来",
			"code":  "claim_cooldown", "retryAfterSeconds": retryAfter,
		})
		return
	}
	if err != nil {
		log.Printf("领取失败: %v", err)
		writeError(w, http.StatusInternalServerError, "领取失败，请稍后再试", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"claim": claim})
}

func (a *App) deviceID(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie(deviceCookieName); err == nil {
		if raw, decodeErr := base64.RawURLEncoding.DecodeString(cookie.Value); decodeErr == nil && len(raw) == 32 {
			sum := sha256.Sum256(raw)
			return "设备-" + strings.ToUpper(hex.EncodeToString(sum[:6])), nil
		}
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name: deviceCookieName, Value: base64.RawURLEncoding.EncodeToString(raw), Path: basePath,
		HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteLaxMode,
		MaxAge: 365 * 24 * 60 * 60,
	})
	sum := sha256.Sum256(raw)
	return "设备-" + strings.ToUpper(hex.EncodeToString(sum[:6])), nil
}

func (a *App) handleAdminLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式不正确", "bad_request")
		return
	}
	if subtleStringCompare(input.Password, a.adminPassword) == false {
		time.Sleep(250 * time.Millisecond)
		writeError(w, http.StatusUnauthorized, "管理员密码不正确", "unauthorized")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: "activity_admin", Value: a.authToken(), Path: basePath,
		HttpOnly: true, Secure: a.secureCookies, SameSite: http.SameSiteStrictMode,
		MaxAge: 12 * 60 * 60,
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleAdminLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "activity_admin", Value: "", Path: basePath, MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	stats, err := a.store.Stats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取统计失败", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (a *App) handleCodeImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	var input struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "导入内容格式不正确", "bad_request")
		return
	}
	codes, err := parseCodes(input.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "invalid_codes")
		return
	}
	added, duplicates, err := a.store.ImportCodes(r.Context(), codes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "import_failed")
		return
	}
	stats, _ := a.store.Stats(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"added": added, "duplicates": duplicates, "stats": stats})
}

func (a *App) handleClaims(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	page := intQuery(r, "page", 1)
	pageSize := intQuery(r, "pageSize", 20)
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	claims, total, err := a.store.Claims(r.Context(), query, pageSize, (page-1)*pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取领取记录失败", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": claims, "total": total, "page": page, "pageSize": pageSize})
}

func (a *App) handleClaimsExport(w http.ResponseWriter, r *http.Request) {
	claims, _, err := a.store.Claims(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")), 1_000_000, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "导出失败", "internal_error")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="claims.csv"`)
	_, _ = w.Write([]byte("\xEF\xBB\xBF"))
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"参与标识", "兑换码", "领取时间"})
	for _, claim := range claims {
		_ = writer.Write([]string{claim.Email, claim.Code, claim.ClaimedAt.Local().Format("2006-01-02 15:04:05")})
	}
	writer.Flush()
}

func (a *App) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("activity_admin")
		if err != nil || !subtleStringCompare(cookie.Value, a.authToken()) {
			writeError(w, http.StatusUnauthorized, "请先登录管理员页面", "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) authToken() string {
	mac := hmac.New(sha256.New, []byte(a.adminPassword))
	mac.Write([]byte("lucky-code-admin-session-v1"))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func serveEmbedded(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		content, err := webFiles.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(content)
	}
}

func parseCodes(content string) ([]string, error) {
	content = strings.TrimSpace(strings.TrimPrefix(content, "\uFEFF"))
	if content == "" {
		return nil, errors.New("请粘贴或上传兑换码")
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	codes := make([]string, 0, len(lines))
	for index, line := range lines {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		if index == 0 && strings.EqualFold(strings.TrimSpace(strings.Split(line, ",")[0]), "code") {
			continue
		}
		if strings.Contains(line, ",") {
			reader := csv.NewReader(strings.NewReader(line))
			record, err := reader.Read()
			if err != nil || len(record) == 0 {
				return nil, fmt.Errorf("第 %d 行 CSV 格式不正确", index+1)
			}
			line = strings.TrimSpace(record[0])
		}
		if line != "" {
			codes = append(codes, line)
		}
	}
	if len(codes) == 0 {
		return nil, errors.New("没有找到可导入的兑换码")
	}
	return codes, nil
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 6<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("请求只能包含一个 JSON 对象")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, apiError{Error: message, Code: code})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func subtleStringCompare(left, right string) bool {
	return hmac.Equal([]byte(left), []byte(right))
}

func intQuery(r *http.Request, key string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	return value
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
