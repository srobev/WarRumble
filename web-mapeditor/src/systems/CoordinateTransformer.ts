import { PointF } from '../types/MapDef';

export interface ScreenPoint {
  x: number;
  y: number;
}

export interface NormalizedPoint {
  x: number;
  y: number;
}

export interface ViewportInfo {
  width: number;
  height: number;
  cameraX: number;
  cameraY: number;
  zoom: number;
  bgWidth?: number;
  bgHeight?: number;
  topUIHeight: number;
}

/**
 * CoordinateTransformer handles conversion between different coordinate systems:
 * - Screen coordinates (pixels from top-left of canvas)
 * - World coordinates (normalized 0-1 map coordinates)
 * - View coordinates (screen coordinates relative to camera/viewport)
 */
export class CoordinateTransformer {
  private viewport: ViewportInfo;

  constructor(viewport: ViewportInfo) {
    this.viewport = viewport;
  }

  updateViewport(viewport: ViewportInfo) {
    this.viewport = viewport;
  }

  /**
   * Convert normalized coordinates (0-1) to screen coordinates
   */
  normalizedToScreen(normalized: NormalizedPoint): ScreenPoint {
    if (!this.viewport.bgWidth || !this.viewport.bgHeight) {
      // Fallback when no background - use viewport dimensions
      return {
        x: this.viewport.cameraX + normalized.x * this.viewport.width / this.viewport.zoom,
        y: this.viewport.topUIHeight + this.viewport.cameraY + normalized.y * this.viewport.height / this.viewport.zoom
      };
    }

    // Use background image dimensions for proper scaling
    const vh = this.viewport.height;
    const sx = this.viewport.width / this.viewport.bgWidth;
    const sy = vh / this.viewport.bgHeight;
    const scale = Math.min(sx, sy);

    // Calculate extended dimensions (same as Ebiten version)
    const extendedW = this.viewport.bgWidth * scale * 1.2;
    const extendedH = this.viewport.bgHeight * scale * 1.2;

    // Calculate border offsets
    const borderOffX = (this.viewport.width - extendedW) / 2;
    const borderOffY = this.viewport.topUIHeight + (vh - extendedH) / 2;

    // Calculate map border dimensions
    const mapBorderW = extendedW / 1.2;
    const mapBorderH = extendedH / 1.2;
    const mapOffX = borderOffX + (extendedW - mapBorderW) / 2;
    const mapOffY = borderOffY + (extendedH - mapBorderH) / 2;

    // Apply camera transformation
    const screenX = mapOffX + (normalized.x * mapBorderW) * this.viewport.zoom - this.viewport.cameraX * this.viewport.zoom;
    const screenY = mapOffY + (normalized.y * mapBorderH) * this.viewport.zoom - this.viewport.cameraY * this.viewport.zoom;

    return { x: screenX, y: screenY };
  }

  /**
   * Convert screen coordinates to normalized coordinates (0-1)
   */
  screenToNormalized(screen: ScreenPoint): NormalizedPoint {
    if (!this.viewport.bgWidth || !this.viewport.bgHeight) {
      // Fallback when no background
      return {
        x: (screen.x - this.viewport.cameraX) * this.viewport.zoom / this.viewport.width,
        y: (screen.y - this.viewport.topUIHeight - this.viewport.cameraY) * this.viewport.zoom / this.viewport.height
      };
    }

    // Use background image dimensions for proper scaling
    const vh = this.viewport.height;
    const sx = this.viewport.width / this.viewport.bgWidth;
    const sy = vh / this.viewport.bgHeight;
    const scale = Math.min(sx, sy);

    // Calculate extended dimensions
    const extendedW = this.viewport.bgWidth * scale * 1.2;
    const extendedH = this.viewport.bgHeight * scale * 1.2;

    // Calculate border offsets
    const borderOffX = (this.viewport.width - extendedW) / 2;
    const borderOffY = this.viewport.topUIHeight + (vh - extendedH) / 2;

    // Calculate map border dimensions
    const mapBorderW = extendedW / 1.2;
    const mapBorderH = extendedH / 1.2;
    const mapOffX = borderOffX + (extendedW - mapBorderW) / 2;
    const mapOffY = borderOffY + (extendedH - mapBorderH) / 2;

    // Apply reverse camera transformation
    const adjustedX = screen.x + this.viewport.cameraX * this.viewport.zoom - mapOffX;
    const adjustedY = screen.y + this.viewport.cameraY * this.viewport.zoom - mapOffY;

    // Convert to normalized coordinates
    const normalizedX = adjustedX / (mapBorderW * this.viewport.zoom);
    const normalizedY = adjustedY / (mapBorderH * this.viewport.zoom);

    return { x: normalizedX, y: normalizedY };
  }

