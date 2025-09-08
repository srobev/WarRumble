import { MapDef, PointF } from './MapDef';

// Selection and editing types
export type SelectionKind = '' | 'deploy' | 'stone' | 'mine' | 'lane' | 'obstacle' | 'decorative' | 'frame' | 'playerbase' | 'enemybase';

// Layer system
export const LAYER_NAMES = [
  'BG', 'Frame', 'Deploy', 'Lanes', 'Obstacles', 'Assets', 'Bases', 'All'
] as const;

export type LayerType = typeof LAYER_NAMES[number];

// Tool types (for creating new objects)
export const TOOL_NAMES = [
  'Deploy Zones', 'Meeting Stones', 'Gold Mines', 'Movement Lanes', 'Obstacles', 'Decorative'
] as const;

export type ToolType = typeof TOOL_NAMES[number];
export type ToolIndex = 0 | 1 | 2 | 3 | 4 | 5;

// Editor state interface
export interface EditorState {
  // Connection
  ws?: WebSocket;
  isConnected: boolean;

  // Authentication
  isAuthenticated: boolean;
  showLogin: boolean;
  loginUser: string;
  loginPass: string;
  authToken?: string;

  // Map data
  mapDef: MapDef;
  originalMapDef?: MapDef; // For change tracking

  // UI state
  currentLayer: LayerType;
  showAllLayers: boolean;
  showGrid: boolean;
  showHelp: boolean;
  helpMode: boolean;

  // Selection
  selKind: SelectionKind;
  selIndex: number;
  selHandle: number;

  // Camera
  cameraX: number;
  cameraY: number;
  cameraZoom: number;
  cameraMinZoom: number;
  cameraMaxZoom: number;
  cameraDragging: boolean;
  cameraDragStartX: number;
  cameraDragStartY: number;

  // Base management
  playerBaseExists: boolean;
  enemyBaseExists: boolean;

  // Frame manipulation
  frameX: number;
  frameY: number;
  frameWidth: number;
  frameHeight: number;
  frameScale: number;
  frameDragging: boolean;
  frameDragStartX: number;
  frameDragStartY: number;
  frameDragHandle: number;

  // Camera preview window
  showCameraPreview: boolean;
  previewDragging: boolean;
  previewDragStartX: number;
  previewDragStartY: number;
  previewOffsetX: number;
  previewOffsetY: number;
  previewWidth: number;
  previewHeight: number;

  // Temporary objects (for drawing lanes, etc.)
  tmpLane: PointF[];

  // Dragging state
  dragging: boolean;
  lastMx: number;
  lastMy: number;

  // Browser states
  showMapBrowser: boolean;
  showAssetsBrowser: boolean;
  showObstaclesBrowser: boolean;
  showDecorativeBrowser: boolean;
  availableMaps: string[];
  availableAssets: string[];
  availableObstacles: string[];
  availableDecorative: string[];

  // Browser selections
  mapBrowserSel: number;
  mapBrowserScroll: number;
  assetsBrowserSel: number;
  assetsBrowserScroll: number;
  obstaclesBrowserSel: number;
  obstaclesBrowserScroll: number;
  decorativeBrowserSel: number;
  decorativeBrowserScroll: number;

  // File paths
  assetsCurrentPath: string;
  obstaclesCurrentPath: string;
  decorativeCurrentPath: string;
  bgPath?: string;

  // Name editing
  name: string;
  nameFocus: boolean;

  // Background input
  bgInput: string;
  bgFocus: boolean;

  // Status and logging
  status: string;
  statusMessages: string[];
  maxStatusMessages: number;
  showStatusLog: boolean;
  statusLogX: number;
  statusLogY: number;
  statusLogDragging: boolean;
  statusLogDragStartX: number;
  statusLogDragStartY: number;

  // Tool selection
  tool: ToolIndex | -1;

  // Undo/Redo (for future implementation)
  undoStack: MapDef[];
  redoStack: MapDef[];
  maxUndoSteps: number;
}
