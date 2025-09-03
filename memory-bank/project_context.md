# WarRumble Project Context

This project is WarRumble, a multiplayer game built in Go.

## Overview
- **Language**: Go 1.24.6
- **Modules**: client, server, shared, cmd/mapeditor, cmd/splitgame
- **Platform**: Cross-platform (desktop via Ebiten, Android support)
- **Genre**: Multiplayer strategy game with battles, guilds, and miniatures

## Structure
- `client/`: Contains the game client code, including UI, game logic, assets.
  - `internal/game/`: Core game logic (app.go, battle.go, world.go, etc.)
  - `assets/`: Maps, minis (units), portraits, UI elements
  - `mobile/`: Android-specific code
- `server/`: Server-side code for handling game state, authentication, social features.
  - `auth/`: Authentication logic
  - `data/`: JSON data files for friends, guilds, messages, minis, arenas, maps, profiles
  - `srv/`: Server implementation
- `shared/`: Shared protocol and types between client and server
- `cmd/`: Command-line tools
  - `mapeditor/`: Tool for editing maps
  - `splitgame/`: Tool for splitting game data
- `memory-bank/`: Project documentation and context

## Architecture
- **Client-Server Model**: Client handles UI and local game state, server manages global state and multiplayer interactions.
- **Networking**: Uses WebSockets or similar for real-time communication (see `client/internal/game/net.go`, `net_handlers.go`).
- **Game Loop**: Ebiten-based game loop in `client/internal/game/app.go`.
- **Data Storage**: Server uses JSON files for persistence (e.g., profiles, guilds).
- **Assets**: Organized in `client/internal/game/assets/` with subdirs for maps, minis, portraits, UI.

## Key Components
- **Battle System**: Handles PVP battles (`battle.go`)
- **World/Map System**: Manages game world and navigation (`world.go`, map data in `server/data/maps/`)
- **Social Features**: Guilds, friends, chat (`social.go`, `guild_chat_persist.go`)
- **Authentication**: Login and user management (`auth.go`, `auth_ui.go`)
- **UI System**: Various UI components (`topbar.go`, `bottombar.go`, `ui_common.go`)
- **Miniatures/Units**: Game units with stats and abilities (data in `server/data/minis.json`, assets in `assets/minis/`)
- **Profiles and Progression**: XP, levels, ratings (`profile.go`, `mini_xp.go`)

## Development Setup
- **Go Version**: 1.24.6
- **Dependencies**: Ebiten for client graphics, standard library for networking
- **Build**: Use `go build` in respective modules
- **Run Client**: `go run client/main_desktop.go` or build APK for Android
- **Run Server**: Implement server startup (likely in `server/srv/`)
- **Tools**: Map editor in `cmd/mapeditor/`, splitgame in `cmd/splitgame/`

## TODOs
- [ ] Implement server startup and connection handling
- [ ] Complete battle mechanics and unit interactions
- [ ] Add more maps and arenas
- [ ] Enhance UI for mobile responsiveness
- [ ] Implement matchmaking for PVP
- [ ] Add sound effects and music
- [ ] Optimize performance for large battles
- [ ] Add tutorial system

## Decisions Log
- **2023-XX-XX**: Chose Go for cross-platform compatibility and performance.
- **Initial Setup**: Used Ebiten for game framework due to simplicity and Go integration.
- **Asset Organization**: Structured assets by type (maps, minis, portraits) for easy management.
- **Data Storage**: JSON files for simplicity; consider database for scaling.
- **Networking**: Custom protocol in `shared/protocol/` for game-specific messages.

## Notes
- Project uses Go workspaces (`go.work`) for multi-module setup.
- Assets include PNG images for maps, units, and UI elements.
- Open tabs indicate active development areas: battle, world, networking, types.
- Consider adding unit tests for core logic.
- Mobile version uses `main_android.go` for Android builds.

This memory bank serves as a central repository for project information. Update as needed during development.
