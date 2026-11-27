package biz

import (
	"context"
	"testing"
	"time"

	"hoptrace/internal/conf"

	"google.golang.org/protobuf/types/known/durationpb"
)

type memUserRepo struct {
	next  uint
	users map[uint]*User
}

func newMemUserRepo() *memUserRepo { return &memUserRepo{next: 1, users: map[uint]*User{}} }
func (r *memUserRepo) Count(_ context.Context) (int64, error) {
	return int64(len(r.users)), nil
}
func (r *memUserRepo) List(_ context.Context) ([]*User, error) {
	var out []*User
	for _, u := range r.users {
		out = append(out, u)
	}
	return out, nil
}
func (r *memUserRepo) Get(_ context.Context, id uint) (*User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u, nil
}
func (r *memUserRepo) FindByUsername(_ context.Context, username string) (*User, error) {
	for _, u := range r.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, ErrUserNotFound
}
func (r *memUserRepo) Create(_ context.Context, u *User) (*User, error) {
	cp := *u
	cp.ID = r.next
	r.next++
	r.users[cp.ID] = &cp
	return &cp, nil
}
func (r *memUserRepo) UpdatePassword(_ context.Context, id uint, hash string) error {
	r.users[id].PasswordHash = hash
	return nil
}
func (r *memUserRepo) Delete(_ context.Context, id uint) error {
	delete(r.users, id)
	return nil
}

func TestAuthLoginJWTAndPasswordChange(t *testing.T) {
	ctx := context.Background()
	repo := newMemUserRepo()
	uc, err := NewAuthUsecase(repo, &conf.Security{
		AuthEnabled:       true,
		JwtSecret:         "unit-test-secret",
		JwtTtl:            durationpb.New(time.Hour),
		InitAdminUser:     "admin",
		InitAdminPassword: "secret123",
	})
	if err != nil {
		t.Fatalf("NewAuthUsecase: %v", err)
	}

	// 1. 初始管理员登录成功
	tok, _, u, err := uc.Login(ctx, "admin", "secret123")
	if err != nil || tok == "" || u.Role != UserRoleAdmin {
		t.Fatalf("Login failed: err=%v tok=%q user=%+v", err, tok, u)
	}

	// 2. 校验合法与被篡改的 JWT
	claims, err := uc.ParseToken(tok)
	if err != nil || claims.Username != "admin" || claims.Role != UserRoleAdmin {
		t.Fatalf("ParseToken: err=%v claims=%+v", err, claims)
	}
	if _, err := uc.ParseToken(tok + "x"); err == nil {
		t.Fatal("expected tampered token to fail")
	}

	// 3. 修改密码后旧密码失效、新密码可用
	authCtx := WithClaims(ctx, claims)
	if err := uc.ChangePassword(authCtx, "secret123", "newpass456"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if _, _, _, err := uc.Login(ctx, "admin", "secret123"); err == nil {
		t.Fatal("expected old password to fail")
	}
	if _, _, _, err := uc.Login(ctx, "admin", "newpass456"); err != nil {
		t.Fatalf("expected new password to succeed: %v", err)
	}
}
