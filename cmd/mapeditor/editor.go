package main

// This file previously contained utility functions and editor methods.
// All functionality has been moved to specialized files:
// - types.go: Data structures (wsMsg, editor struct)
// - config.go: Utility functions (getenv, sanitize, profileID, configDir, loadToken)
// - connection.go: WebSocket handling (dialWS, runReader)
// - save.go: Save functionality (save method)
// - auth.go: Authentication (attemptLogin method)
//
// Note: The main Update/Draw functionality remains in main.go for now
