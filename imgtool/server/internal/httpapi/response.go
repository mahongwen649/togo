package httpapi

import (
	"encoding/json"
	"net/http"
)

type Error struct {
	Source string `json:"source"`
	Code   string `json:"code"`
	Message string `json:"message"`
}

func OK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"data": data,
	})
}

func Fail(w http.ResponseWriter, status int, source string, code string, message string) {
	writeJSON(w, status, map[string]any{
		"ok": false,
		"error": Error{
			Source:  source,
			Code:    code,
			Message: message,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
