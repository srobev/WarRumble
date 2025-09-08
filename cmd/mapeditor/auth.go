package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"rumble/shared/protocol"
)

// attemptLogin handles user authentication and WebSocket connection
func (e *editor) attemptLogin() {
	if strings.TrimSpace(e.loginUser) == "" || strings.TrimSpace(e.loginPass) == "" {
		e.loginStatus = "Please enter username and password"
		return
	}

	e.loginStatus = "Logging in..."

	// Run login in a goroutine to avoid blocking the UI
	go func() {
		req := map[string]interface{}{
			"username": e.loginUser,
			"password": e.loginPass,
			"version":  protocol.GameVersion,
		}
		b, _ := json.Marshal(req)

		resp, err := http.Post("http://localhost:8080/api/login", "application/json", strings.NewReader(string(b)))
		if err != nil {
			e.loginStatus = "Connection failed: " + err.Error()
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			if resp.StatusCode == 426 {
				e.loginStatus = "Version mismatch: please update your game"
			} else {
				e.loginStatus = "Invalid credentials"
			}
			return
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			e.loginStatus = "Failed to parse response"
			return
		}

		token, ok := result["token"].(string)
		if !ok {
			e.loginStatus = "Invalid response format"
			return
		}

		// Save token
		if err := os.WriteFile(filepath.Join(configDir(), "token.json"), []byte(strings.TrimSpace(token)), 0o600); err != nil {
			e.loginStatus = "Failed to save token"
			return
		}

		// Save username
		if err := os.WriteFile(filepath.Join(configDir(), "username.txt"), []byte(strings.TrimSpace(e.loginUser)), 0o600); err != nil {
			e.loginStatus = "Failed to save username"
			return
		}

		e.loginStatus = "Login successful! Connecting..."

		// Connect to WebSocket
		ws, err := dialWS(getenv("WAR_WS_URL", "ws://127.0.0.1:8080/ws"), token)
		if err != nil {
			e.loginStatus = "WebSocket connection failed: " + err.Error()
			return
		}

		e.ws = ws
		e.inCh = make(chan wsMsg, 128)
		go e.runReader()
		e.showLogin = false
		e.loginStatus = ""
	}()
}
