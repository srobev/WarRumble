# WarRumble Technical Context

## Technology Stack

### Core Technologies
- **Go 1.24.6**: Primary programming language for all components
- **Ebiten v2**: 2D game library for client-side rendering and game loop
- **WebSocket**: Real-time communication protocol between client and server
- **JSON**: Data serialization format for configuration and persistence

### Supporting Technologies
- **Go Modules**: Dependency management and workspace organization
- **Go Workspaces**: Multi-module project structure management
- **Standard Library**: Extensive use of net/http, encoding/json, sync, etc.

## Development Environment

### Project Structure
```
WarRumble/
├── client/           # Game client module
├── server/           # Game server module
├── shared/           # Shared types and protocols
├── cmd/              # Command-line tools
│   ├── mapeditor/    # Map editing tool
│   └── splitgame/    # Game data splitting tool
├── memory-bank/      # Project documentation
└── go.work          # Workspace configuration
```

### Module Organization
- **client**: Contains game client with platform-specific entry points
- **server**: Backend services for game state, authentication, social features
- **shared**: Common types, protocols, and utilities
- **cmd**: Development and utility tools

## Development Setup

### Prerequisites
- **Go 1.24.6+**: Download from golang.org
- **Git**: Version control
- **Android SDK**: For mobile builds (optional)

### Environment Setup
```bash
# Clone repository
git clone https://github.com/srobev/WarRumble.git
cd WarRumble

# Initialize workspace
go work init
go work use ./client
go work use ./server
go work use ./shared
go work use ./cmd/mapeditor
go work use ./cmd/splitgame
```

### Build Commands
```bash
# Build desktop client
cd client
go build -o ../bin/warrumble-desktop ./main_desktop.go

# Build Android APK
go build -buildmode=c-shared -o ../bin/android-arm64.so ./main_android.go
# Use Android Studio or gradle to create APK

# Build server
cd ../server
go build -o ../bin/warrumble-server ./main.go

# Build tools
cd ../cmd/mapeditor
go build -o ../../bin/mapeditor ./main.go
```

### Run Commands
```bash
# Run desktop client
go run client/main_desktop.go

# Run server
go run server/main.go

# Run map editor
go run cmd/mapeditor/main.go
```

## Technical Constraints

### Performance Constraints
- **Mobile Target**: 60 FPS on Android devices (entry-level to mid-range)
- **Memory Limits**: < 500MB RAM usage on mobile devices
- **Battery Life**: Minimize CPU/GPU usage for extended play sessions
- **Network Latency**: Support for 100-500ms network conditions

### Platform Constraints
- **Cross-Platform Compatibility**: Consistent behavior across Windows, macOS, Linux, Android
- **Input Methods**: Support touch, mouse, keyboard inputs
- **Screen Sizes**: Responsive UI for various resolutions (480p to 4K)
- **File System**: Handle different file system limitations and permissions

### Development Constraints
- **Single Developer**: Code must be maintainable and well-documented
- **No External Dependencies**: Minimize third-party libraries to reduce complexity
- **Go Idioms**: Follow Go best practices and conventions
- **Version Compatibility**: Support recent Go versions without breaking changes

## Dependencies

### Core Dependencies
- **github.com/hajimehoshi/ebiten/v2**: 2D game library
  - Version: v2.x (latest stable)
  - Purpose: Graphics rendering, input handling, game loop
  - License: Apache 2.0

### Indirect Dependencies
- **golang.org/x packages**: Official Go extensions
  - x/image: Image processing utilities
  - x/mobile: Mobile platform support
  - x/net/websocket: WebSocket implementation

### Development Dependencies
- **Go Tools**: gofmt, go vet, golint for code quality
- **Testing**: Go's built-in testing framework
- **Profiling**: pprof for performance analysis

## Tool Usage Patterns

### Development Workflow
1. **Code Changes**: Edit in VSCode with Go extension
2. **Testing**: Run `go test ./...` for unit tests
3. **Building**: Use `go build` for compilation
4. **Running**: Execute with `go run` for quick testing
5. **Debugging**: Use Delve debugger or print statements

### Asset Management
- **Image Assets**: PNG format in `client/internal/game/assets/`
- **Data Files**: JSON format in `server/data/`
- **Version Control**: All assets committed to Git
- **Organization**: Grouped by type (maps, minis, portraits, UI)

### Map Editor Tool
```bash
# Run map editor
go run cmd/mapeditor/main.go

# Edit specific map
# Tool provides GUI for:
# - Placing terrain tiles
# - Setting spawn points
# - Configuring obstacles
# - Previewing layouts
```

### Split Game Tool
```bash
# Run split game tool
go run cmd/splitgame/main.go

# Purpose: Split large game data files
# Useful for optimizing load times
# Creates smaller chunks for streaming
```

## Build Configuration

### Desktop Build
```go
// main_desktop.go
package main

import (
    "log"
    "github.com/hajimehoshi/ebiten/v2"
    "client/internal/game"
)

func main() {
    game := game.NewGame()
    ebiten.SetWindowSize(800, 600)
    ebiten.SetWindowTitle("WarRumble")

    if err := ebiten.RunGame(game); err != nil {
        log.Fatal(err)
    }
}
```

### Android Build
```go
// main_android.go
package main

import (
    "client/internal/game"
    "github.com/hajimehoshi/ebiten/v2/mobile"
)

func init() {
    mobile.SetGame(game.NewGame())
}

// Dummy main for build
func main() {}
```

### Server Build
```go
// server/main.go
package main

import (
    "log"
    "net/http"
    "server/srv"
)

func main() {
    hub := srv.NewHub()
    go hub.Run()

    http.HandleFunc("/ws", hub.HandleWebSocket)

    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## Deployment Considerations

### Desktop Deployment
- **Single Binary**: No installation required
- **Cross-Compilation**: Build for multiple platforms
- **Asset Bundling**: Embed assets in binary or distribute separately

### Mobile Deployment
- **APK Generation**: Use Android Studio or command-line tools
- **App Store Requirements**: Follow platform guidelines
- **Update Mechanism**: Implement in-app update system

### Server Deployment
- **Containerization**: Docker for consistent deployment
- **Process Management**: Systemd or similar for production
- **Load Balancing**: Nginx or similar for scaling
- **Backup Strategy**: Regular data snapshots and offsite storage

## Monitoring and Debugging

### Logging
- **Structured Logging**: Use Go's log package with consistent format
- **Log Levels**: Debug, Info, Warn, Error
- **Log Rotation**: Implement log file rotation for long-running server

### Performance Monitoring
- **Frame Rate**: Monitor FPS in client
- **Memory Usage**: Track allocations and garbage collection
- **Network Stats**: Monitor connection quality and message rates
- **Profiling**: Use pprof for performance analysis

### Error Handling
- **Graceful Shutdown**: Handle interrupts and cleanup resources
- **Panic Recovery**: Use recover() in critical goroutines
- **Error Reporting**: Log errors with context for debugging
