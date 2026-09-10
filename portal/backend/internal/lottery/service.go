package lottery

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jqcode/portal/backend/internal/coreclient"
)

type Problem struct {
	Status  int
	Code    string
	Message string
}

func (p *Problem) Error() string { return p.Message }

type Campaign struct {
	ID                int64      `json:"id"`
	Name              string     `json:"name"`
	Subtitle          string     `json:"subtitle"`
	RegistrationStart time.Time  `json:"registration_start"`
	DrawAt            time.Time  `json:"draw_at"`
	ParticipantLimit  int        `json:"participant_limit"`
	RandomLimit       int        `json:"random_limit"`
	RandomMin         float64    `json:"random_min"`
	RandomMax         float64    `json:"random_max"`
	RandomBudget      float64    `json:"random_budget"`
	GuaranteeAmount   float64    `json:"guarantee_amount"`
	Participants      int        `json:"participants"`
	RandomWinners     int        `json:"random_winners"`
	GuaranteedWinners int        `json:"guaranteed_winners"`
	Credited          int        `json:"credited"`
	Failed            int        `json:"failed"`
	RandomPrize       float64    `json:"random_prize"`
	GuaranteePrize    float64    `json:"guarantee_prize"`
	Status            string     `json:"status"`
	Published         bool       `json:"published"`
	DrawnAt           *time.Time `json:"drawn_at,omitempty"`
}

