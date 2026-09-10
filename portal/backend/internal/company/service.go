package company

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

type Company struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Scope struct {
	UserID     int64
	IsAdmin    bool
	CompanyIDs []int64
}

type MemberRef struct {
	CompanyID   int64
	CompanyName string
	CoreUserID  int64
}

type ScopedMemberRef struct {
	CompanyID   int64
	CompanyName string
	CoreUserID  int64
}

type ManagerRef struct {
	CompanyID  int64
	CoreUserID int64
	CreatedAt  time.Time
}

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) Scope(ctx context.Context, coreUserID int64, role string) (Scope, error) {
	scope := Scope{UserID: coreUserID, IsAdmin: role == "admin", CompanyIDs: []int64{}}
	if scope.IsAdmin {
		return scope, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT m.company_id FROM portal_company_managers m JOIN portal_companies c ON c.id=m.company_id WHERE m.core_user_id=$1 AND c.status='active' AND c.deleted_at IS NULL ORDER BY m.company_id`, coreUserID)
	if err != nil {
		return scope, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return scope, err
		}
		scope.CompanyIDs = append(scope.CompanyIDs, id)
	}
	return scope, rows.Err()
}

func (s *Service) List(ctx context.Context, scope Scope) ([]Company, error) {
	query := `SELECT id,name,status,created_at,updated_at FROM portal_companies WHERE deleted_at IS NULL`
	args := []any{}
	if !scope.IsAdmin {
		query += ` AND id = ANY($1)`
		args = append(args, scope.CompanyIDs)
	}
	query += ` ORDER BY name,id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Company{}
	for rows.Next() {
		var item Company
		if err := rows.Scan(&item.ID, &item.Name, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Service) Create(ctx context.Context, scope Scope, name, status string) (Company, error) {
	if !scope.IsAdmin {
		return Company{}, problem(403, "FULL_ADMIN_REQUIRED", "Full administrator access required")
	}
	name, status, err := normalize(name, status)
	if err != nil {
		return Company{}, err
	}
	var item Company
	err = s.db.QueryRowContext(ctx, `INSERT INTO portal_companies(name,status) VALUES($1,$2) RETURNING id,name,status,created_at,updated_at`, name, status).Scan(&item.ID, &item.Name, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	return item, mapWriteError(err)
}

func (s *Service) Update(ctx context.Context, scope Scope, id int64, name, status string) (Company, error) {
	if !scope.IsAdmin {
		return Company{}, problem(403, "FULL_ADMIN_REQUIRED", "Full administrator access required")
	}
	name, status, err := normalize(name, status)
	if err != nil {
		return Company{}, err
	}
	var item Company
	err = s.db.QueryRowContext(ctx, `UPDATE portal_companies SET name=$2,status=$3,updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND deleted_at IS NULL RETURNING id,name,status,created_at,updated_at`, id, name, status).Scan(&item.ID, &item.Name, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Company{}, problem(404, "COMPANY_NOT_FOUND", "Company not found")
	}
	return item, mapWriteError(err)
}

func (s *Service) Deactivate(ctx context.Context, scope Scope, id int64) (Company, error) {
	if !scope.IsAdmin {
		return Company{}, problem(403, "FULL_ADMIN_REQUIRED", "Full administrator access required")
	}
	var item Company
	err := s.db.QueryRowContext(ctx, `UPDATE portal_companies SET status='disabled',deleted_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND deleted_at IS NULL RETURNING id,name,status,created_at,updated_at`, id).Scan(&item.ID, &item.Name, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Company{}, problem(404, "COMPANY_NOT_FOUND", "Company not found")
	}
	return item, err
}

func (s *Service) AssignMember(ctx context.Context, scope Scope, companyID, coreUserID int64) error {
	if !allows(scope, companyID) {
		return problem(403, "COMPANY_SCOPE_REQUIRED", "Company access required")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO portal_company_members(company_id,core_user_id) SELECT id,$2 FROM portal_companies WHERE id=$1 AND status='active' AND deleted_at IS NULL`, companyID, coreUserID)
	if err != nil {
		return mapWriteError(err)
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return problem(404, "COMPANY_NOT_FOUND", "Company not found")
	}
	return nil
}

func (s *Service) RequireActiveAccess(ctx context.Context, scope Scope, companyID int64) error {
	if !allows(scope, companyID) {
		return problem(403, "COMPANY_SCOPE_REQUIRED", "Company access required")
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM portal_companies WHERE id=$1 AND status='active' AND deleted_at IS NULL)`, companyID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return problem(404, "COMPANY_NOT_FOUND", "Company not found")
	}
	return nil
}

func (s *Service) RequireMember(ctx context.Context, scope Scope, companyID, coreUserID int64) error {
	if coreUserID <= 0 {
		return problem(400, "INVALID_USER", "Invalid user")
	}
	if err := s.RequireActiveAccess(ctx, scope, companyID); err != nil {
		return err
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM portal_company_members WHERE company_id=$1 AND core_user_id=$2)`, companyID, coreUserID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return problem(404, "COMPANY_MEMBER_NOT_FOUND", "Company member not found")
	}
	return nil
}

func (s *Service) RemoveMember(ctx context.Context, scope Scope, companyID, coreUserID int64) error {
	if !allows(scope, companyID) {
		return problem(403, "COMPANY_SCOPE_REQUIRED", "Company access required")
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM portal_company_members WHERE company_id=$1 AND core_user_id=$2`, companyID, coreUserID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return problem(404, "COMPANY_MEMBER_NOT_FOUND", "Company member not found")
	}
	return nil
}

func (s *Service) AddManager(ctx context.Context, scope Scope, companyID, coreUserID int64) error {
	if !scope.IsAdmin {
		return problem(403, "FULL_ADMIN_REQUIRED", "Full administrator access required")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO portal_company_managers(company_id,core_user_id) SELECT company_id,core_user_id FROM portal_company_members WHERE company_id=$1 AND core_user_id=$2 ON CONFLICT DO NOTHING`, companyID, coreUserID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n != 1 {
		return problem(400, "MANAGER_MUST_BE_MEMBER", "Company manager must be a member")
	}
	return nil
}

func (s *Service) RemoveManager(ctx context.Context, scope Scope, companyID, coreUserID int64) error {
	if !scope.IsAdmin {
		return problem(403, "FULL_ADMIN_REQUIRED", "Full administrator access required")
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM portal_company_managers WHERE company_id=$1 AND core_user_id=$2`, companyID, coreUserID)
	return err
}

func (s *Service) RemoveUserRelations(ctx context.Context, coreUserID int64) error {
	if coreUserID <= 0 {
		return problem(400, "INVALID_USER", "Invalid user")
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM portal_company_managers WHERE core_user_id=$1`, coreUserID); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM portal_company_members WHERE core_user_id=$1`, coreUserID); err != nil {
		return err
	}
	return nil
}

func (s *Service) Members(ctx context.Context, scope Scope, companyID int64) ([]MemberRef, error) {
	if !allows(scope, companyID) {
		return nil, problem(403, "COMPANY_SCOPE_REQUIRED", "Company access required")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT m.company_id,c.name,m.core_user_id FROM portal_company_members m JOIN portal_companies c ON c.id=m.company_id AND c.deleted_at IS NULL WHERE m.company_id=$1 ORDER BY m.core_user_id`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []MemberRef{}
	for rows.Next() {
		var item MemberRef
		if err := rows.Scan(&item.CompanyID, &item.CompanyName, &item.CoreUserID); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Service) ScopedMembers(ctx context.Context, scope Scope, companyIDs []int64) ([]ScopedMemberRef, error) {
	ids := normalizeCompanyIDs(companyIDs)
	if !scope.IsAdmin {
		allowed := map[int64]bool{}
		for _, id := range scope.CompanyIDs {
			allowed[id] = true
		}
		filtered := ids[:0]
		for _, id := range ids {
			if allowed[id] {
				filtered = append(filtered, id)
			}
		}
		if len(ids) > 0 && len(filtered) != len(ids) {
			return nil, problem(403, "COMPANY_SCOPE_REQUIRED", "Company access required")
		}
		ids = filtered
		if len(ids) == 0 {
			ids = normalizeCompanyIDs(scope.CompanyIDs)
		}
	}
	query := `SELECT m.company_id,c.name,m.core_user_id FROM portal_company_members m JOIN portal_companies c ON c.id=m.company_id WHERE c.status='active' AND c.deleted_at IS NULL`
	args := []any{}
	if len(ids) > 0 {
		query += ` AND m.company_id = ANY($1)`
		args = append(args, ids)
	}
	query += ` ORDER BY c.name,m.core_user_id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ScopedMemberRef{}
	for rows.Next() {
		var item ScopedMemberRef
		if err := rows.Scan(&item.CompanyID, &item.CompanyName, &item.CoreUserID); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Service) Managers(ctx context.Context, scope Scope, companyID int64) ([]ManagerRef, error) {
	if !allows(scope, companyID) {
		return nil, problem(403, "COMPANY_SCOPE_REQUIRED", "Company access required")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT company_id,core_user_id,created_at FROM portal_company_managers WHERE company_id=$1 ORDER BY core_user_id`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []ManagerRef{}
	for rows.Next() {
		var item ManagerRef
		if err := rows.Scan(&item.CompanyID, &item.CoreUserID, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func normalize(name, status string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 100 {
		return "", "", problem(400, "COMPANY_NAME_INVALID", "Invalid company name")
	}
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "disabled" {
		return "", "", problem(400, "COMPANY_STATUS_INVALID", "Invalid company status")
	}
	return name, status, nil
}

func allows(scope Scope, companyID int64) bool {
	if scope.IsAdmin {
		return true
	}
	for _, id := range scope.CompanyIDs {
		if id == companyID {
			return true
		}
	}
	return false
}

func normalizeCompanyIDs(ids []int64) []int64 {
	seen := map[int64]bool{}
	result := []int64{}
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

func problem(status int, code, message string) error {
	return &Error{Status: status, Code: code, Message: message}
}

func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "duplicate key") {
		return problem(409, "COMPANY_CONFLICT", "Company or membership already exists")
	}
	return fmt.Errorf("company storage: %w", err)
}
