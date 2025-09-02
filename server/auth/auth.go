// server/auth/auth.go
package auth

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"log"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

type userStore struct {
	mu    sync.RWMutex
	path  string
	users map[string]*User
}

func newUserStore(path string) (*userStore, error) {
	us := &userStore{path: path, users: map[string]*User{}}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &us.users)
	}
	return us, nil
}

func (s *userStore) save() error {
	// Read under RLock, then write file without holding the lock
	s.mu.RLock()
	b, _ := json.MarshalIndent(s.users, "", "  ")
	s.mu.RUnlock()
	return os.WriteFile(s.path, b, 0o600)
}

func (s *userStore) exists(username string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.users[strings.ToLower(username)]
	return ok
}

func (s *userStore) get(username string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[strings.ToLower(username)]
	return u, ok
}

func (s *userStore) put(u *User) error {
	s.mu.Lock()
	s.users[strings.ToLower(u.Username)] = u
	s.mu.Unlock()
	return s.save()
}

type Auth struct {
	users  *userStore
	jwtKey []byte
	issuer string
}

func NewAuth(dataDir string) (*Auth, error) {
	users, err := newUserStore(filepath.Join(dataDir, "users.json"))
	if err != nil {
		return nil, err
	}
	keyPath := filepath.Join(dataDir, "jwt.key")
	key, err := os.ReadFile(keyPath)
	if err != nil || len(key) < 32 {
		key = make([]byte, 32)
		_, _ = rand.Read(key)
		_ = os.WriteFile(keyPath, key, 0o600)
	}
	return &Auth{users: users, jwtKey: key, issuer: "WarRumble"}, nil
}

type RegisterReq struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"password_confirm"`
}
type RegisterResp struct{ OK bool `json:"ok"` }

func (a *Auth) HandleRegister(w http.ResponseWriter, r *http.Request) {
	log.Println("HandleRegister")
	var req RegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest); return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || len(req.Password) < 6 || req.Password != req.PasswordConfirm {
		http.Error(w, "invalid username or password mismatch / too short", http.StatusBadRequest); return
	}
	if a.users.exists(req.Username) {
		http.Error(w, "username already exists", http.StatusConflict); return
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	u := &User{Username: req.Username, PasswordHash: string(hash), CreatedAt: time.Now()}
	if err := a.users.put(u); err != nil { http.Error(w, "save failed", http.StatusInternalServerError); return }
	_ = json.NewEncoder(w).Encode(RegisterResp{OK: true})
}

type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type LoginResp struct {
	Token    string `json:"token"`
	Username string `json:"username"`
}

func (a *Auth) HandleLogin(w http.ResponseWriter, r *http.Request) {
	log.Println("HandleLogin")
	var req LoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest); return
	}
	u, ok := a.users.get(req.Username)
	if !ok || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized); return
	}
	claims := jwt.MapClaims{
		"sub": u.Username,
		"iss": a.issuer,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := t.SignedString(a.jwtKey)
	_ = json.NewEncoder(w).Encode(LoginResp{Token: signed, Username: u.Username})
}

func (a *Auth) ParseToken(tok string) (string, error) {
	if tok == "" { return "", errors.New("missing token") }
	t, err := jwt.Parse(tok, func(t *jwt.Token) (interface{}, error) {
		return a.jwtKey, nil
	})
	if err != nil || !t.Valid { return "", errors.New("invalid token") }
	if claims, ok := t.Claims.(jwt.MapClaims); ok {
		if sub, ok := claims["sub"].(string); ok { return sub, nil }
	}
	return "", errors.New("bad claims")
}

// Use this for protecting REST endpoints (already used in your main.go).
func (a *Auth) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tok string
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			tok = strings.TrimPrefix(h, "Bearer ")
		} else {
			tok = r.URL.Query().Get("token")
		}
		user, err := a.ParseToken(tok)
		if err != nil || user == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized); return
		}
		next.ServeHTTP(w, r)
	})
}

