import { WSMessage, MapDef, MapDefMessage, ErrorMessage, GetMapRequest, SaveMapRequest } from '../types/MapDef';

/**
 * WebSocketClient handles communication with the Go server
 * Replicates the WebSocket functionality from the Ebiten version
 */
export class WebSocketClient {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000; // Start with 1 second
  private url: string;
  private token?: string;

  // Event callbacks
  onConnected?: () => void;
  onDisconnected?: () => void;
  onError?: (error: string) => void;
  onMapReceived?: (mapDef: MapDef) => void;
  onErrorReceived?: (error: string) => void;
  onAuthenticated?: () => void;

  constructor(url: string, token?: string) {
    this.url = url;
    this.token = token;
  }

  /**
   * Connect to the WebSocket server
   */
  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        resolve();
        return;
      }

      try {
        // Create WebSocket URL with token if available
        const wsUrl = this.token ? `${this.url}?token=${this.token}` : this.url;
        this.ws = new WebSocket(wsUrl);

        this.ws.onopen = () => {
          console.log('WebSocket connected');
          this.reconnectAttempts = 0;
          this.reconnectDelay = 1000;
          if (this.onConnected) {
            this.onConnected();
          }
          resolve();
        };

        this.ws.onmessage = (event) => {
          try {
            const message: WSMessage = JSON.parse(event.data);
            this.handleMessage(message);
          } catch (error) {
            console.error('Failed to parse WebSocket message:', error);
            if (this.onError) {
              this.onError('Invalid message format');
            }
          }
        };

        this.ws.onclose = (event) => {
          console.log('WebSocket disconnected:', event.code, event.reason);
          if (this.onDisconnected) {
            this.onDisconnected();
          }

          // Attempt to reconnect if not a normal closure
          if (event.code !== 1000 && this.reconnectAttempts < this.maxReconnectAttempts) {
            setTimeout(() => {
              this.reconnectAttempts++;
              console.log(`Reconnecting... (${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
              this.connect().catch(() => {
                // Reconnection failed, will try again in next interval
              });
            }, this.reconnectDelay);
            this.reconnectDelay *= 2; // Exponential backoff
          }
        };

        this.ws.onerror = (event) => {
          console.error('WebSocket error:', event);
          if (this.onError) {
            this.onError('WebSocket connection error');
          }
          reject(new Error('WebSocket connection failed'));
        };

      } catch (error) {
        console.error('Failed to create WebSocket:', error);
        reject(error);
      }
    });
  }

  /**
   * Disconnect from the WebSocket server
   */
  disconnect() {
    if (this.ws) {
      this.ws.close(1000, 'Normal closure');
      this.ws = null;
    }
  }

  /**
   * Send a message to the server
   */
  sendMessage<T>(message: T): boolean {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.warn('WebSocket is not connected. Message not sent:', message);
      return false;
    }

    try {
      this.ws.send(JSON.stringify(message));
      return true;
    } catch (error) {
      console.error('Failed to send WebSocket message:', error);
      return false;
    }
  }

  /**
   * Request a map from the server
   */
  requestMap(mapId: string): boolean {
    const message: WSMessage = {
      type: 'GetMap',
      data: { ID: mapId } as GetMapRequest
    };
    return this.sendMessage(message);
  }

  /**
   * Send a map to save on the server
   */
  saveMap(mapDef: MapDef): boolean {
    const message: WSMessage = {
      type: 'SaveMap',
      data: { Def: mapDef } as SaveMapRequest
    };
    return this.sendMessage(message);
  }

  /**
   * Send authentication credentials
   */
  authenticate(username: string, password: string): boolean {
    const message: WSMessage = {
      type: 'Authenticate',
      data: { username, password }
    };
    return this.sendMessage(message);
  }

  /**
   * Request list of available maps
   */
  requestMapList(): boolean {
    const message: WSMessage = {
      type: 'ListMaps',
      data: {}
    };
    return this.sendMessage(message);
  }

  /**
   * Handle incoming messages from the server
   */
  private handleMessage(message: WSMessage) {
    console.log('Received message:', message.type, message);

    switch (message.type) {
      case 'MapDef':
        const mapMessage = message.data as MapDefMessage;
        if (this.onMapReceived) {
          this.onMapReceived(mapMessage.Def);
        }
        break;

      case 'Error':
        const errorMessage = message.data as ErrorMessage;
        if (this.onErrorReceived) {
          this.onErrorReceived(errorMessage.message);
        }
        break;

      case 'Authentication':
        const authData = message.data as { success: boolean; token?: string };
        if (authData.success) {
          if (authData.token) {
            this.token = authData.token;
          }
          if (this.onAuthenticated) {
            this.onAuthenticated();
          }
        } else {
          if (this.onErrorReceived) {
            this.onErrorReceived('Authentication failed');
          }
        }
        break;

      case 'MapsList':
        const mapsList = message.data as { maps: string[] };
        // Handle maps list - could trigger a callback
        console.log('Available maps:', mapsList.maps);
        break;

      default:
        console.warn('Unknown message type:', message.type);
    }
  }

  /**
   * Check if WebSocket is currently connected
   */
  isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN;
  }

  /**
   * Get current connection state
   */
  getConnectionState(): string {
    if (!this.ws) return 'disconnected';

    switch (this.ws.readyState) {
      case WebSocket.CONNECTING: return 'connecting';
      case WebSocket.OPEN: return 'connected';
      case WebSocket.CLOSING: return 'closing';
      case WebSocket.CLOSED: return 'disconnected';
      default: return 'unknown';
    }
  }

  /**
   * Update the WebSocket URL
   */
  setUrl(url: string) {
    this.url = url;
    // If currently connected, disconnect and prepare for reconnect
    if (this.isConnected()) {
      this.disconnect();
    }
  }

  /**
   * Update authentication token
   */
  setToken(token: string) {
    this.token = token;
  }

  /**
   * Get current token
   */
  getToken(): string | undefined {
    return this.token;
  }

  /**
   * Reset reconnection attempts (useful after successful connection)
   */
  resetReconnectAttempts() {
    this.reconnectAttempts = 0;
    this.reconnectDelay = 1000;
  }
}

/**
 * Create a WebSocket client with default settings
 */
export function createWebSocketClient(serverUrl?: string): WebSocketClient {
  const defaultUrl = serverUrl || `ws://${window.location.hostname}:8080/ws`;
  return new WebSocketClient(defaultUrl);
}

/**
 * Environment variable helper (for development)
 */
function getEnvVar(key: string, defaultValue?: string): string | undefined {
  // In browser environment, we can't access environment variables directly
  // This is a placeholder for any browser-specific config
  const env = (window as any).ENV || {};
  return env[key] || defaultValue;
}
