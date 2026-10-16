package httpapi

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"
)

func (s *Server) exclusiveRefreshUsage(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.exclusiveUser(w, r)
	if !ok {
		return
	}
	cid, e1 := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	aid, e2 := strconv.ParseInt(r.PathValue("accountID"), 10, 64)
	if e1 != nil || e2 != nil || cid <= 0 || aid <= 0 {
		writeAPIError(w, 400, "INVALID_ACCOUNT", "通道或账号无效")
		return
	}
	c, err := scanExclusiveChannel(s.portalDB.QueryRowContext(r.Context(), `SELECT `+exclusiveChannelColumns+` FROM portal_exclusive_channels WHERE id=$1`, cid))
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	if (user.Role != "admin" && slices.Contains([]string{"unpublished", "provisioning", "error"}, c.State)) || !slices.Contains(c.AccountIDs, aid) {
		writeAPIError(w, 404, "EXCLUSIVE_NOT_FOUND", "通道账号不存在")
		return
	}
	// Bound repeated forced upstream probes; this does not reset account quotas.
	key, now := fmt.Sprintf("refresh:%d", aid), time.Now()
	until := now.Add(15 * time.Second)
	old, loaded := s.exclusiveAccounts.LoadOrStore(key, until)
	if loaded && (now.Before(old.(time.Time)) || !s.exclusiveAccounts.CompareAndSwap(key, old, until)) {
		w.Header().Set("Retry-After", "15")
		writeAPIError(w, 429, "USAGE_REFRESH_THROTTLED", "刷新过于频繁，请稍后重试")
		return
	}
	a := s.exclusiveAccountUsage(r.Context(), aid, true)
	if a.FiveHourRemaining == nil && a.WeeklyRemaining == nil && a.Error != "" {
		writeAPIError(w, 502, "USAGE_UNAVAILABLE", "Core 用量查询失败，保留之前的数据，请稍后重试")
		return
	}
	s.exclusiveAccounts.Store(aid, exclusiveAccountCache{Value: a, Until: time.Now().Add(15 * time.Second)})
	if user.Role != "admin" {
		a.Name = fmt.Sprintf("账号 %02d", slices.Index(c.AccountIDs, aid)+1)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeSuccess(w, a)
}
