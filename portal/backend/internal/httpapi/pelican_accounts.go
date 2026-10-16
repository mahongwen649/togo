package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

const pelicanMaxAttempts = 3
const pelicanAttemptTimeout = 6 * time.Minute

type pelicanAccountPrompter interface {
	PelicanAccountPromptRequest(context.Context, int64, string, string, string, ...string) (coreclient.Response, error)
}

type pelicanAccount struct {
	ID                     int64      `json:"id"`
	Name                   string     `json:"name"`
	Platform               string     `json:"platform"`
	Type                   string     `json:"type"`
	Status                 string     `json:"status"`
	Schedulable            bool       `json:"schedulable"`
	GroupIDs               []int64    `json:"group_ids"`
	RateLimitResetAt       *time.Time `json:"rate_limit_reset_at"`
	OverloadUntil          *time.Time `json:"overload_until"`
	TempUnschedulableUntil *time.Time `json:"temp_unschedulable_until"`
	ExpiresAt              *time.Time `json:"expires_at"`
	AutoPauseOnExpired     bool       `json:"auto_pause_on_expired"`
	Credentials            struct {
		ModelMapping map[string]string `json:"model_mapping"`
	} `json:"credentials"`
}

func listPelicanAccounts(ctx context.Context, core marketCore, groupID, runID int64, model string) ([]pelicanAccount, error) {
	accounts := []pelicanAccount{}
	seen := map[int64]bool{}
	for page := 1; ; page++ {
		response, err := core.AdminMarketRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/admin/accounts?platform=openai&group=%d&page=%d&page_size=100", groupID, page), nil, "")
		if err != nil {
			return nil, errors.New("无法读取绘图账号列表")
		}
		var result struct {
			Items []pelicanAccount `json:"items"`
			Pages int              `json:"pages"`
		}
		if err = decodePelicanCoreData(response.Body, &result); err != nil {
			return nil, errors.New("绘图账号列表无效")
		}
		for _, account := range result.Items {
			if seen[account.ID] || !account.eligible(groupID, time.Now(), model) {
				continue
			}
			seen[account.ID] = true
			accounts = append(accounts, account)
		}
		if page >= result.Pages {
			break
		}
	}
	if len(accounts) == 0 {
		return nil, errors.New("分组中暂无支持绘图模型的可用账号")
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].ID < accounts[j].ID })
	// Rotate the first candidate between runs; never reuse an account in a run.
	offset := int(runID % int64(len(accounts)))
	return append(accounts[offset:], accounts[:offset]...), nil
}

func (a pelicanAccount) eligible(groupID int64, now time.Time, models ...string) bool {
	model := exclusivePageModel
	if len(models) > 0 {
		model = models[0]
	}
	if a.ID <= 0 || a.Platform != "openai" || (a.Type != "apikey" && a.Type != "oauth") || a.Status != "active" || !a.Schedulable {
		return false
	}
	inGroup := false
	for _, id := range a.GroupIDs {
		if id == groupID {
			inGroup = true
		}
	}
	if !inGroup {
		return false
	}
	for _, until := range []*time.Time{a.RateLimitResetAt, a.OverloadUntil, a.TempUnschedulableUntil} {
		if until != nil && now.Before(*until) {
			return false
		}
	}
	if a.AutoPauseOnExpired && a.ExpiresAt != nil && !now.Before(*a.ExpiresAt) {
		return false
	}
	if len(a.Credentials.ModelMapping) == 0 {
		return true
	}
	for pattern, target := range a.Credentials.ModelMapping {
		if target != "" && (pattern == model || (strings.HasSuffix(pattern, "*") && strings.HasPrefix(model, strings.TrimSuffix(pattern, "*")))) {
			return true
		}
	}
	return false
}

func requestPelicanAccountPage(ctx context.Context, core pelicanAccountPrompter, accountID int64, requestID, model, prompt string, efforts ...string) (string, error) {
	response, err := core.PelicanAccountPromptRequest(ctx, accountID, model, prompt+"\n仅返回完整、自包含的 HTML 页面，包含内联 SVG、CSS 和必要的 JavaScript。不要解释、不要 Markdown 代码围栏，不引用外部资源。", requestID, efforts...)
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", errors.New("绘图账号检查请求失败")
	}
	result := parseAccountCheck(response.Body)
	if result.Error != "" {
		return "", errors.New(result.Error)
	}
	if result.HTML == "" {
		return "", errors.New("回复中未找到可展示的 HTML 或 SVG 页面")
	}
	return result.HTML, nil
}
