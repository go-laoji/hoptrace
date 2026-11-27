package biz

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v1 "hoptrace/api/hoptrace/v1"
	"hoptrace/internal/conf"

	"github.com/go-kratos/kratos/v3/errors"
)

// 用户角色
const (
	UserRoleAdmin    = "admin"
	UserRoleOperator = "operator"
	UserRoleViewer   = "viewer"
)

var (
	ErrUnauthorized       = errors.Unauthorized(v1.ErrorReason_UNAUTHORIZED.String(), "authentication required")
	ErrForbidden          = errors.Forbidden(v1.ErrorReason_FORBIDDEN.String(), "permission denied")
	ErrInvalidCredentials = errors.Unauthorized(v1.ErrorReason_INVALID_CREDENTIALS.String(), "invalid username or password")
	ErrUserNotFound       = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
	ErrUserAlreadyExists  = errors.Conflict(v1.ErrorReason_USER_ALREADY_EXISTS.String(), "username already exists")
)

// User 平台用户 DO。
type User struct {
	ID           uint
	Username     string
	DisplayName  string
	PasswordHash string
	Role         string // admin / operator / viewer
	CreatedAt    time.Time
}

// UserRepo 用户仓储接口。
type UserRepo interface {
	Count(ctx context.Context) (int64, error)
	List(ctx context.Context) ([]*User, error)
	Get(ctx context.Context, id uint) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	Create(ctx context.Context, u *User) (*User, error)
	UpdatePassword(ctx context.Context, id uint, hash string) error
	Delete(ctx context.Context, id uint) error
}

// AuthClaims JWT 载荷。
type AuthClaims struct {
	UserID      uint   `json:"uid"`
	Username    string `json:"sub"`
	DisplayName string `json:"name"`
	Role        string `json:"role"`
	Exp         int64  `json:"exp"`
}

type claimsCtxKey struct{}

// WithClaims 将已认证的用户信息存入 Context。
func WithClaims(ctx context.Context, c *AuthClaims) context.Context {
	return context.WithValue(ctx, claimsCtxKey{}, c)
}

// ClaimsFromContext 从 Context 读取当前用户。
func ClaimsFromContext(ctx context.Context) (*AuthClaims, bool) {
	c, ok := ctx.Value(claimsCtxKey{}).(*AuthClaims)
	return c, ok && c != nil
}

// AuthUsecase 鉴权与用户管理。
type AuthUsecase struct {
	repo        UserRepo
	enabled     bool
	jwtSecret   []byte
	jwtTTL      time.Duration
	initUser    string
	initPass    string
}

func NewAuthUsecase(repo UserRepo, sec *conf.Security) (*AuthUsecase, error) {
	secret := sec.GetJwtSecret()
	if secret == "" {
		secret = sec.GetSecretKey()
	}
	if secret == "" {
		secret = "hoptrace-dev-secret-change-me"
	}
	ttl := sec.GetJwtTtl().AsDuration()
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	initUser := strings.TrimSpace(sec.GetInitAdminUser())
	if initUser == "" {
		initUser = "admin"
	}
	initPass := sec.GetInitAdminPassword()
	if initPass == "" {
		initPass = "admin123"
	}
	uc := &AuthUsecase{
		repo:      repo,
		enabled:   sec.GetAuthEnabled(),
		jwtSecret: []byte(secret),
		jwtTTL:    ttl,
		initUser:  initUser,
		initPass:  initPass,
	}
	if err := uc.ensureInitialAdmin(context.Background()); err != nil {
		return nil, err
	}
	return uc, nil
}

func (uc *AuthUsecase) Enabled() bool { return uc.enabled }

func (uc *AuthUsecase) ensureInitialAdmin(ctx context.Context) error {
	n, err := uc.repo.Count(ctx)
	if err != nil || n > 0 {
		return err
	}
	hash, err := hashPassword(uc.initPass)
	if err != nil {
		return err
	}
	_, err = uc.repo.Create(ctx, &User{
		Username:     uc.initUser,
		DisplayName:  "系统管理员",
		PasswordHash: hash,
		Role:         UserRoleAdmin,
	})
	return err
}

func (uc *AuthUsecase) Login(ctx context.Context, username, password string) (string, int64, *User, error) {
	u, err := uc.repo.FindByUsername(ctx, strings.TrimSpace(username))
	if err != nil {
		return "", 0, nil, ErrInvalidCredentials
	}
	if !verifyPassword(password, u.PasswordHash) {
		return "", 0, nil, ErrInvalidCredentials
	}
	exp := time.Now().Add(uc.jwtTTL).Unix()
	tok, err := uc.SignToken(&AuthClaims{
		UserID:      u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Role:        u.Role,
		Exp:         exp,
	})
	if err != nil {
		return "", 0, nil, err
	}
	return tok, exp, u, nil
}

