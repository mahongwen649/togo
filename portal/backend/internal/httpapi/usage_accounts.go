package httpapi

import (
	"net/http"
	"net/url"
	"strings"
)

// usageSearchAccounts exposes only the display fields needed by usage filters.
// Unlike account checks, usage includes every Core platform.
func (s *Server) usageSearchAccounts(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok {
		return
	}
	core, accessToken, ok := s.accountCheckCore(w, r)
	if !ok {
		return
	}
	keyword := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(keyword) > 200 {
		writeAPIError(w, http.StatusBadRequest, "INVALID_SEARCH", "搜索条件过长")
		return
	}
	type accountOption struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	result := struct {
		Items []accountOption `json:"items"`
	}{Items: []accountOption{}}
	if keyword != "" {
		query := url.Values{"page": {"1"}, "page_size": {"20"}, "search": {keyword}}
		if err := accountCheckRequest(r.Context(), core, http.MethodGet, "/api/v1/admin/accounts?"+query.Encode(), nil, &result, accessToken); err != nil {
			s.writeRentalError(w, err)
			return
		}
	}
	if result.Items == nil {
		result.Items = []accountOption{}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeSuccess(w, result.Items)
}
