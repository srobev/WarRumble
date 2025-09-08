import { NormalizedPoint, ScreenPoint } from './CoordinateTransformer';

export interface MouseState {
  position: ScreenPoint;
  buttons: Set<number>;
  wheelDelta: number;
  isDragging: boolean;
  dragStart: ScreenPoint;
}

export interface KeyboardState {
  keys: Set<string>;
  modifiers: {
    ctrl: boolean;
    shift: boolean;
    alt: boolean;
    meta: boolean;
  };
}

/**
 * InputManager handles all user input (mouse and keyboard) and provides
 * a unified interface similar to Ebiten's input system
 */
export class InputManager {
  private mouseState: MouseState;
  private keyboardState: KeyboardState;
  private eventTarget: HTMLElement;

  // Event caches
  private readonly keysJustPressed: Set<string>;
  private readonly keysJustReleased: Set<string>;
  private readonly mouseButtonsJustPressed: Set<number>;
  private readonly mouseButtonsJustReleased: Set<number>;
  private inputChars: string;

  // Callbacks
  onMouseMove?: (mouse: MouseState) => void;
  onMouseDown?: (button: number, mouse: MouseState) => void;
  onMouseUp?: (button: number, mouse: MouseState) => void;
  onWheel?: (delta: number, mouse: MouseState) => void;
  onKeyDown?: (key: string, keyboard: KeyboardState) => void;
  onKeyUp?: (key: string, keyboard: KeyboardState) => void;

  constructor(eventTarget: HTMLElement) {
    this.eventTarget = eventTarget;
    this.mouseState = {
      position: { x: 0, y: 0 },
      buttons: new Set(),
      wheelDelta: 0,
      isDragging: false,
      dragStart: { x: 0, y: 0 }
    };
    this.keyboardState = {
      keys: new Set(),
      modifiers: {
        ctrl: false,
        shift: false,
        alt: false,
        meta: false
      }
    };

    this.keysJustPressed = new Set();
    this.keysJustReleased = new Set();
    this.mouseButtonsJustPressed = new Set();
    this.mouseButtonsJustReleased = new Set();
    this.inputChars = '';

    this.setupEventListeners();
  }

  private setupEventListeners() {
    // Mouse events
    this.eventTarget.addEventListener('mousemove', (e) => {
      this.handleMouseMove(e);
    });

    this.eventTarget.addEventListener('mousedown', (e) => {
      this.handleMouseDown(e);
    });

    this.eventTarget.addEventListener('mouseup', (e) => {
      this.handleMouseUp(e);
    });

    this.eventTarget.addEventListener('wheel', (e) => {
      this.handleWheel(e);
    });

    // Keyboard events
    this.eventTarget.addEventListener('keydown', (e) => {
      this.handleKeyDown(e);
    });

    this.eventTarget.addEventListener('keyup', (e) => {
      this.handleKeyUp(e);
    });

    // Input events for text input
    this.eventTarget.addEventListener('input', (e) => {
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        // Handle input elements separately
        return;
      }
      this.handleInput(e);
    });

    // Prevent context menu on right click
    this.eventTarget.addEventListener('contextmenu', (e) => {
      e.preventDefault();
    });

    // Focus management
    this.eventTarget.addEventListener('focus', () => {
      this.clearInputState();
    });

