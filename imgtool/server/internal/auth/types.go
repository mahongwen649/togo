package auth

const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

type User struct {
	ID               string `json:"id"`
	Username         string `json:"username"`
	PasswordHash     string `json:"-"`
	Role             string `json:"role"`
	DisabledAt       int64  `json:"-"`
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
	ExternalProvider string `json:"-"`
	ExternalSubject  string `json:"-"`
}

type Session struct {
	ID         string
	UserID     string
	ExpiresAt  int64
	CreatedAt  int64
	LastSeenAt int64
}
