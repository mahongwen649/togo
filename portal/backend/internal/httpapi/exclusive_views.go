package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

type exclusiveAccountCache struct {
	Value exclusiveAccount
	Until time.Time
}

func (s *Server) cachedExclusiveAccount(ctx context.Context, id int64) exclusiveAccount {
	if item, ok := s.exclusiveAccounts.Load(id); ok {
		c := item.(exclusiveAccountCache)
		if time.Now().Before(c.Until) {
			return c.Value
		}
	}
	a := s.exclusiveAccountView(ctx, id)
	ttl := 15 * time.Second
	if a.Error != "" {
		ttl = 3 * time.Second
	}
	s.exclusiveAccounts.Store(id, exclusiveAccountCache{a, time.Now().Add(ttl)})
	return a
}
func (s *Server) exclusiveList(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.exclusiveUser(w, r)
	if !ok {
		return
	}
	admin := strings.HasPrefix(r.URL.Path, "/api/v1/admin/")
	if admin && user.Role != "admin" {
		writeAPIError(w, 403, "FORBIDDEN", "需要管理员权限")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		writeAPIError(w, 400, "INVALID_PAGE", "页码无效")
		return
	}
	var total int
	if err := s.portalDB.QueryRowContext(r.Context(), `SELECT count(*) FROM portal_exclusive_channels WHERE ($1 OR state NOT IN ('unpublished','provisioning','error'))`, admin).Scan(&total); err != nil {
		s.exclusiveError(w, err)
		return
	}
	rows, err := s.portalDB.QueryContext(r.Context(), `SELECT `+exclusiveChannelColumns+` FROM portal_exclusive_channels WHERE ($1 OR state NOT IN ('unpublished','provisioning','error')) ORDER BY id LIMIT 20 OFFSET $2`, admin, (page-1)*20)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	channels := []exclusiveChannel{}
	for rows.Next() {
		c, e := scanExclusiveChannel(rows)
		if e != nil {
			rows.Close()
			s.exclusiveError(w, e)
			return
		}
		channels = append(channels, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	for i := range channels {
		c := &channels[i]
		c.PageTest, err = s.exclusivePageMetadata(r.Context(), c.ID, false)
		if err != nil {
			s.exclusiveError(w, err)
			return
		}
		l, e := scanExclusiveLease(s.portalDB.QueryRowContext(r.Context(), `SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE channel_id=$1 AND state<>'closed'`, c.ID))
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			s.exclusiveError(w, e)
			return
		}
		if e == nil {
			l.Mine = l.UserID == user.ID
			if !admin {
				l.UserID = 0
			}
			if !l.Mine && !admin {
				l.Error = ""
				l.Cost = 0
				l.Requests = 0
				l.Tokens = 0
			}
			c.Lease = &l
		}
		c.Accounts = make([]exclusiveAccount, len(c.AccountIDs))
		for j, accountID := range c.AccountIDs {
			c.Accounts[j].PageTest, err = s.exclusiveAccountTestMetadata(r.Context(), accountID, false)
			if err != nil {
				s.exclusiveError(w, err)
				return
			}
		}
		if admin && c.PendingConfig != nil {
			for _, accountID := range c.PendingConfig.AccountIDs {
				if !slices.Contains(c.AccountIDs, accountID) {
					c.PendingAccounts = append(c.PendingAccounts, exclusiveAccount{ID: accountID})
				}
			}
		}
	}
	group, _ := errgroup.WithContext(r.Context())
	group.SetLimit(6)
	for i := range channels {
		for j, aid := range channels[i].AccountIDs {
			i, j, aid := i, j, aid
			group.Go(func() error {
				a := s.cachedExclusiveAccount(r.Context(), aid)
				a.PageTest = channels[i].Accounts[j].PageTest
				if !admin {
					a.Name = fmt.Sprintf("账号 %02d", j+1)
				}
				channels[i].Accounts[j] = a
				return nil
			})
		}
		for j, pending := range channels[i].PendingAccounts {
			i, j, aid := i, j, pending.ID
			group.Go(func() error {
				channels[i].PendingAccounts[j] = s.cachedExclusiveAccount(r.Context(), aid)
				return nil
			})
		}
	}
	_ = group.Wait()
	writeSuccess(w, map[string]any{"items": channels, "page": page, "pages": (total + 19) / 20, "total": total, "server_time": time.Now().UTC()})
}
func (s *Server) exclusiveMine(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.exclusiveUser(w, r)
	if !ok {
		return
	}
	l, err := scanExclusiveLease(s.portalDB.QueryRowContext(r.Context(), `SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE user_id=$1 AND state<>'closed'`, user.ID))
	if errors.Is(err, sql.ErrNoRows) {
		writeSuccess(w, map[string]any{"lease": nil, "channel": nil, "balance": user.Balance, "server_time": time.Now().UTC()})
		return
	}
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	c, err := scanExclusiveChannel(s.portalDB.QueryRowContext(r.Context(), `SELECT `+exclusiveChannelColumns+` FROM portal_exclusive_channels WHERE id=$1`, l.ChannelID))
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	err = s.portalDB.QueryRowContext(r.Context(), `SELECT count(*) FILTER(WHERE state='running'),count(*) FILTER(WHERE state='uncertain') FROM portal_exclusive_requests WHERE lease_id=$1`, l.ID).Scan(&l.Inflight, &l.Uncertain)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	if l.State == "active" {
		l.Key, err = s.openExclusive(l.TokenSecret)
		if err != nil {
			s.exclusiveError(w, err)
			return
		}
	}
	l.BaseURL = strings.TrimRight(s.exclusive.PublicURL, "/") + "/exclusive/v1"
	c.PageTest, err = s.exclusivePageMetadata(r.Context(), c.ID, false)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	l.Mine = true
	for i, aid := range c.AccountIDs {
		a := s.cachedExclusiveAccount(r.Context(), aid)
		a.Name = fmt.Sprintf("账号 %02d", i+1)
		a.PageTest, err = s.exclusiveAccountTestMetadata(r.Context(), aid, false)
		if err != nil {
			s.exclusiveError(w, err)
			return
		}
		c.Accounts = append(c.Accounts, a)
	}
	writeSuccess(w, map[string]any{"lease": l, "channel": c, "balance": user.Balance, "server_time": time.Now().UTC()})
}
func (s *Server) exclusiveHistory(w http.ResponseWriter, r *http.Request) {
	user, _, ok := s.exclusiveUser(w, r)
	if !ok {
		return
	}
	rows, err := s.portalDB.QueryContext(r.Context(), `SELECT `+exclusiveLeaseColumns+` FROM portal_exclusive_leases WHERE user_id=$1 AND state='closed' ORDER BY id DESC LIMIT 100`, user.ID)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	history := []exclusiveLease{}
	for rows.Next() {
		l, e := scanExclusiveLease(rows)
		if e != nil {
			rows.Close()
			s.exclusiveError(w, e)
			return
		}
		history = append(history, l)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	for i := range history {
		_ = s.portalDB.QueryRowContext(r.Context(), `SELECT name FROM portal_exclusive_channels WHERE id=$1`, history[i].ChannelID).Scan(&history[i].ChannelName)
	}
	writeSuccess(w, map[string]any{"items": history})
}
func (s *Server) exclusiveEvents(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireActiveAdmin(w, r); !ok || !s.exclusiveAvailable(w) {
		return
	}
	rows, err := s.portalDB.QueryContext(r.Context(), `SELECT e.id,e.channel_id,c.name,e.actor_id,e.action,e.created_at FROM portal_exclusive_events e JOIN portal_exclusive_channels c ON c.id=e.channel_id ORDER BY e.id DESC LIMIT 100`)
	if err != nil {
		s.exclusiveError(w, err)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, cid, actor int64
		var name, action string
		var at time.Time
		if err = rows.Scan(&id, &cid, &name, &actor, &action, &at); err != nil {
			s.exclusiveError(w, err)
			return
		}
		items = append(items, map[string]any{"id": id, "channel_id": cid, "channel_name": name, "actor_id": actor, "action": action, "created_at": at})
	}
	if err = rows.Err(); err != nil {
		s.exclusiveError(w, err)
		return
	}
	writeSuccess(w, map[string]any{"items": items})
}