func (uc *AuthUsecase) CurrentUser(ctx context.Context) (*User, bool, error) {
	if !uc.enabled {
		return &User{ID: 0, Username: "local-admin", DisplayName: "网络运维", Role: UserRoleAdmin}, false, nil
	}
	claims, ok := ClaimsFromContext(ctx)
	if !ok {
		return nil, true, nil
	}
	u, err := uc.repo.Get(ctx, claims.UserID)
	if err != nil {
		return nil, true, nil
	}
	return u, true, nil
}

func (uc *AuthUsecase) ChangePassword(ctx context.Context, oldPwd, newPwd string) error {
	if len(newPwd) < 6 {
		return errors.BadRequest(v1.ErrorReason_INVALID_CREDENTIALS.String(), "password must be at least 6 characters")
	}
	claims, ok := ClaimsFromContext(ctx)
	if !ok || claims.UserID == 0 {
		return ErrUnauthorized
	}
	u, err := uc.repo.Get(ctx, claims.UserID)
	if err != nil {
		return err
	}
	if !verifyPassword(oldPwd, u.PasswordHash) {
		return ErrInvalidCredentials
	}
	hash, err := hashPassword(newPwd)
	if err != nil {
		return err
	}
	return uc.repo.UpdatePassword(ctx, u.ID, hash)
}

func (uc *AuthUsecase) ListUsers(ctx context.Context) ([]*User, error) {
	return uc.repo.List(ctx)
}

func (uc *AuthUsecase) CreateUser(ctx context.Context, username, password, displayName, role string) (*User, error) {
	username = strings.TrimSpace(username)
	if username == "" || len(password) < 6 {
		return nil, errors.BadRequest(v1.ErrorReason_INVALID_CREDENTIALS.String(), "invalid username or password too short")
	}
	switch role {
	case UserRoleAdmin, UserRoleOperator, UserRoleViewer:
	case "":
		role = UserRoleViewer
	default:
		return nil, errors.BadRequest(v1.ErrorReason_INVALID_CREDENTIALS.String(), "invalid role")
	}
	if _, err := uc.repo.FindByUsername(ctx, username); err == nil {
		return nil, ErrUserAlreadyExists
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	if displayName == "" {
		displayName = username
	}
	return uc.repo.Create(ctx, &User{
		Username:     username,
		DisplayName:  displayName,
		PasswordHash: hash,
		Role:         role,
	})
}

func (uc *AuthUsecase) DeleteUser(ctx context.Context, id uint) error {
	if claims, ok := ClaimsFromContext(ctx); ok && claims.UserID == id {
		return ErrForbidden
	}
	return uc.repo.Delete(ctx, id)
}

// ---------- HS256 JWT ----------

func (uc *AuthUsecase) SignToken(c *AuthClaims) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, uc.jwtSecret)
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig, nil
}

func (uc *AuthUsecase) ParseToken(tok string) (*AuthClaims, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return nil, ErrUnauthorized
	}
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, uc.jwtSecret)
	mac.Write([]byte(signingInput))
	expected := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || subtle.ConstantTimeCompare(expected, got) != 1 {
		return nil, ErrUnauthorized
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrUnauthorized
	}
	var c AuthClaims
	if err := json.Unmarshal(payloadBytes, &c); err != nil {
		return nil, ErrUnauthorized
	}
	if c.Exp > 0 && time.Now().Unix() > c.Exp {
		return nil, ErrUnauthorized
	}
	return &c, nil
}

// ---------- 加盐迭代 SHA-256 密码哈希 ----------

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	dk := deriveKey([]byte(password), salt)
	return fmt.Sprintf("s256$%s$%s", hex.EncodeToString(salt), hex.EncodeToString(dk)), nil
}

func verifyPassword(password, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 3 || parts[0] != "s256" {
		return false
	}
	salt, err1 := hex.DecodeString(parts[1])
	want, err2 := hex.DecodeString(parts[2])
	if err1 != nil || err2 != nil {
		return false
	}
	got := deriveKey([]byte(password), salt)
	return subtle.ConstantTimeCompare(want, got) == 1
}

func deriveKey(password, salt []byte) []byte {
	cur := make([]byte, 0, len(salt)+len(password))
	cur = append(cur, salt...)
	cur = append(cur, password...)
	sum := sha256.Sum256(cur)
	for i := 0; i < 10000; i++ {
		h := sha256.New()
		h.Write(sum[:])
		h.Write(salt)
		h.Sum(sum[:0])
	}
	out := make([]byte, len(sum))
	copy(out, sum[:])
	return out
}
