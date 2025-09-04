# WarRumble System Patterns

## System Architecture

### Client-Server Model
```
┌─────────────────┐    WebSocket    ┌─────────────────┐
│     Client      │◄──────────────►│     Server      │
│                 │                │                 │
│ • Game Logic    │                │ • Game State    │
│ • UI Rendering  │                │ • Persistence   │
│ • Local State   │                │ • Social Data   │
│ • Asset Mgmt    │                │ • Matchmaking   │
└─────────────────┘                └─────────────────┘
```

### Client Architecture
```
┌─────────────────────────────────────────────────┐
│                Game Loop (Ebiten)               │
├─────────────────────────────────────────────────┤
│ • Update() - Game logic, networking, UI updates │
│ • Draw() - Rendering pipeline                    │
├─────────────────────────────────────────────────┤
│              Core Systems                       │
├─────────────────────────────────────────────────┤
│ • Battle System - Combat mechanics              │
│ • World System - Map navigation                 │
│ • Social System - Guilds, chat, friends         │
│ • UI System - Interface components              │
│ • Network System - Server communication         │
└─────────────────────────────────────────────────┘
```

### Server Architecture
```
┌─────────────────────────────────────────────────┐
│              Server Core                        │
├─────────────────────────────────────────────────┤
│ • Hub - Central message routing                 │
│ • Game Service - Battle management              │
│ • Social Service - Guild/friend operations      │
│ • Auth Service - User authentication            │
│ • Data Service - Persistence layer              │
└─────────────────────────────────────────────────┘
```

## Key Technical Decisions

### Language & Framework
- **Go 1.24.6**: Chosen for cross-platform compatibility, performance, and concurrency features
- **Ebiten**: Lightweight 2D game library with excellent Go integration and mobile support
- **No external dependencies**: Pure Go implementation for easier deployment and maintenance

### Networking Architecture
- **WebSocket Protocol**: Real-time bidirectional communication between client and server
- **Custom Protocol Layer**: Shared protocol package defines message types and serialization
- **Connection Management**: Automatic reconnection and heartbeat monitoring
- **Message Queueing**: Client-side message buffering for reliable delivery

### Data Persistence
- **JSON File Storage**: Simple, human-readable format for game data
- **In-Memory Caching**: Server maintains active game state in memory
- **Atomic Writes**: File operations use temporary files and atomic moves
- **Backup Strategy**: Regular data snapshots for recovery

### State Management
- **Client-Side State**: Local game state with server synchronization
- **Server Authoritative**: Server maintains true game state, client mirrors
- **Event-Driven Updates**: State changes trigger UI updates and network messages
- **Optimistic Updates**: Client predicts changes, server validates and corrects

## Design Patterns

### Observer Pattern (Game State)
```go
type GameState struct {
    observers []StateObserver
    // ... state fields
}

func (gs *GameState) AddObserver(obs StateObserver) {
    gs.observers = append(gs.observers, obs)
}

func (gs *GameState) NotifyObservers() {
    for _, obs := range gs.observers {
        obs.OnStateChanged(gs)
    }
}
```

### Component Pattern (UI System)
```go
type UIComponent interface {
    Update()
    Draw(screen *ebiten.Image)
    HandleInput(x, y int) bool
}

type CompositeComponent struct {
    children []UIComponent
    // ... layout logic
}
```

### Command Pattern (Battle Actions)
```go
type BattleCommand interface {
    Execute(battle *Battle) error
    Undo(battle *Battle) error
}

type MoveCommand struct {
    unitID int
    from, to Position
}
```

### Factory Pattern (Unit Creation)
```go
type UnitFactory struct {
    unitTemplates map[string]UnitTemplate
}

func (f *UnitFactory) CreateUnit(unitType string) (*Unit, error) {
    template, exists := f.unitTemplates[unitType]
    if !exists {
        return nil, errors.New("unknown unit type")
    }
    return template.Instantiate(), nil
}
```

## Component Relationships

### Core Dependencies
```
Battle System
    ↓ depends on
Unit System ←→ World System
    ↓ depends on      ↓
Network System ←→ UI System
         ↓
    Server API
```

### Data Flow
```
User Input → UI Components → Game Logic → Network Messages → Server
                                      ↓
                                 Local State Updates
                                      ↓
                                 UI Rendering
```

### Critical Implementation Paths

#### Battle Execution Path
1. **Input Processing**: Player selects unit and action
2. **Validation**: Client validates action locally
3. **Network Transmission**: Send action to server
4. **Server Processing**: Validate and execute on server
5. **State Update**: Broadcast new state to all clients
6. **Client Update**: Apply state changes and update UI
7. **Animation**: Play battle animations and effects

#### Network Message Path
1. **Message Creation**: Client creates protocol message
2. **Serialization**: Convert to JSON/binary format
3. **Transmission**: Send via WebSocket connection
4. **Server Reception**: Deserialize and route to handler
5. **Processing**: Execute game logic
6. **Response**: Send result back to client(s)
7. **Client Handling**: Update local state and UI

#### UI Rendering Path
1. **State Check**: Determine current game state
2. **Layout Calculation**: Position UI elements
3. **Asset Loading**: Load required textures/fonts
4. **Draw Calls**: Render components in correct order
5. **Post-Processing**: Apply effects and overlays
6. **Display**: Present final frame

## Performance Patterns

### Memory Management
- **Object Pooling**: Reuse common objects (particles, UI elements)
- **Asset Streaming**: Load assets on-demand, unload unused
- **Garbage Collection**: Minimize allocations during gameplay

### Rendering Optimization
- **Sprite Batching**: Group similar draw calls
- **Viewport Culling**: Only render visible elements
- **Texture Atlasing**: Combine small textures into larger ones

### Network Optimization
- **Message Compression**: Compress large state updates
- **Delta Encoding**: Send only changed state data
- **Rate Limiting**: Prevent message spam and DDoS

## Error Handling Patterns

### Network Errors
- **Graceful Degradation**: Continue with local state when disconnected
- **Automatic Retry**: Exponential backoff for failed connections
- **State Reconciliation**: Sync state when reconnection occurs

### Validation Errors
- **Client-Side Validation**: Prevent invalid actions before sending
- **Server Authoritative**: Always validate on server side
- **User Feedback**: Clear error messages for invalid actions

### Resource Errors
- **Fallback Assets**: Default textures when loading fails
- **Progressive Loading**: Load critical assets first
- **Error Recovery**: Attempt to reload failed resources
