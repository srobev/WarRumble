module rumble/server

go 1.23.0

require (
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/gorilla/websocket v1.5.1
	golang.org/x/crypto v0.41.0
	rumble/shared v0.0.0
)

require golang.org/x/net v0.42.0 // indirect

replace rumble/shared => ../shared
