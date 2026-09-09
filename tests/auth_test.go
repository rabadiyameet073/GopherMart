package tests

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/gophermart/ecommerce-order-inventory/internal/middleware"
	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/internal/service"
)

// In-memory mock user repository
type mockUserRepo struct {
	users map[string]*model.User
}

func (m *mockUserRepo) Create(ctx context.Context, u *model.User) error {
	m.users[u.Email] = u
	return nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	if u, ok := m.users[email]; ok {
		return u, nil
	}
	return nil, nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*model.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func TestAuthService_PasswordHashingAndJWT(t *testing.T) {
	repo := &mockUserRepo{users: make(map[string]*model.User)}
	secret := "test-secret-key-123"
	authSvc := service.NewAuthService(repo, secret, 1*time.Hour)

	// 1. Test Registration
	regReq := model.RegisterRequest{
		Name:     "Test User",
		Email:    "test@gophermart.com",
		Password: "password123",
	}

	regRes, err := authSvc.Register(context.Background(), regReq)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if regRes.Token == "" {
		t.Errorf("Expected valid JWT token, got empty")
	}

	// Verify password was hashed and NOT stored in plaintext
	savedUser := repo.users["test@gophermart.com"]
	if savedUser.PasswordHash == "password123" {
		t.Errorf("Password was stored in plaintext!")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(savedUser.PasswordHash), []byte("password123")); err != nil {
		t.Errorf("Password hash does not verify against original password: %v", err)
	}

	// 2. Test Duplicate Registration Conflict
	_, err = authSvc.Register(context.Background(), regReq)
	if err != service.ErrUserAlreadyExists {
		t.Errorf("Expected ErrUserAlreadyExists on duplicate registration, got: %v", err)
	}

	// 3. Test Successful Login
	loginReq := model.LoginRequest{
		Email:    "test@gophermart.com",
		Password: "password123",
	}
	loginRes, err := authSvc.Login(context.Background(), loginReq)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if loginRes.Token == "" {
		t.Errorf("Expected JWT token upon login, got empty string")
	}

	// 4. Test Invalid Password Login
	badLoginReq := model.LoginRequest{
		Email:    "test@gophermart.com",
		Password: "wrongPassword",
	}
	_, err = authSvc.Login(context.Background(), badLoginReq)
	if err != service.ErrInvalidCredentials {
		t.Errorf("Expected ErrInvalidCredentials on wrong password, got: %v", err)
	}

	// 5. Test JWT Claims Verification
	claims := &middleware.JWTClaims{}
	parsedToken, err := jwt.ParseWithClaims(loginRes.Token, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !parsedToken.Valid {
		t.Fatalf("Failed to parse and validate issued token: %v", err)
	}
	if claims.Email != "test@gophermart.com" {
		t.Errorf("Expected email claim test@gophermart.com, got %s", claims.Email)
	}
}
