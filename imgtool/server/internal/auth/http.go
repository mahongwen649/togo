package auth

import (
	"encoding/json"
	"errors"
	"net/http"
)

const CookieName = "imgtool_session"

type HTTPHandlers struct {
	Service       *Service
	SecureCookies bool
}

func (h HTTPHandlers) Login(w http.ResponseWriter, r *http.Request, ok func(http.ResponseWriter, any), fail func(http.ResponseWriter, int, string, string, string)) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		fail(w, http.StatusBadRequest, "auth", "invalid_json", "登录请求格式错误")
		return
	}
	session, user, err := h.Service.Login(input.Username, input.Password)
	if err != nil {
		fail(w, http.StatusUnauthorized, "auth", "invalid_credentials", "用户名或密码不正确")
		return
	}
	http.SetCookie(w, h.cookie(session.ID, int(h.Service.opts.SessionTTL.Seconds())))
	ok(w, map[string]any{"user": user})
}

func (h HTTPHandlers) Me(w http.ResponseWriter, r *http.Request, ok func(http.ResponseWriter, any), fail func(http.ResponseWriter, int, string, string, string)) {
	sessionID, err := readSessionCookie(r)
	if err != nil {
		fail(w, http.StatusUnauthorized, "auth", "unauthenticated", "请先登录")
		return
	}
	user, err := h.Service.UserForSession(sessionID)
	if err != nil {
		fail(w, http.StatusUnauthorized, "auth", "unauthenticated", "请先登录")
		return
	}
	ok(w, map[string]any{"user": user})
}

func (h HTTPHandlers) Logout(w http.ResponseWriter, r *http.Request, ok func(http.ResponseWriter, any), _ func(http.ResponseWriter, int, string, string, string)) {
	if sessionID, err := readSessionCookie(r); err == nil {
		_ = h.Service.Logout(sessionID)
	}
	http.SetCookie(w, h.cookie("", -1))
	ok(w, map[string]bool{"loggedOut": true})
}

func (h HTTPHandlers) SetSessionCookie(w http.ResponseWriter, sessionID string, maxAge int) {
	http.SetCookie(w, h.cookie(sessionID, maxAge))
}

func (h HTTPHandlers) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.SecureCookies,
	}
}

func readSessionCookie(r *http.Request) (string, error) {
	cookie, err := r.Cookie(CookieName)
	if err != nil || cookie.Value == "" {
		return "", errors.New("missing session cookie")
	}
	return cookie.Value, nil
}
