package auth

type Claims struct {
	UserID     string   `json:"user_id"`
	TenantID   string   `json:"tenant_id"`
	RoleID     string   `json:"role_id"`
	RoleCode   string   `json:"role_code"`
	Roles      []string `json:"roles"`
	IssuedAt   int64    `json:"iat"`
	IssuedAtNS int64    `json:"iat_ns"`
	ExpiresAt  int64    `json:"exp"`
}
