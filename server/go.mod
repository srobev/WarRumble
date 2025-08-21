module rumble/server

go 1.22

require (
	github.com/gorilla/websocket v1.5.1
	rumble/shared v0.0.0
)

replace rumble/shared => ../shared
