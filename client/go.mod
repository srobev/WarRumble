module rumble/client

go 1.23

require (
	github.com/atotto/clipboard v0.1.4
	github.com/gorilla/websocket v1.5.1
	github.com/hajimehoshi/ebiten/v2 v2.8.8
	golang.org/x/image v0.30.0
	rumble/shared v0.0.0
)

require (
	github.com/ebitengine/gomobile v0.0.0-20250329061421-6d0a8e981e4c // indirect
	github.com/ebitengine/hideconsole v1.0.0 // indirect
	github.com/ebitengine/purego v0.8.0 // indirect
	github.com/jezek/xgb v1.1.1 // indirect
	golang.org/x/exp/shiny v0.0.0-20250718183923-645b1fa84792 // indirect
	golang.org/x/mobile v0.0.0-20250813145510-f12310a0cfd9 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
)

replace rumble/shared => ../shared
