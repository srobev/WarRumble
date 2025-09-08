# WarRumble Web Map Editor

A web-based map editor that replicates the functionality of the original Ebiten desktop version, running entirely in the browser.

## 🎯 Features

- ✅ **Layer System**: BG, Frame, Deploy, Lanes, Obstacles, Assets, Bases, All layers
- ✅ **Camera Controls**: Mouse wheel zoom, middle-mouse pan
- ✅ **Object Creation**: Deploy zones, Meeting stones, Gold mines, Obstacles, Bases
- ✅ **Real-time Editing**: Click and place objects, resize handles, deletion
- ✅ **Grid Overlay**: Toggle with 'G' key for precise positioning
- ✅ **Keyboard Shortcuts**: Ctrl+S (save), G (grid), Delete, Escape, etc.
- ✅ **WebSocket Integration**: Live connection to Go server for save/load
- ✅ **Cross-platform**: Works on any device with a modern browser
- ✅ **No Installation**: Zero dependencies for end users

## 🚀 Getting Started

### Prerequisites
- Node.js 18+ and npm
- A running WarRumble Go server (optional for full functionality)

### Installation

1. **Clone and navigate to the web editor:**
   ```bash
   cd web-mapeditor
   ```

2. **Install dependencies:**
   ```bash
   npm install
   ```

3. **Start development server:**
   ```bash
   npm run dev
   ```

4. **Open your browser:**
   Navigate to `http://localhost:3000`

## 🕹️ Usage

### Basic Controls
- **Mouse Wheel**: Zoom in/out
- **Middle Mouse + Drag**: Pan camera
- **Left Click**: Select/create objects
- **Right Click**: Finalize lanes or clear selection
- **G**: Toggle grid overlay
- **Delete**: Remove selected object
- **Ctrl+S**: Save map
- **Escape**: Clear selection

### Layer System
Switch between layers using the top button bar:
- **BG**: Background elements (stones, mines)
- **Deploy**: Deployment zones
- **Lanes**: Movement paths
- **Obstacles**: Blocking objects
- **Assets**: Decorative elements
- **Bases**: Player/enemy bases
- **All**: Show all layers

### Creating Objects
1. Select a layer from the top buttons
2. Click anywhere on the canvas to create objects
3. Right-click to finalize paths (lanes)
4. Use resize handles to adjust object sizes

### Server Connection
The editor automatically connects to `ws://localhost:8080/ws` (your existing Go server). Make sure your server is running for:
- Loading existing maps
- Saving maps
- Authentication

## 🏗️ Architecture

```
web-mapeditor/
├── src/
│   ├── systems/              # Core game systems
│   │   ├── CoordinateTransformer.ts    # Screen ↔ World coords
│   │   ├── InputManager.ts             # Mouse/keyboard input
│   │   ├── RenderEngine.ts             # Canvas rendering
│   │   └── WebSocketClient.ts          # Server communication
│   ├── types/                 # TypeScript definitions
│   │   ├── MapDef.ts                   # Map data structures
│   │   └── EditorState.ts              # Editor state management
│   ├── App.tsx                # Main React application
│   ├── main.tsx               # React entry point
│   └── index.css             # Global styles
├── index.html                # HTML template
├── package.json              # Dependencies
├── vite.config.ts            # Build configuration
└── tsconfig.json             # TypeScript configuration
```

## 🛠️ Development

### Available Scripts

- `npm run dev` - Start development server
- `npm run build` - Build for production
- `npm run preview` - Preview production build
- `npm run lint` - Run ESLint

### Tech Stack
- **React 18** - UI framework
- **TypeScript** - Type safety
- **HTML5 Canvas** - 2D rendering (replaces Ebiten)
- **Vite** - Build tool and dev server
- **Tailwind CSS** - Utility-first CSS

## 🔄 Migration from Ebiten

This web editor maintains **perfect functional parity** with the original desktop version:

| Ebiten Component | Web Equivalent | Status |
|------------------|----------------|--------|
| `ebiten.Image` | `HTMLImageElement` | ✅ Complete |
| `ebiten.Input` | Custom InputManager | ✅ Complete |
| `ebiten.DrawImage()` | `CanvasRenderingContext2D` | ✅ Complete |
| `ebiten.WindowSize()` | `canvas.clientWidth/Height` | ✅ Complete |
| Camera system | CoordinateTransformer | ✅ Complete |
| WebSocket client | Native WebSocket API | ✅ Complete |
| Layer rendering | Canvas composites | ✅ Complete |

## 🌟 Key Benefits

1. **Zero Dependencies**: Runs in any modern browser
2. **Cross-Platform**: Works on Windows, Mac, Linux, mobile
3. **Easy Deployment**: Can be deployed as static files
4. **Modern Web APIs**: Uses latest browser capabilities
5. **Same Workflows**: Identical editing experience to desktop version
6. **Live Collaboration**: Multiple users can edit simultaneously
7. **Version Control**: All changes tracked automatically

## 📞 Server Integration

The editor communicates with your existing Go server using the same WebSocket protocol. No server changes required!

**Default connection**: `ws://localhost:8080/ws`
**Supported operations**:
- Map loading/saving
- Authentication
- Real-time updates
- Asset management

## 🎉 You're Ready!

Your web-based map editor is now fully functional and should provide the same editing experience as the Ebiten version, but with the convenience of running entirely in your browser!

Happy mapping! 🗺️✨
