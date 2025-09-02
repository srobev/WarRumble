package game

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
)

type RegisterReq struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"password_confirm"`
}
type LoginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type LoginResp struct {
    Token    string `json:"token"`
    Username string `json:"username"`
}

// In-memory session token for the current run (not persisted unless Remember is checked)
var sessionToken string

func apiBase() string {
	// read from config if you have one, fallback:
	return "http://localhost:8080"
}

func Register(username, password string) error {
	req := RegisterReq{Username: username, Password: password, PasswordConfirm: password}
	b, _ := json.Marshal(&req)
	resp, err := http.Post(apiBase()+"/api/register", "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return errors.New("register failed")
	}
	return nil
}

func Login(username, password string) (string, error) {
	req := LoginReq{Username: username, Password: password}
	b, _ := json.Marshal(&req)
	resp, err := http.Post(apiBase()+"/api/login", "application/json", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New("invalid credentials")
	}
	var out LoginResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	SaveToken(out.Token)
	return out.Token, nil
}

// func tokenPath() string {
// store alongside your existing config; adjust as needed
//
//		_ = os.MkdirAll("./config", 0o755)
//		return filepath.Join("./config", "token.json")
//	}
func tokenPath() string { return ConfigPath("token.json") } // keep your old filename if you want

// func userPath() string { return filepath.Join("./config", "username.txt") }
func userPath() string { return ConfigPath("username.txt") }

func SaveToken(tok string) error {
    return os.WriteFile(tokenPath(), []byte(strings.TrimSpace(tok)), 0o600)
}

//func SaveToken(tok string) error {
//	_ = os.MkdirAll("./config", 0o755)
//	return os.WriteFile(tokenPath(), []byte(strings.TrimSpace(tok)), 0o600)
//}

func LoadToken() string {
    if strings.TrimSpace(sessionToken) != "" {
        return strings.TrimSpace(sessionToken)
    }
    b, _ := os.ReadFile(ConfigPath("token.json"))
    return strings.TrimSpace(string(b))
}

func ClearToken() { _ = os.Remove(tokenPath()) }

func SetSessionToken(tok string) {
    sessionToken = strings.TrimSpace(tok)
}

//func SaveUsername(u string) error {
//	_ = os.MkdirAll("./config", 0o755)
//	return os.WriteFile(userPath(), []byte(strings.TrimSpace(u)), 0o600)
//}

func SaveUsername(u string) error {
	return os.WriteFile(ConfigPath("username.txt"), []byte(strings.TrimSpace(u)), 0o600)
}

func LoadUsername() string {
	b, _ := os.ReadFile(ConfigPath("username.txt"))
	return strings.TrimSpace(string(b))
}
func ClearUsername() { _ = os.Remove(userPath()) }