type Payout struct {
	ID            int64      `json:"id"`
	CampaignID    int64      `json:"campaign_id"`
	CampaignName  string     `json:"campaign_name,omitempty"`
	ParticipantID int64      `json:"participant_id"`
	CoreUserID    int64      `json:"core_user_id"`
	Email         string     `json:"email"`
	PrizeType     string     `json:"prize_type"`
	Amount        float64    `json:"amount"`
	Status        string     `json:"status"`
	ErrorMessage  string     `json:"error_message,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	CreditedAt    *time.Time `json:"credited_at,omitempty"`
}

type UserDirectory interface {
	AdminUsers(context.Context, int, int, ...coreclient.AdminUsersOption) (coreclient.UserPage, error)
}

type BalanceIssuer interface {
	AdminAddBalance(context.Context, int64, float64, string, string) error
}

type Service struct {
	db        *sql.DB
	directory UserDirectory
	issuer    BalanceIssuer
	logger    *slog.Logger
}

func New(db *sql.DB, directory UserDirectory, issuer BalanceIssuer, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{db: db, directory: directory, issuer: issuer, logger: logger}
}

func (s *Service) ListCampaigns(ctx context.Context, status string) ([]Campaign, error) {
	query := `SELECT id,name,subtitle,registration_start,draw_at,participant_limit,random_limit,random_min,random_max,random_budget,guarantee_amount,status,published,drawn_at FROM portal_lottery_campaigns`
	args := []any{}
	if strings.TrimSpace(status) != "" && status != "all" {
		query += ` WHERE status=$1`
		args = append(args, status)
	}
	query += ` ORDER BY draw_at DESC, id DESC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Campaign{}
	for rows.Next() {
		item, err := scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		if err := s.addCampaignCounts(ctx, &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Service) GetCampaign(ctx context.Context, id int64) (Campaign, error) {
	item, err := s.queryCampaign(ctx, `WHERE id=$1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Campaign{}, &Problem{http.StatusNotFound, "CAMPAIGN_NOT_FOUND", "Campaign not found"}
	}
	if err != nil {
		return Campaign{}, err
	}
	if err := s.addCampaignCounts(ctx, &item); err != nil {
		return Campaign{}, err
	}
	return item, nil
}

func (s *Service) Current(ctx context.Context) (Campaign, error) {
	item, err := s.queryCampaign(ctx, `WHERE published=true AND status IN ('scheduled','registering','drawing') ORDER BY draw_at ASC LIMIT 1`)
	if errors.Is(err, sql.ErrNoRows) {
		return Campaign{}, &Problem{http.StatusNotFound, "NO_ACTIVE_CAMPAIGN", "No active campaign"}
	}
	if err != nil {
		return Campaign{}, err
	}
	now := time.Now()
	if item.Status == "scheduled" && !now.Before(item.RegistrationStart) {
		item.Status = "registering"
	}
	if !now.Before(item.DrawAt) && item.Status != "drawing" {
		item.Status = "drawing"
	}
	if err := s.addCampaignCounts(ctx, &item); err != nil {
		return Campaign{}, err
	}
	return item, nil
}

type CampaignInput struct {
	Name              string    `json:"name"`
	Subtitle          string    `json:"subtitle"`
	RegistrationStart time.Time `json:"registration_start"`
	DrawAt            time.Time `json:"draw_at"`
	ParticipantLimit  int       `json:"participant_limit"`
	RandomLimit       int       `json:"random_limit"`
	RandomMin         float64   `json:"random_min"`
	RandomMax         float64   `json:"random_max"`
	RandomBudget      float64   `json:"random_budget"`
	GuaranteeAmount   float64   `json:"guarantee_amount"`
	Published         bool      `json:"published"`
}

func validateInput(input CampaignInput) error {
	if strings.TrimSpace(input.Name) == "" || input.RegistrationStart.IsZero() || input.DrawAt.IsZero() {
		return &Problem{http.StatusBadRequest, "INVALID_REQUEST", "Name and activity times are required"}
	}
	if !input.DrawAt.After(input.RegistrationStart) {
		return &Problem{http.StatusBadRequest, "INVALID_REQUEST", "Draw time must be after registration start"}
	}
	if input.ParticipantLimit < 1 || input.RandomLimit < 1 || input.RandomLimit > input.ParticipantLimit {
		return &Problem{http.StatusBadRequest, "INVALID_REQUEST", "Invalid participant limits"}
	}
	if input.RandomMin <= 0 || input.RandomMax < input.RandomMin || input.RandomBudget <= 0 || input.GuaranteeAmount <= 0 {
		return &Problem{http.StatusBadRequest, "INVALID_REQUEST", "Invalid amount configuration"}
	}
	average := input.RandomBudget / float64(input.RandomLimit)
	if average < input.RandomMin || average > input.RandomMax {
		return &Problem{http.StatusBadRequest, "INVALID_REQUEST", "Random budget average must be within the random amount range"}
	}
	return nil
}

func (s *Service) CreateCampaign(ctx context.Context, input CampaignInput) (Campaign, error) {
	if err := validateInput(input); err != nil {
		return Campaign{}, err
	}
	status := "draft"
	if input.Published {
		status = "scheduled"
	}
	var item Campaign
	err := s.db.QueryRowContext(ctx, `INSERT INTO portal_lottery_campaigns(name,subtitle,registration_start,draw_at,participant_limit,random_limit,random_min,random_max,random_budget,guarantee_amount,status,published) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id,name,subtitle,registration_start,draw_at,participant_limit,random_limit,random_min,random_max,random_budget,guarantee_amount,status,published,drawn_at`, strings.TrimSpace(input.Name), strings.TrimSpace(input.Subtitle), input.RegistrationStart, input.DrawAt, input.ParticipantLimit, input.RandomLimit, input.RandomMin, input.RandomMax, input.RandomBudget, input.GuaranteeAmount, status, input.Published).Scan(campaignArgs(&item)...)
	if err != nil {
		return Campaign{}, mapDBError(err)
	}
	return item, nil
}

func (s *Service) UpdateCampaign(ctx context.Context, id int64, input CampaignInput) (Campaign, error) {
	if err := validateInput(input); err != nil {
		return Campaign{}, err
	}
	var existing Campaign
	if err := s.db.QueryRowContext(ctx, `SELECT id,name,subtitle,registration_start,draw_at,participant_limit,random_limit,random_min,random_max,random_budget,guarantee_amount,status,published,drawn_at FROM portal_lottery_campaigns WHERE id=$1`, id).Scan(campaignArgs(&existing)...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Campaign{}, &Problem{http.StatusNotFound, "CAMPAIGN_NOT_FOUND", "Campaign not found"}
		}
		return Campaign{}, err
	}
	if existing.Status != "draft" && existing.Status != "scheduled" {
		return Campaign{}, &Problem{http.StatusConflict, "CAMPAIGN_LOCKED", "Campaign configuration is locked after registration starts"}
	}
	if existing.Status == "scheduled" && !time.Now().Before(existing.RegistrationStart) {
		return Campaign{}, &Problem{http.StatusConflict, "CAMPAIGN_LOCKED", "Campaign configuration is locked after registration starts"}
	}
	status := existing.Status
	if input.Published {
		status = "scheduled"
	} else {
		status = "draft"
	}
	err := s.db.QueryRowContext(ctx, `UPDATE portal_lottery_campaigns SET name=$2,subtitle=$3,registration_start=$4,draw_at=$5,participant_limit=$6,random_limit=$7,random_min=$8,random_max=$9,random_budget=$10,guarantee_amount=$11,status=$12,published=$13,updated_at=CURRENT_TIMESTAMP WHERE id=$1 RETURNING id,name,subtitle,registration_start,draw_at,participant_limit,random_limit,random_min,random_max,random_budget,guarantee_amount,status,published,drawn_at`, id, strings.TrimSpace(input.Name), strings.TrimSpace(input.Subtitle), input.RegistrationStart, input.DrawAt, input.ParticipantLimit, input.RandomLimit, input.RandomMin, input.RandomMax, input.RandomBudget, input.GuaranteeAmount, status, input.Published).Scan(campaignArgs(&existing)...)
	if err != nil {
		return Campaign{}, mapDBError(err)
	}
	return existing, nil
}

func (s *Service) CancelCampaign(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx, `UPDATE portal_lottery_campaigns SET status='cancelled',published=false,updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND status IN ('draft','scheduled','registering')`, id)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return &Problem{http.StatusConflict, "CAMPAIGN_NOT_CANCELLABLE", "Campaign cannot be cancelled"}
	}
	return nil
}

func (s *Service) Register(ctx context.Context, email string) (map[string]any, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, &Problem{http.StatusBadRequest, "INVALID_EMAIL", "A valid email is required"}
	}
	campaign, err := s.Current(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if now.Before(campaign.RegistrationStart) {
		return nil, &Problem{http.StatusConflict, "REGISTRATION_NOT_STARTED", "Registration has not started"}
	}
	if !now.Before(campaign.DrawAt) {
		return nil, &Problem{http.StatusConflict, "REGISTRATION_CLOSED", "Registration is closed"}
	}
	if s.directory == nil {
		return nil, errors.New("Core user directory is not configured")
	}
	page, err := s.directory.AdminUsers(ctx, 1, 20, coreclient.AdminUsersSearch(email))
	if err != nil {
		return nil, err
	}
	var user coreclient.User
	for _, candidate := range page.Items {
		if strings.EqualFold(strings.TrimSpace(candidate.Email), email) {
			user = candidate
			break
		}
	}
	if user.ID <= 0 || user.Status != "active" {
		return nil, &Problem{http.StatusNotFound, "ACCOUNT_NOT_FOUND", "Registered active account not found"}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var lockedCampaignID int64
	var lockedStatus string
	var lockedPublished bool
	var lockedRegistrationStart, lockedDrawAt time.Time
	var lockedParticipantLimit int
	if err := tx.QueryRowContext(ctx, `SELECT id,status,published,registration_start,draw_at,participant_limit FROM portal_lottery_campaigns WHERE id=$1 FOR UPDATE`, campaign.ID).Scan(&lockedCampaignID, &lockedStatus, &lockedPublished, &lockedRegistrationStart, &lockedDrawAt, &lockedParticipantLimit); err != nil {
		return nil, err
	}
	now = time.Now()
	if !lockedPublished || (lockedStatus != "scheduled" && lockedStatus != "registering") || !now.Before(lockedDrawAt) {
		return nil, &Problem{http.StatusConflict, "REGISTRATION_CLOSED", "Registration is closed"}
	}
	if now.Before(lockedRegistrationStart) {
		return nil, &Problem{http.StatusConflict, "REGISTRATION_NOT_STARTED", "Registration has not started"}
	}
	if lockedStatus == "scheduled" {
		if _, err := tx.ExecContext(ctx, `UPDATE portal_lottery_campaigns SET status='registering',updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND status='scheduled'`, lockedCampaignID); err != nil {
			return nil, err
		}
	}
	var participantCount int
	var alreadyRegistered bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM portal_lottery_participants WHERE campaign_id=$1 AND LOWER(email)=LOWER($2))`, lockedCampaignID, email).Scan(&alreadyRegistered); err != nil {
		return nil, err
	}
	if alreadyRegistered {
		return nil, &Problem{http.StatusConflict, "ALREADY_REGISTERED", "This account has already registered for the campaign"}
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM portal_lottery_participants WHERE campaign_id=$1`, lockedCampaignID).Scan(&participantCount); err != nil {
		return nil, err
	}
	if participantCount >= lockedParticipantLimit {
		return nil, &Problem{http.StatusConflict, "PARTICIPANT_LIMIT_REACHED", "This campaign has reached its participant limit"}
	}
	var participantID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO portal_lottery_participants(campaign_id,core_user_id,email) VALUES($1,$2,$3) RETURNING id`, campaign.ID, user.ID, email).Scan(&participantID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, &Problem{http.StatusConflict, "ALREADY_REGISTERED", "This account has already registered for the campaign"}
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return map[string]any{"campaign_id": campaign.ID, "participant_id": participantID, "email": maskEmail(email), "status": "waiting"}, nil
}

func (s *Service) CurrentResult(ctx context.Context, email string) (Payout, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return Payout{}, &Problem{http.StatusBadRequest, "INVALID_EMAIL", "A valid email is required"}
	}
	var item Payout
	err := s.db.QueryRowContext(ctx, `SELECT p.id,p.campaign_id,c.name,p.participant_id,p.core_user_id,p.email,p.prize_type,p.amount,p.status,p.error_message,p.created_at,p.credited_at FROM portal_lottery_payouts p JOIN portal_lottery_campaigns c ON c.id=p.campaign_id WHERE p.campaign_id=(SELECT c2.id FROM portal_lottery_campaigns c2 JOIN portal_lottery_participants u2 ON u2.campaign_id=c2.id WHERE c2.status='completed' AND LOWER(u2.email)=$1 ORDER BY c2.draw_at DESC LIMIT 1) AND LOWER(p.email)=$1`, email).Scan(payoutArgs(&item)...)
	if errors.Is(err, sql.ErrNoRows) {
		return Payout{}, &Problem{http.StatusNotFound, "RESULT_NOT_AVAILABLE", "The result is not available yet"}
	}
	return item, err
}

func (s *Service) ResultsByEmail(ctx context.Context, email string) ([]Payout, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, &Problem{http.StatusBadRequest, "INVALID_EMAIL", "A valid email is required"}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT p.id,p.campaign_id,c.name,p.participant_id,p.core_user_id,p.email,p.prize_type,p.amount,p.status,p.error_message,p.created_at,p.credited_at FROM portal_lottery_payouts p JOIN portal_lottery_campaigns c ON c.id=p.campaign_id WHERE c.status='completed' AND LOWER(p.email)=$1 ORDER BY c.draw_at DESC,c.id DESC,p.id DESC`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Payout{}
	for rows.Next() {
		var item Payout
		if err := rows.Scan(payoutArgs(&item)...); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Service) History(ctx context.Context, page, pageSize int) ([]Campaign, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM portal_lottery_campaigns WHERE status='completed'`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,subtitle,registration_start,draw_at,participant_limit,random_limit,random_min,random_max,random_budget,guarantee_amount,status,published,drawn_at FROM portal_lottery_campaigns WHERE status='completed' ORDER BY draw_at DESC,id DESC LIMIT $1 OFFSET $2`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []Campaign{}
	for rows.Next() {
		item, err := scanCampaign(rows)
		if err != nil {
			return nil, 0, err
		}
		if err := s.addCampaignCounts(ctx, &item); err != nil {
			return nil, 0, err
		}
		result = append(result, item)
	}
	return result, total, rows.Err()
}

func (s *Service) Payouts(ctx context.Context, status string, campaignID int64) ([]Payout, error) {
	query := `SELECT p.id,p.campaign_id,c.name,p.participant_id,p.core_user_id,p.email,p.prize_type,p.amount,p.status,p.error_message,p.created_at,p.credited_at FROM portal_lottery_payouts p JOIN portal_lottery_campaigns c ON c.id=p.campaign_id WHERE 1=1`
	args := []any{}
	if status != "" && status != "all" {
		args = append(args, status)
		query += fmt.Sprintf(" AND p.status=$%d", len(args))
	}
	if campaignID > 0 {
		args = append(args, campaignID)
		query += fmt.Sprintf(" AND p.campaign_id=$%d", len(args))
	}
	query += ` ORDER BY p.created_at DESC,p.id DESC LIMIT 1000`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Payout{}
	for rows.Next() {
		var item Payout
		if err := rows.Scan(payoutArgs(&item)...); err != nil {
			return nil, err
		}
		item.Email = maskEmail(item.Email)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Service) DrawDue(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM portal_lottery_campaigns WHERE published=true AND status IN ('scheduled','registering','drawing') AND draw_at <= CURRENT_TIMESTAMP ORDER BY draw_at ASC`)
	if err != nil {
		return err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := s.draw(ctx, id); err != nil {
			s.logger.Error("lottery draw failed", "campaign_id", id, "error", err)
		}
	}
	return s.RetryPayouts(ctx)
}

func (s *Service) draw(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var c Campaign
	if err := tx.QueryRowContext(ctx, `SELECT id,name,subtitle,registration_start,draw_at,participant_limit,random_limit,random_min,random_max,random_budget,guarantee_amount,status,published,drawn_at FROM portal_lottery_campaigns WHERE id=$1 FOR UPDATE`, id).Scan(campaignArgs(&c)...); err != nil {
		return err
	}
	if c.Status == "completed" || c.Status == "cancelled" {
		return tx.Commit()
	}
	if !c.Published || (c.Status != "scheduled" && c.Status != "registering" && c.Status != "drawing") {
		return &Problem{http.StatusConflict, "CAMPAIGN_NOT_DRAWABLE", "Only a published active campaign can be drawn"}
	}
	var participants []participant
	rows, err := tx.QueryContext(ctx, `SELECT id,core_user_id,email FROM portal_lottery_participants WHERE campaign_id=$1 ORDER BY created_at,id`, id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var p participant
		if err := rows.Scan(&p.ID, &p.CoreUserID, &p.Email); err != nil {
			rows.Close()
			return err
		}
		participants = append(participants, p)
	}
	rows.Close()
	winners := c.RandomLimit
	if len(participants) < winners {
		winners = len(participants)
	}
	if len(participants) > 1 {
		for i := len(participants) - 1; i > 0; i-- {
			j, err := randomInt(i + 1)
			if err != nil {
				return err
			}
			participants[i], participants[j] = participants[j], participants[i]
		}
	}
	amounts, err := randomAmounts(c, winners)
	if err != nil {
		return err
	}
	for i, p := range participants {
		prizeType := "guarantee"
		amount := c.GuaranteeAmount
		if i < winners {
			prizeType = "random"
			amount = amounts[i]
		}
		key := fmt.Sprintf("lottery:%d:%d", c.ID, p.ID)
		_, err := tx.ExecContext(ctx, `INSERT INTO portal_lottery_payouts(campaign_id,participant_id,core_user_id,email,prize_type,amount,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (idempotency_key) DO NOTHING`, c.ID, p.ID, p.CoreUserID, p.Email, prizeType, amount, key)
		if err != nil {
			return err
		}
	}
	now := time.Now()
	if _, err := tx.ExecContext(ctx, `UPDATE portal_lottery_campaigns SET status='completed',published=false,drawn_at=$2,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.processCampaignPayouts(ctx, id)
}

func (s *Service) RetryPayouts(ctx context.Context) error {
	if s.issuer == nil {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM portal_lottery_payouts WHERE status IN ('pending','failed') OR (status='processing' AND updated_at < CURRENT_TIMESTAMP - INTERVAL '5 minutes') ORDER BY updated_at,id LIMIT 200`)
	if err != nil {
		return err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := s.processPayout(ctx, id); err != nil {
			s.logger.Warn("lottery payout failed", "payout_id", id, "error", err)
		}
	}
	return nil
}

func (s *Service) DrawCampaign(ctx context.Context, id int64) error {
	return s.draw(ctx, id)
}

func (s *Service) RetryPayout(ctx context.Context, id int64) error {
	return s.processPayout(ctx, id)
}

func (s *Service) processCampaignPayouts(ctx context.Context, campaignID int64) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM portal_lottery_payouts WHERE campaign_id=$1 AND status IN ('pending','failed')`, campaignID)
	if err != nil {
		return err
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		_ = s.processPayout(ctx, id)
	}
	return nil
}

func (s *Service) processPayout(ctx context.Context, id int64) error {
	if s.issuer == nil {
		return errors.New("Core balance issuer is not configured")
	}
	var p Payout
	if err := s.db.QueryRowContext(ctx, `UPDATE portal_lottery_payouts SET status='processing',updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND (status IN ('pending','failed') OR (status='processing' AND updated_at < CURRENT_TIMESTAMP - INTERVAL '5 minutes')) RETURNING id,campaign_id,participant_id,core_user_id,email,prize_type,amount,status,error_message,created_at,credited_at`, id).Scan(&p.ID, &p.CampaignID, &p.ParticipantID, &p.CoreUserID, &p.Email, &p.PrizeType, &p.Amount, &p.Status, &p.ErrorMessage, &p.CreatedAt, &p.CreditedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	key := fmt.Sprintf("lottery:%d:%d", p.CampaignID, p.ParticipantID)
	err := s.issuer.AdminAddBalance(ctx, p.CoreUserID, p.Amount, fmt.Sprintf("Lottery campaign #%d %s", p.CampaignID, p.PrizeType), key)
	if err != nil {
		_, _ = s.db.ExecContext(ctx, `UPDATE portal_lottery_payouts SET status='failed',error_message=$2,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id, err.Error())
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE portal_lottery_payouts SET status='credited',error_message='',credited_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=$1`, id)
	return err
}

func (s *Service) RunWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if err := s.DrawDue(ctx); err != nil {
		s.logger.Error("lottery worker initial run failed", "error", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.DrawDue(ctx); err != nil {
				s.logger.Error("lottery worker failed", "error", err)
			}
		}
	}
}

type participant struct {
	ID, CoreUserID int64
	Email          string
}

func randomInt(max int) (int, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

func randomAmounts(c Campaign, winners int) ([]float64, error) {
	if winners == 0 {
		return []float64{}, nil
	}
	min, max := cents(c.RandomMin), cents(c.RandomMax)
	target := int64(math.Round(c.RandomBudget * 100 * float64(winners) / float64(c.RandomLimit)))
	if target < int64(winners)*min || target > int64(winners)*max {
		return nil, &Problem{http.StatusBadRequest, "INVALID_BUDGET", "Budget cannot satisfy the configured random amount range"}
	}
	amounts := make([]int64, winners)
	for i := range amounts {
		amounts[i] = min
	}
	remaining := target
	for i := 0; i < winners; i++ {
		remainingSlots := winners - i - 1
		low := maxInt64(min, remaining-int64(remainingSlots)*max)
		high := minInt64(max, remaining-int64(remainingSlots)*min)
		if low > high {
			return nil, &Problem{http.StatusBadRequest, "INVALID_BUDGET", "Budget cannot satisfy the configured random amount range"}
		}
		amount := low
		if high > low {
			n, err := randomInt(int(high-low) + 1)
			if err != nil {
				return nil, err
			}
			amount += int64(n)
		}
		amounts[i] = amount
		remaining -= amount
	}
	sort.Slice(amounts, func(i, j int) bool { return amounts[i] > amounts[j] })
	result := make([]float64, winners)
	for i, amount := range amounts {
		result[i] = float64(amount) / 100
	}
	return result, nil
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func cents(value float64) int64 { return int64(math.Round(value * 100)) }
func maskEmail(value string) string {
	parts := strings.SplitN(strings.ToLower(strings.TrimSpace(value)), "@", 2)
	if len(parts) != 2 {
		return value
	}
	prefix := parts[0]
	if len(prefix) > 2 {
		prefix = prefix[:2]
	}
	return prefix + "***@" + parts[1]
}

func (s *Service) queryCampaign(ctx context.Context, suffix string, args ...any) (Campaign, error) {
	query := `SELECT id,name,subtitle,registration_start,draw_at,participant_limit,random_limit,random_min,random_max,random_budget,guarantee_amount,status,published,drawn_at FROM portal_lottery_campaigns ` + suffix
	var item Campaign
	err := s.db.QueryRowContext(ctx, query, args...).Scan(campaignArgs(&item)...)
	return item, err
}
func campaignArgs(c *Campaign) []any {
	return []any{&c.ID, &c.Name, &c.Subtitle, &c.RegistrationStart, &c.DrawAt, &c.ParticipantLimit, &c.RandomLimit, &c.RandomMin, &c.RandomMax, &c.RandomBudget, &c.GuaranteeAmount, &c.Status, &c.Published, &c.DrawnAt}
}
func payoutArgs(p *Payout) []any {
	return []any{&p.ID, &p.CampaignID, &p.CampaignName, &p.ParticipantID, &p.CoreUserID, &p.Email, &p.PrizeType, &p.Amount, &p.Status, &p.ErrorMessage, &p.CreatedAt, &p.CreditedAt}
}

type scanner interface{ Scan(...any) error }

func scanCampaign(row scanner) (Campaign, error) {
	var c Campaign
	err := row.Scan(campaignArgs(&c)...)
	return c, err
}
func (s *Service) addCampaignCounts(ctx context.Context, c *Campaign) error {
	return s.db.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(*) FILTER (WHERE prize_type='random'),COUNT(*) FILTER (WHERE prize_type='guarantee'),COUNT(*) FILTER (WHERE status='credited'),COUNT(*) FILTER (WHERE status='failed'),COALESCE(SUM(amount) FILTER (WHERE prize_type='random'),0),COALESCE(SUM(amount) FILTER (WHERE prize_type='guarantee'),0) FROM portal_lottery_participants p LEFT JOIN portal_lottery_payouts o ON o.participant_id=p.id WHERE p.campaign_id=$1`, c.ID).Scan(&c.Participants, &c.RandomWinners, &c.GuaranteedWinners, &c.Credited, &c.Failed, &c.RandomPrize, &c.GuaranteePrize)
}
func mapDBError(err error) error {
	if isUniqueViolation(err) {
		return &Problem{http.StatusConflict, "ACTIVE_CAMPAIGN_EXISTS", "Only one published campaign window may be active"}
	}
	return err
}
func isUniqueViolation(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key") || strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
