// Core geometry types for maps
export interface PointF {
  x: number;
  y: number;
}

export interface RectF {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface DeployZone {
  x: number;
  y: number;
  w: number;
  h: number;
  owner: string; // "player" or "enemy"
}

export interface Lane {
  points: PointF[];
  dir: number; // 1 or -1 (flow direction)
}

export interface Obstacle {
  x: number;
  y: number;
  type: string; // obstacle type (e.g., "tree", "rock", "building")
  image: string; // image path
  width: number; // normalized width (0-1)
  height: number; // normalized height (0-1)
}

export interface DecorativeElement {
  x: number;
  y: number;
  image: string; // image path
  width: number; // normalized width (0-1)
  height: number; // normalized height (0-1)
  layer: number; // rendering layer (0=background, 1=middle, 2=foreground)
}

// MapDef describes a PVE map layout for gameplay
export interface MapDef {
  id: string;
  name: string;
  width?: number; // background width in pixels (optional)
  height?: number; // background height in pixels (optional)
  bg?: string; // optional background image path

  // Background positioning and scaling
  bgScale?: number; // background scale factor
  bgOffsetX?: number; // background X offset
  bgOffsetY?: number; // background Y offset

  // Frame boundaries for camera movement limits
  frameX?: number; // frame center X position (normalized 0-1)
  frameY?: number; // frame center Y position (normalized 0-1)
  frameWidth?: number; // frame width (normalized 0-1)
  frameHeight?: number; // frame height (normalized 0-1)

  deployZones: DeployZone[];
  meetingStones: PointF[];
  goldMines: PointF[];
  lanes: Lane[];
  obstacles: Obstacle[];
  decorativeElements?: DecorativeElement[];

  // Base positions for PvP (configurable per map)
  playerBase?: PointF; // Player base position (normalized 0-1)
  enemyBase?: PointF; // Enemy base position (normalized 0-1)

  // Match timer configuration
  timeLimit?: number; // Time limit in seconds (default 180 = 3:00)

  // Arena mode (for PvP) - if true, bottom 50% is mirrored to top 50%
  isArena?: boolean; // Whether this is an arena map (auto-mirrors bottom to top)
}

// WebSocket message types
export interface WSMessage {
  type: string;
  data: any;
}

export interface GetMapRequest {
  ID: string;
}

export interface SaveMapRequest {
  Def: MapDef;
}

// Server responses
export interface MapDefMessage {
  Def: MapDef;
}

export interface ErrorMessage {
  message: string;
}

export interface MapsList {
  maps: string[];
}
