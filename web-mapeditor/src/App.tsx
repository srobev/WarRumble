import React, { useEffect, useRef, useState, useCallback } from 'react';
import { MapDef, PointF } from './types/MapDef';
import { EditorState, LAYER_NAMES, SelectionKind } from './types/EditorState';
import { CoordinateTransformer } from './systems/CoordinateTransformer';
import { InputManager } from './systems/InputManager';
import { RenderEngine } from './systems/RenderEngine';
import { WebSocketClient, createWebSocketClient } from './systems/WebSocketClient';
import { MapBrowser } from './components/MapBrowser';
import { AssetBrowser } from './components/AssetBrowser';
import { CameraPreviewWindow } from './components/CameraPreviewWindow';

const App: React.FC = () => {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // Core systems
  const [transformer, setTransformer] = useState<CoordinateTransformer>();
  const [inputManager, setInputManager] = useState<InputManager>();
  const [renderEngine, setRenderEngine] = useState<RenderEngine>();
  const [wsClient, setWsClient] = useState<WebSocketClient>();

  // Editor state
  const [editorState, setEditorState] = useState<EditorState>({
    // Connection
    isConnected: false,

    // Authentication
    isAuthenticated: false,
    showLogin: false,
    loginUser: '',
    loginPass: '',

    // Map data
    mapDef: createEmptyMap(),

    // UI state
    currentLayer: 'BG',
    showAllLayers: false,
    showGrid: false,
    showHelp: false,
    helpMode: false,

    // Selection
    selKind: '',
    selIndex: -1,
    selHandle: -1,

    // Camera
    cameraX: 0,
    cameraY: 0,
    cameraZoom: 1.0,
    cameraMinZoom: 0.1,
    cameraMaxZoom: 2.0,
    cameraDragging: false,
    cameraDragStartX: 0,
    cameraDragStartY: 0,

    // Base management
    playerBaseExists: false,
    enemyBaseExists: false,

    // Frame manipulation
    frameX: 0.5,
    frameY: 0.5,
    frameWidth: 1.0,
    frameHeight: 1.0,
    frameScale: 1.0,
    frameDragging: false,
    frameDragStartX: 0,
    frameDragStartY: 0,
    frameDragHandle: 0,

    // Camera preview window
    showCameraPreview: false,
    previewDragging: false,
    previewDragStartX: 0,
    previewDragStartY: 0,
    previewOffsetX: 20,
    previewOffsetY: 0,
    previewWidth: 250,
    previewHeight: 200,

    // Temporary objects
    tmpLane: [],

    // Dragging state
    dragging: false,
    lastMx: 0,
    lastMy: 0,

    // Browser states
    showMapBrowser: false,
    showAssetsBrowser: false,
    showObstaclesBrowser: false,
    showDecorativeBrowser: false,
    availableMaps: [],
    availableAssets: [],
    availableObstacles: [],
    availableDecorative: [],

    // Browser selections
    mapBrowserSel: -1,
    mapBrowserScroll: 0,
    assetsBrowserSel: -1,
    assetsBrowserScroll: 0,
    obstaclesBrowserSel: -1,
    obstaclesBrowserScroll: 0,
    decorativeBrowserSel: -1,
    decorativeBrowserScroll: 0,

    // File paths
    assetsCurrentPath: './assets/maps',
    obstaclesCurrentPath: './assets/obstacles',
    decorativeCurrentPath: './assets/decorative',

    // Name editing
    name: 'New Map',
    nameFocus: false,

    // Background input
    bgInput: '',
    bgFocus: false,

    // Status
    status: 'Ready',
    statusMessages: ['Web map editor initialized'],
    maxStatusMessages: 10,
    showStatusLog: true,
    statusLogX: 8,
    statusLogY: 0,
    statusLogDragging: false,
    statusLogDragStartX: 0,
    statusLogDragStartY: 0,

    // Tool selection
    tool: -1,
    undoStack: [],
    redoStack: [],
    maxUndoSteps: 50,
  });

  // Animation frame
  const animationFrameRef = useRef<number>();

  // Initialize systems
  const initializeSystems = useCallback(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (!canvas || !container) return;

    // Set canvas size
    const rect = container.getBoundingClientRect();
    canvas.width = rect.width;
    canvas.height = rect.height - 120; // Account for UI height

    // Create coordinate transformer
    const newTransformer = new CoordinateTransformer({
      width: rect.width,
      height: rect.height - 120,
      cameraX: editorState.cameraX,
      cameraY: editorState.cameraY,
      zoom: editorState.cameraZoom,
      bgWidth: editorState.mapDef.bg ? 800 : undefined, // Default BG size
      bgHeight: editorState.mapDef.bg ? 600 : undefined,
      topUIHeight: 120
    });

    // Create input manager
    const newInputManager = new InputManager(canvas);

    // Create render engine
    const newRenderEngine = new RenderEngine(canvas);
    newRenderEngine.updateTransformer(newTransformer);

    // Create WebSocket client
    const newWsClient = createWebSocketClient();
    setupWebSocketHandlers(newWsClient);

    setTransformer(newTransformer);
    setInputManager(newInputManager);
    setRenderEngine(newRenderEngine);
    setWsClient(newWsClient);

    // Start game loop
    if (animationFrameRef.current) {
      cancelAnimationFrame(animationFrameRef.current);
    }
    gameLoop();
  }, []);

  // Setup WebSocket event handlers
  const setupWebSocketHandlers = useCallback((client: WebSocketClient) => {
    client.onConnected = () => {
      setEditorState(prev => ({ ...prev, isConnected: true, status: 'Connected to server' }));
    };

    client.onDisconnected = () => {
      setEditorState(prev => ({ ...prev, isConnected: false, status: 'Disconnected from server' }));
    };

    client.onError = (error: string) => {
      setEditorState(prev => ({ ...prev, status: `Connection error: ${error}` }));
    };

    client.onMapReceived = (mapDef: MapDef) => {
      setEditorState(prev => ({ ...prev, mapDef, status: `Loaded map: ${mapDef.name}` }));
    };

    client.onErrorReceived = (error: string) => {
      setEditorState(prev => ({ ...prev, status: `Server error: ${error}` }));
    };

    client.onAuthenticated = () => {
      setEditorState(prev => ({ ...prev, isAuthenticated: true, showLogin: false, status: 'Authenticated' }));
    };
  }, []);

  // Game loop
  const gameLoop = useCallback(() => {
    animationFrameRef.current = requestAnimationFrame(() => {
      update();
      render();
      gameLoop();
    });
  }, []);

  // Update function (called every frame)
  const update = useCallback(() => {
    if (!inputManager) return;

    // Update input manager
    inputManager.update();

    // Handle camera controls
    handleCameraControls();

    // Handle object selection and manipulation
    handleObjectInteraction();

    // Handle keyboard shortcuts
    handleKeyboardShortcuts();
  }, [inputManager]);

  // Render function
  const render = useCallback(() => {
    if (!renderEngine || !transformer) return;

    // Clear canvas
    renderEngine.clear();

    // Render background
    renderEngine.drawBackground();

    // Render map elements
    renderMapElements();

    // Render UI overlays (grid, selection highlights, etc.)
    renderUIOverlays();
  }, [renderEngine, transformer]);

  // Handle camera controls
  const handleCameraControls = useCallback(() => {
    if (!inputManager || !transformer) return;

    setEditorState(prevState => {
      let newState = { ...prevState };

      // Mouse wheel zoom
      const wheelDelta = inputManager.getMouseState().wheelDelta;
      if (wheelDelta !== 0) {
        const mousePos = inputManager.getMousePosition();
        const zoomFactor = wheelDelta > 0 ? 1.1 : 0.9;

        newState.cameraZoom = Math.max(
          prevState.cameraMinZoom,
          Math.min(prevState.cameraMaxZoom, prevState.cameraZoom * zoomFactor)
        );

        // Zoom towards mouse cursor
        const normalizedMouse = transformer.screenToNormalized(mousePos);
        const centerX = 0.5;
        const centerY = 0.5;

        const deltaX = (normalizedMouse.x - centerX) * (1 - zoomFactor);
        const deltaY = (normalizedMouse.y - centerY) * (1 - zoomFactor);

        newState.cameraX = prevState.cameraX + deltaX * transformCoordinateToScreen(1, transformer) * prevState.cameraZoom;
        newState.cameraY = prevState.cameraY + deltaY * transformCoordinateToScreen(1, transformer) * prevState.cameraZoom;
      }

      // Middle mouse button panning
      if (inputManager.isMouseButtonPressed(1)) { // Middle button
        if (!prevState.cameraDragging) {
          const mousePos = inputManager.getMousePosition();
          newState.cameraDragging = true;
          newState.cameraDragStartX = mousePos.x;
          newState.cameraDragStartY = mousePos.y;
        } else {
          const currentMouse = inputManager.getMousePosition();
          const deltaX = currentMouse.x - prevState.cameraDragStartX;
          const deltaY = currentMouse.y - prevState.cameraDragStartY;

          newState.cameraX = prevState.cameraX - deltaX;
          newState.cameraY = prevState.cameraY - deltaY;

          newState.cameraDragStartX = currentMouse.x;
          newState.cameraDragStartY = currentMouse.y;
        }
      } else {
        newState.cameraDragging = false;
      }

      // Update transformer with new camera values
      if (transformer && (newState.cameraX !== prevState.cameraX ||
                          newState.cameraY !== prevState.cameraY ||
                          newState.cameraZoom !== prevState.cameraZoom)) {
        const container = containerRef.current;
        if (container) {
          const rect = container.getBoundingClientRect();
          transformer.updateViewport({
            width: rect.width,
            height: rect.height - 120,
            cameraX: newState.cameraX,
            cameraY: newState.cameraY,
            zoom: newState.cameraZoom,
            bgWidth: newState.mapDef.bg ? 800 : undefined,
            bgHeight: newState.mapDef.bg ? 600 : undefined,
            topUIHeight: 120
          });
        }
      }

      return newState;
    });
  }, [inputManager, transformer]);

  // Handle object selection and manipulation
  const handleObjectInteraction = useCallback(() => {
    if (!inputManager || !transformer) return;

    const mousePos = inputManager.getMousePosition();

    // Handle left mouse button
    if (inputManager.isMouseButtonJustPressed(0)) { // Left button
      if (transformer.isPointInCanvas(mousePos)) {
        const normalizedMouse = transformer.screenToNormalized(mousePos);

        // Check for object hits first
        const hitResult = findObjectAtPosition(normalizedMouse.x, normalizedMouse.y);

        if (hitResult.found) {
          // Select existing object
          setEditorState(prev => ({
            ...prev,
            selKind: hitResult.kind,
            selIndex: hitResult.index,
            selHandle: hitResult.handle,
            dragging: true,
            lastMx: mousePos.x,
            lastMy: mousePos.y,
            status: `Selected ${hitResult.kind} ${hitResult.index}`
          }));
        } else {
          // Create new object based on current layer
          createObjectAtPosition(normalizedMouse.x, normalizedMouse.y);
        }
      }
    }

    // Handle right mouse button (finalize lane or clear selection)
    if (inputManager.isMouseButtonJustPressed(2)) { // Right button
      if (editorState.tmpLane.length > 0) {
        // Finalize lane
        setEditorState(prev => ({
          ...prev,
          mapDef: {
            ...prev.mapDef,
            lanes: [...prev.mapDef.lanes, {
              points: [...prev.tmpLane],
              dir: 1
            }]
          },
          tmpLane: [],
          status: 'Lane created'
        }));
      } else {
        // Clear selection
        setEditorState(prev => ({
          ...prev,
          selKind: '',
          selIndex: -1,
          selHandle: -1,
          tmpLane: [],
          status: 'Selection cleared'
        }));
      }
    }
  }, [inputManager, transformer, editorState.tmpLane]);

  // Handle keyboard shortcuts
  const handleKeyboardShortcuts = useCallback(() => {
    if (!inputManager) return;

    // Ctrl+S: Save
    if (inputManager.isKeyPressed('Control') && inputManager.isKeyJustPressed('S')) {
      saveMap();
    }

    // Ctrl+O: Load map browser
    if (inputManager.isKeyPressed('Control') && inputManager.isKeyJustPressed('O')) {
      setEditorState(prev => ({ ...prev, showMapBrowser: !prev.showMapBrowser }));
    }

    // G: Toggle grid
    if (inputManager.isKeyJustPressed('g') || inputManager.isKeyJustPressed('G')) {
      setEditorState(prev => ({ ...prev, showGrid: !prev.showGrid }));
    }

    // Delete: Remove selected object
    if (inputManager.isKeyJustPressed('Delete')) {
      deleteSelectedObject();
    }

    // Escape: Clear selection
    if (inputManager.isKeyJustPressed('Escape')) {
      setEditorState(prev => ({
        ...prev,
        selKind: '',
        selIndex: -1,
        selHandle: -1,
        tmpLane: [],
        status: 'Selection cleared'
      }));
    }
  }, [inputManager]);

  // Render map elements
  const renderMapElements = useCallback(() => {
    if (!renderEngine || !transformer) return;

    const { mapDef, currentLayer, showAllLayers } = editorState;

    // Render based on current layer
    const shouldRenderLayer = (layer: string) =>
      showAllLayers || currentLayer === 'All' || currentLayer === layer;

    // Background elements
    if (shouldRenderLayer('BG')) {
      // Meeting stones
      mapDef.meetingStones.forEach((stone, index) => {
        const screenPos = transformer.normalizedToScreen({ x: stone.x, y: stone.y });
        const isSelected = editorState.selKind === 'stone' && editorState.selIndex === index;
        renderEngine.drawPointElement(screenPos, 'stone', isSelected);
      });

      // Gold mines
      mapDef.goldMines.forEach((mine, index) => {
        const screenPos = transformer.normalizedToScreen({ x: mine.x, y: mine.y });
        const isSelected = editorState.selKind === 'mine' && editorState.selIndex === index;
        renderEngine.drawPointElement(screenPos, 'mine', isSelected);
      });
    }

    // Deploy zones
    if (shouldRenderLayer('Deploy')) {
      mapDef.deployZones.forEach((zone, index) => {
        const screenPos = transformer.normalizedToScreen({ x: zone.x, y: zone.y });
        const isSelected = editorState.selKind === 'deploy' && editorState.selIndex === index;
        renderEngine.drawDeployZone(zone, screenPos, isSelected);
      });
    }

    // Movement lanes
    if (shouldRenderLayer('Lanes')) {
      mapDef.lanes.forEach((lane, index) => {
        const screenPoints = lane.points.map(point =>
          transformer.normalizedToScreen({ x: point.x, y: point.y })
        );
        const isSelected = editorState.selKind === 'lane' && editorState.selIndex === index;
        renderEngine.drawLane(lane, screenPoints, isSelected);
      });

      // Draw temporary lane
      if (editorState.tmpLane.length > 0) {
        const screenPoints = editorState.tmpLane.map(point =>
          transformer.normalizedToScreen({ x: point.x, y: point.y })
        );
        renderEngine.drawLane({ points: editorState.tmpLane, dir: 1 }, screenPoints, true);
      }
    }

    // Obstacles
    if (shouldRenderLayer('Obstacles')) {
      mapDef.obstacles.forEach((obstacle, index) => {
        const screenPos = transformer.normalizedToScreen({ x: obstacle.x, y: obstacle.y });
        const isSelected = editorState.selKind === 'obstacle' && editorState.selIndex === index;
        renderEngine.drawObstacle(obstacle, screenPos, undefined, isSelected);
      });
    }

    // Decorative elements
    if (shouldRenderLayer('Assets')) {
      mapDef.decorativeElements?.forEach((decorative, index) => {
        const screenPos = transformer.normalizedToScreen({ x: decorative.x, y: decorative.y });
        const isSelected = editorState.selKind === 'decorative' && editorState.selIndex === index;
        renderEngine.drawDecorative(decorative, screenPos, undefined, isSelected);
      });
    }

    // Bases
    if (shouldRenderLayer('Bases')) {
      if (mapDef.playerBase) {
        const screenPos = transformer.normalizedToScreen(mapDef.playerBase);
        const isSelected = editorState.selKind === 'playerbase';
        renderEngine.drawBase(screenPos, 'player', isSelected);
      }

      if (mapDef.enemyBase) {
        const screenPos = transformer.normalizedToScreen(mapDef.enemyBase);
        const isSelected = editorState.selKind === 'enemybase';
        renderEngine.drawBase(screenPos, 'enemy', isSelected);
      }
    }

    // Frame
    if (shouldRenderLayer('Frame')) {
      const center = transformer.normalizedToScreen({ x: 0.5, y: 0.5 });
      renderEngine.drawFrame(center, 400, 300); // Default frame size
    }
  }, [renderEngine, transformer, editorState]);

  // Render UI overlays
  const renderUIOverlays = useCallback(() => {
    if (!renderEngine || !transformer) return;

    // Draw grid if enabled
    if (editorState.showGrid) {
      const canvasBounds = transformer.getCanvasBounds();
      renderEngine.drawGrid(
        canvasBounds.left,
        canvasBounds.top,
        canvasBounds.right,
        canvasBounds.bottom,
        20 / editorState.cameraZoom
      );
    }
  }, [renderEngine, transformer, editorState.showGrid, editorState.cameraZoom]);

  // Find object at position
  const findObjectAtPosition = useCallback((x: number, y: number): {
    found: boolean;
    kind: SelectionKind;
    index: number;
    handle: number;
  } => {
    // Check in reverse order (top to bottom)

    // Bases first
    if (editorState.mapDef.playerBase &&
        Math.abs(x - editorState.mapDef.playerBase.x) < 0.05 &&
        Math.abs(y - editorState.mapDef.playerBase.y) < 0.05) {
      return { found: true, kind: 'playerbase', index: -1, handle: -1 };
    }

    if (editorState.mapDef.enemyBase &&
        Math.abs(x - editorState.mapDef.enemyBase.x) < 0.05 &&
        Math.abs(y - editorState.mapDef.enemyBase.y) < 0.05) {
      return { found: true, kind: 'enemybase', index: -1, handle: -1 };
    }

    // Deploy zones
    for (let i = 0; i < editorState.mapDef.deployZones.length; i++) {
      const zone = editorState.mapDef.deployZones[i];
      if (x >= zone.x && x <= zone.x + zone.w &&
          y >= zone.y && y <= zone.y + zone.h) {
        return { found: true, kind: 'deploy', index: i, handle: -1 };
      }
    }

    // Default - no object found
    return { found: false, kind: '', index: -1, handle: -1 };
  }, [editorState.mapDef]);

  // Save current map
  const saveMap = useCallback(() => {
    if (!wsClient) {
      setEditorState(prev => ({ ...prev, status: 'No connection to server' }));
      return;
    }

    const success = wsClient.saveMap(editorState.mapDef);
    if (success) {
      setEditorState(prev => ({ ...prev, status: 'Saving map...' }));
    } else {
      setEditorState(prev => ({ ...prev, status: 'Failed to send save request' }));
    }
  }, [wsClient, editorState.mapDef]);

  // Delete selected object
  const deleteSelectedObject = useCallback(() => {
    setEditorState(prev => {
      if (prev.selKind === '') return prev;

      let newMapDef = { ...prev.mapDef };

      switch (prev.selKind) {
        case 'deploy':
          if (prev.selIndex >= 0 && prev.selIndex < newMapDef.deployZones.length) {
            newMapDef.deployZones = newMapDef.deployZones.filter((_, i) => i !== prev.selIndex);
          }
          break;
        case 'stone':
          if (prev.selIndex >= 0 && prev.selIndex < newMapDef.meetingStones.length) {
            newMapDef.meetingStones = newMapDef.meetingStones.filter((_, i) => i !== prev.selIndex);
          }
          break;
        case 'mine':
          if (prev.selIndex >= 0 && prev.selIndex < newMapDef.goldMines.length) {
            newMapDef.goldMines = newMapDef.goldMines.filter((_, i) => i !== prev.selIndex);
          }
          break;
        case 'lane':
          if (prev.selIndex >= 0 && prev.selIndex < newMapDef.lanes.length) {
            newMapDef.lanes = newMapDef.lanes.filter((_, i) => i !== prev.selIndex);
          }
          break;
        case 'obstacle':
          if (prev.selIndex >= 0 && prev.selIndex < newMapDef.obstacles.length) {
            newMapDef.obstacles = newMapDef.obstacles.filter((_, i) => i !== prev.selIndex);
          }
          break;
        case 'decorative':
          if (newMapDef.decorativeElements && prev.selIndex >= 0 && prev.selIndex < newMapDef.decorativeElements.length) {
            newMapDef.decorativeElements = newMapDef.decorativeElements.filter((_, i) => i !== prev.selIndex);
          }
          break;
        case 'playerbase':
          newMapDef.playerBase = undefined;
          break;
        case 'enemybase':
          newMapDef.enemyBase = undefined;
          break;
      }

      return {
        ...prev,
        mapDef: newMapDef,
        selKind: '',
        selIndex: -1,
        selHandle: -1,
        status: `${prev.selKind} deleted`
      };
    });
  }, [editorState.selKind, editorState.selIndex]);

  // Handle window resize
  const handleResize = useCallback(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (!canvas || !container) return;

    const rect = container.getBoundingClientRect();
    canvas.width = rect.width;
    canvas.height = rect.height - 120;

    // Update transformer if it exists
    if (transformer) {
      transformer.updateViewport({
        width: rect.width,
        height: rect.height - 120,
        cameraX: editorState.cameraX,
        cameraY: editorState.cameraY,
        zoom: editorState.cameraZoom,
        bgWidth: editorState.mapDef.bg ? 800 : undefined,
        bgHeight: editorState.mapDef.bg ? 600 : undefined,
        topUIHeight: 120
      });
    }
  }, [transformer, editorState]);

  // Lifecycle
  useEffect(() => {
    initializeSystems();

    // Handle window resize
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      if (animationFrameRef.current) {
        cancelAnimationFrame(animationFrameRef.current);
      }
      if (inputManager) {
        inputManager.destroy();
      }
    };
  }, [initializeSystems, handleResize, inputManager]);

  // Handle map selection from browser
  const handleMapSelected = useCallback((mapId: string) => {
    if (wsClient) {
      wsClient.requestMap(mapId);
    }
    setEditorState(prev => ({ ...prev, status: `Loading map: ${mapId}...` }));
  }, [wsClient]);

  // Handle asset selection from browser
  const handleAssetSelected = useCallback((assetPath: string, assetType: 'background' | 'obstacle' | 'decorative') => {
    if (assetType === 'background') {
      // Load background image
      if (renderEngine && wsClient) {
        const img = new Image();
        img.onload = () => {
          renderEngine.setBackground(img);
          setEditorState(prev => ({
            ...prev,
            mapDef: { ...prev.mapDef, bg: assetPath },
            status: `Background loaded: ${assetPath}`
          }));
        };
        img.onerror = () => {
          setEditorState(prev => ({ ...prev, status: `Failed to load background: ${assetPath}` }));
        };
        // Try to load from server assets
        img.src = `/api/assets/maps/${assetPath}`; // This would need a backend endpoint
      }
    } else {
      // Create new object at center of canvas
      const centerX = 0.5;
      const centerY = 0.5;

      if (assetType === 'obstacle') {
        setEditorState(prev => ({
          ...prev,
          mapDef: {
            ...prev.mapDef,
            obstacles: [...prev.mapDef.obstacles, {
              x: centerX - 0.05,
              y: centerY - 0.05,
              type: 'custom',
              image: assetPath,
              width: 0.1,
              height: 0.1
            }]
          },
          status: `Obstacle added: ${assetPath}`
        }));
      } else if (assetType === 'decorative') {
        setEditorState(prev => ({
          ...prev,
          mapDef: {
            ...prev.mapDef,
            decorativeElements: [
              ...(prev.mapDef.decorativeElements || []),
              {
                x: centerX - 0.05,
                y: centerY - 0.05,
                image: assetPath,
                width: 0.1,
                height: 0.1,
                layer: 1
              }
            ]
          },
          status: `Decorative element added: ${assetPath}`
        }));
      }
    }
  }, [renderEngine, wsClient]);

  // Create object at position based on current layer
  const createObjectAtPosition = useCallback((x: number, y: number) => {
    setEditorState(prev => {
      let newMapDef = { ...prev.mapDef };
      let status = 'Object created';

      switch (prev.currentLayer) {
        case 'BG': // Background layer - meeting stones and gold mines
          if (prev.tool === 1) { // Meeting stone tool
            newMapDef.meetingStones.push({ x, y });
            status = 'Meeting stone created';
          } else if (prev.tool === 2) { // Gold mine tool
            newMapDef.goldMines.push({ x, y });
            status = 'Gold mine created';
          } else {
            // Default to meeting stone
            newMapDef.meetingStones.push({ x, y });
            status = 'Meeting stone created (use 1 or 2 for gold mines)';
          }
          break;

        case 'Deploy': // Deployment zones
          newMapDef.deployZones.push({
            x: x - 0.05,
            y: y - 0.05,
            w: 0.1,
            h: 0.1,
            owner: 'player'
          });
          status = 'Deployment zone created';
          break;

        case 'Lanes': // Movement lanes
          status = `Lane creation not implemented (use tool selection)`;
          break;

        case 'Obstacles': // Obstacles
          newMapDef.obstacles.push({
            x: x - 0.05,
            y: y - 0.05,
            type: 'tree',
            image: 'tree.png',
            width: 0.1,
            height: 0.1
          });
          status = 'Obstacle created';
          break;

        case 'Assets': // Decorative elements
          if (!newMapDef.decorativeElements) {
            newMapDef.decorativeElements = [];
          }
          newMapDef.decorativeElements.push({
            x: x - 0.05,
            y: y - 0.05,
            image: 'decorative.png',
            width: 0.1,
            height: 0.1,
            layer: 1
          });
          status = 'Decorative element created';
          break;

        case 'Bases': // Player/enemy bases
          if (!prev.playerBaseExists) {
            newMapDef.playerBase = { x, y };
            status = 'Player base created';
          } else if (!prev.enemyBaseExists) {
            newMapDef.enemyBase = { x, y };
            status = 'Enemy base created';
          } else {
            status = 'Both bases already exist';
          }
          break;

        default:
          status = `Layer "${prev.currentLayer}" - click to create objects`;
          break;
      }

      return {
        ...prev,
        mapDef: newMapDef,
        status
      };
    });
  }, []);

  // Helper function (should be moved to utilities)
  function transformCoordinateToScreen(value: number, transformer: CoordinateTransformer): number {
    return value;
  }

  return (
    <div ref={containerRef} className="map-editor">
      {/* Canvas */}
      <canvas ref={canvasRef} />

      {/* Top UI Bar */}
      <div className="top-ui">
        <div className="top-ui-row">
          {/* Layer buttons */}
          <div className="layer-buttons">
            {LAYER_NAMES.map(layerName => (
              <button
                key={layerName}
                className={`editor-button ${editorState.currentLayer === layerName ? 'active' : ''}`}
                onClick={() => setEditorState(prev => ({ ...prev, currentLayer: layerName }))}
              >
                {layerName}
              </button>
            ))}
          </div>

          {/* Quick action buttons */}
          <button
            className="editor-button"
            onClick={() => setEditorState(prev => ({ ...prev, showGrid: !prev.showGrid }))}
          >
            Grid
          </button>

          <button
            className="editor-button"
            onClick={() => setEditorState(prev => ({ ...prev, showCameraPreview: !prev.showCameraPreview }))}
          >
            Camera Preview
          </button>
        </div>

        <div className="top-ui-row">
          {/* Main action buttons */}
          <button className="editor-button accent" onClick={saveMap}>
            Save
          </button>

          <button
            className="editor-button"
            onClick={() => setEditorState(prev => ({ ...prev, selKind: '', selIndex: -1, selHandle: -1 }))}
          >
            Clear
          </button>

          <button
            className="editor-button"
            onClick={() => setEditorState(prev => ({ ...prev, showAssetsBrowser: !prev.showAssetsBrowser }))}
          >
            Assets
          </button>

          <button
            className="editor-button"
            onClick={() => setEditorState(prev => ({ ...prev, showMapBrowser: !prev.showMapBrowser }))}
          >
            Load Map
          </button>

          <button
            className="editor-button"
            onClick={() => setEditorState(prev => ({ ...prev, helpMode: !prev.helpMode }))}
          >
            Help
          </button>
        </div>
      </div>

      {/* Status Display */}
      <div className="status-display">
        {editorState.status}
      </div>

      {/* Connection Status */}
      <div className={`connection-status ${editorState.isConnected ? 'connected' : 'disconnected'}`}>
        {editorState.isConnected ? '● Connected' : '● Disconnected'}
      </div>

      {/* Camera Preview Window */}
      {editorState.showCameraPreview && transformer && renderEngine && (
        <CameraPreviewWindow
          editorState={editorState}
          transformer={transformer}
          renderEngine={renderEngine}
          onToggle={() => setEditorState(prev => ({ ...prev, showCameraPreview: false }))}
          onDrag={(deltaX, deltaY) => {
            setEditorState(prev => ({
              ...prev,
              previewOffsetX: prev.previewOffsetX + deltaX,
              previewOffsetY: prev.previewOffsetY + deltaY
            }));
          }}
          onResize={(newWidth, newHeight) => {
            setEditorState(prev => ({
              ...prev,
              previewWidth: newWidth,
              previewHeight: newHeight
            }));
          }}
        />
      )}
    </div>
  );
};

// Create empty map
function createEmptyMap(): MapDef {
  return {
    id: 'new-map',
    name: 'New Map',
    deployZones: [],
    meetingStones: [],
    goldMines: [],
    lanes: [],
    obstacles: [],
    decorativeElements: []
  };
}

export default App;