    this.eventTarget.addEventListener('blur', () => {
      this.clearInputState();
    });
  }

  private handleMouseMove(event: MouseEvent) {
    const rect = this.eventTarget.getBoundingClientRect();
    this.mouseState.position = {
      x: event.clientX - rect.left,
      y: event.clientY - rect.top
    };

    // Update drag state
    if (this.mouseState.buttons.size > 0) {
      this.mouseState.isDragging = true;
    }

    if (this.onMouseMove) {
      this.onMouseMove(this.mouseState);
    }
  }

  private handleMouseDown(event: MouseEvent) {
    event.preventDefault();
    this.mouseState.buttons.add(event.button);

    if (!this.mouseState.isDragging) {
      this.mouseState.dragStart = { ...this.mouseState.position };
    }

    this.mouseButtonsJustPressed.add(event.button);

    if (this.onMouseDown) {
      this.onMouseDown(event.button, this.mouseState);
    }
  }

  private handleMouseUp(event: MouseEvent) {
    event.preventDefault();
    this.mouseState.buttons.delete(event.button);
    this.mouseButtonsJustReleased.add(event.button);

    // Reset drag state when all buttons are released
    if (this.mouseState.buttons.size === 0) {
      this.mouseState.isDragging = false;
    }

    if (this.onMouseUp) {
      this.onMouseUp(event.button, this.mouseState);
    }
  }

  private handleWheel(event: WheelEvent) {
    event.preventDefault();
    this.mouseState.wheelDelta = event.deltaY;

    if (this.onWheel) {
      this.onWheel(event.deltaY, this.mouseState);
    }
  }

  private handleKeyDown(event: KeyboardEvent) {
    const key = this.normalizeKeyName(event.key);
    if (!this.keyboardState.keys.has(key)) {
      this.keyboardState.keys.add(key);
      this.keysJustPressed.add(key);
    }

    // Update modifiers
    this.updateModifiers(event);

    if (this.onKeyDown) {
      this.onKeyDown(key, this.keyboardState);
    }
  }

  private handleKeyUp(event: KeyboardEvent) {
    const key = this.normalizeKeyName(event.key);
    if (this.keyboardState.keys.has(key)) {
      this.keyboardState.keys.delete(key);
      this.keysJustReleased.add(key);
    }

    // Update modifiers
    this.updateModifiers(event);

    if (this.onKeyUp) {
      this.onKeyUp(key, this.keyboardState);
    }
  }

  private handleInput(event: Event) {
    // This is for character input (useful for text fields)
    if (event instanceof InputEvent && event.data) {
      this.inputChars += event.data;
    }
  }

  private updateModifiers(event: KeyboardEvent) {
    this.keyboardState.modifiers = {
      ctrl: event.ctrlKey,
      shift: event.shiftKey,
      alt: event.altKey,
      meta: event.metaKey
    };
  }

  private normalizeKeyName(key: string): string {
    // Normalize key names to match common conventions
    switch (key.toLowerCase()) {
      case 'escape': return 'Escape';
      case 'enter': return 'Enter';
      case ' ': return 'Space';
      case 'arrowup': return 'ArrowUp';
      case 'arrowdown': return 'ArrowDown';
      case 'arrowleft': return 'ArrowLeft';
      case 'arrowright': return 'ArrowRight';
      case 'backspace': return 'Backspace';
      case 'delete': return 'Delete';
      case 'tab': return 'Tab';
      case 'control': return 'Control';
      case 'shift': return 'Shift';
      case 'alt': return 'Alt';
      case 'meta': return 'Meta';
      default: return key;
    }
  }

  private clearInputState() {
    this.mouseState.buttons.clear();
    this.keyboardState.keys.clear();
    this.mouseState.isDragging = false;
  }

  // Public API methods (similar to Ebiten)

  /**
   * Get current mouse position
   */
  getMousePosition(): ScreenPoint {
    return { ...this.mouseState.position };
  }

  /**
   * Check if a mouse button is currently pressed
   */
  isMouseButtonPressed(button: number): boolean {
    return this.mouseState.buttons.has(button);
  }

  /**
   * Check if a mouse button was just pressed this frame
   */
  isMouseButtonJustPressed(button: number): boolean {
    return this.mouseButtonsJustPressed.has(button);
  }

  /**
   * Check if a mouse button was just released this frame
   */
  isMouseButtonJustReleased(button: number): boolean {
    return this.mouseButtonsJustReleased.has(button);
  }

  /**
   * Check if a key is currently pressed
   */
  isKeyPressed(key: string): boolean {
    return this.keyboardState.keys.has(key);
  }

  /**
   * Check if a key was just pressed this frame
   */
  isKeyJustPressed(key: string): boolean {
    return this.keysJustPressed.has(key);
  }

  /**
   * Check if a key was just released this frame
   */
  isKeyJustReleased(key: string): boolean {
    return this.keysJustReleased.has(key);
  }

  /**
   * Get input characters entered this frame
   */
  appendInputChars(): string {
    const chars = this.inputChars;
    this.inputChars = '';
    return chars;
  }

  /**
   * Get just-pressed keys (similar to ebiten.Input)
   */
  appendJustPressedKeys(): string[] {
    const keys = Array.from(this.keysJustPressed);
    return keys;
  }

  /**
   * Get modifier states
   */
  getModifiers() {
    return { ...this.keyboardState.modifiers };
  }

  /**
   * Update method to be called each frame to clear just-pressed/released states
   */
  update() {
    this.keysJustPressed.clear();
    this.keysJustReleased.clear();
    this.mouseButtonsJustPressed.clear();
    this.mouseButtonsJustReleased.clear();
    this.mouseState.wheelDelta = 0;
  }

  /**
   * Get current mouse state
   */
  getMouseState(): Readonly<MouseState> {
    return this.mouseState;
  }

  /**
   * Get current keyboard state
   */
  getKeyboardState(): Readonly<KeyboardState> {
    return this.keyboardState;
  }

  /**
   * Force clear all input states (useful when switching contexts)
   */
  clearAllInput() {
    this.clearInputState();
    this.keysJustPressed.clear();
    this.keysJustReleased.clear();
    this.mouseButtonsJustPressed.clear();
    this.mouseButtonsJustReleased.clear();
    this.inputChars = '';
    this.mouseState.wheelDelta = 0;
    this.mouseState.isDragging = false;
  }

  /**
   * Cleanup method - remove event listeners
   */
  destroy() {
    // Note: In a real implementation, you'd remove all the event listeners here
    // For simplicity, we'll rely on garbage collection when the eventTarget is removed
    this.clearAllInput();
  }
}