  /**
   * Check if a screen point is within the canvas area
   */
  isPointInCanvas(screen: ScreenPoint): boolean {
    const vh = this.viewport.height;
    const topUIH = this.viewport.topUIHeight;

    if (!this.viewport.bgWidth || !this.viewport.bgHeight) {
      return screen.x >= 0 && screen.x < this.viewport.width &&
             screen.y >= topUIH && screen.y < topUIH + vh;
    }

    // Use same calculations as normalizedToScreen for consistency
    const sx = this.viewport.width / this.viewport.bgWidth;
    const sy = vh / this.viewport.bgHeight;
    const scale = Math.min(sx, sy);

    const extendedW = this.viewport.bgWidth * scale * 1.2;
    const extendedH = this.viewport.bgHeight * scale * 1.2;

    const borderOffX = (this.viewport.width - extendedW) / 2;
    const borderOffY = topUIH + (vh - extendedH) / 2;

    const mapBorderW = extendedW / 1.2;
    const mapBorderH = extendedH / 1.2;
    const mapOffX = borderOffX + (extendedW - mapBorderW) / 2;
    const mapOffY = borderOffY + (extendedH - mapBorderH) / 2;

    return screen.x >= mapOffX && screen.x < mapOffX + mapBorderW * this.viewport.zoom &&
           screen.y >= mapOffY && screen.y < mapOffY + mapBorderH * this.viewport.zoom;
  }

  /**
   * Get the visible canvas bounds in screen coordinates
   */
  getCanvasBounds(): { left: number; top: number; right: number; bottom: number } {
    if (!this.viewport.bgWidth || !this.viewport.bgHeight) {
      return {
        left: 0,
        top: this.viewport.topUIHeight,
        right: this.viewport.width,
        bottom: this.viewport.topUIHeight + this.viewport.height
      };
    }

    // Use same calculations as other methods
    const vh = this.viewport.height;
    const sx = this.viewport.width / this.viewport.bgWidth;
    const sy = vh / this.viewport.bgHeight;
    const scale = Math.min(sx, sy);

    const extendedW = this.viewport.bgWidth * scale * 1.2;
    const extendedH = this.viewport.bgHeight * scale * 1.2;

    const borderOffX = (this.viewport.width - extendedW) / 2;
    const borderOffY = this.viewport.topUIHeight + (vh - extendedH) / 2;

    const mapBorderW = extendedW / 1.2;
    const mapBorderH = extendedH / 1.2;
    const mapOffX = borderOffX + (extendedW - mapBorderW) / 2;
    const mapOffY = borderOffY + (extendedH - mapBorderH) / 2;

    return {
      left: mapOffX,
      top: mapOffY,
      right: mapOffX + mapBorderW * this.viewport.zoom,
      bottom: mapOffY + mapBorderH * this.viewport.zoom
    };
  }

  /**
   * Apply camera zoom to a scale factor
   */
  applyZoom(scale: number): number {
    return scale * this.viewport.zoom;
  }

  /**
   * Utility method to convert PointF to NormalizedPoint
   */
  pointFToNormalized(point: PointF): NormalizedPoint {
    return { x: point.x, y: point.y };
  }

  /**
   * Utility method to convert ScreenPoint to PointF
   */
  normalizedToPointF(point: NormalizedPoint): PointF {
    return { x: point.x, y: point.y };
  }
}
