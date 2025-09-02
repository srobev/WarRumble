package netcfg

import "os"

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

var APIBase = getenv("WAR_API_BASE", "http://127.0.0.1:8080")  // REST
var ServerURL = getenv("WAR_WS_URL", "ws://127.0.0.1:8080/ws") // WebSocket

//var APIBase   = getenv("WAR_API_BASE", "http://34.173.240.153:8080") // REST
//var ServerURL = getenv("WAR_WS_URL", "ws://34.173.240.153:8080/ws") // WebSocket
