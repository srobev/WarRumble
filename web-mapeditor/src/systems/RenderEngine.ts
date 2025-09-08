import { CoordinateTransformer, ScreenPoint, NormalizedPoint } from './CoordinateTransformer';
import { MapDef, PointF, DeployZone, Obstacle, DecorativeElement, Lane } from '../types/MapDef';

export interface RenderContext {
  ctx: CanvasRenderingContext2D;
  width: number;
  height: number;
  transformer: CoordinateTransformer;
}

/**
 * RenderEngine handles all visual rendering using HTML5 Canvas
 * Replicates the functionality of Ebiten's rendering system
 */
export class RenderEngine {
  private canvas: HTMLCanvasElement;
  private ctx: CanvasRenderingContext2D;
  private transformer: CoordinateTransformer;
  private currentBg?: HTMLImageElement;

  constructor(canvas: HTMLCanvasElement) {
    this.canvas = canvas;
    const ctx = canvas.getContext('2d');
    if (!ctx) {
      throw new Error('Could not get 2D context from canvas');
    }
    this.ctx = ctx;

    // Create a default transformer (will be updated with proper viewport info)
    this.transformer = new CoordinateTransformer({
      width: canvas.width,
      height: canvas.height,
      cameraX: 0,
      cameraY: 0,
      zoom: 1,
      topUIHeight: 120
    });

    // Set up canvas context
    this.ctx.imageSmoothingEnabled = true;
    this.ctx.imageSmoothingQuality = 'high';
    this.ctx.textAlign = 'left';
    this.ctx.textBaseline = 'top';
  }

  /**
   * Update the coordinate transformation system
   */
  updateTransformer(transformer: CoordinateTransformer) {
    this.transformer = transformer;
  }

  /**
   * Set the background image
   */
  setBackground(image: HTMLImageElement) {
    this.currentBg = image;
  }

  /**
   * Clear the background image
   */
  clearBackground() {
    this.currentBg = undefined;
  }

  /**
   * Get the current render context
   */
  getRenderContext(): RenderContext {
    return {
      ctx: this.ctx,
      width: this.canvas.width,
      height: this.canvas.height,
      transformer: this.transformer
    };
  }

  /**
   * Clear the entire canvas
   */
  clear(color: string = '#1a1a2e') {
    this.ctx.fillStyle = color;
    this.ctx.fillRect(0, 0, this.canvas.width, this.canvas.height);
  }

  /**
   * Draw the background image with proper scaling and positioning
   */
  drawBackground(x: number = 0, y: number = 120) {
    if (!this.currentBg) return;

    const imgW = this.currentBg.width;
    const imgH = this.currentBg.height;
    const canvasW = this.canvas.width;
    const canvasH = this.canvas.height;
    const uiHeight = 120;

    // Calculate scaling to fit the image in the available space
    const vh = canvasH - uiHeight;
    const sx = canvasW / imgW;
    const sy = vh / imgH;
    const scale = Math.min(sx, sy);

    // Calculate extended dimensions (1.2x for camera space like Ebiten)
    const extendedW = imgW * scale * 1.2;
    const extendedH = imgH * scale * 1.2;

    // Center the image
    const offsetX = (canvasW - extendedW) / 2;
    const offsetY = uiHeight + (vh - extendedH) / 2;

    // Create pattern for tiling if needed, but for now just draw centered
    this.ctx.save();
    this.ctx.translate(offsetX, offsetY);
    this.ctx.scale(scale, scale);
    this.ctx.drawImage(this.currentBg, 0, 0);
    this.ctx.restore();
  }

  /**
   * Draw a rectangle with optional fill and stroke
   */
  drawRect(x: number, y: number, width: number, height: number, options: {
    fillColor?: string;
    strokeColor?: string;
    lineWidth?: number;
  } = {}) {
    const { fillColor, strokeColor, lineWidth = 1 } = options;

    if (fillColor) {
      this.ctx.fillStyle = fillColor;
      this.ctx.fillRect(x, y, width, height);
    }

    if (strokeColor) {
      this.ctx.strokeStyle = strokeColor;
      this.ctx.lineWidth = lineWidth;
      this.ctx.strokeRect(x, y, width, height);
    }
  }

  /**
   * Draw a line between two points
   */
  drawLine(x1: number, y1: number, x2: number, y2: number, color: string = '#ffffff', lineWidth: number = 1) {
    this.ctx.strokeStyle = color;
    this.ctx.lineWidth = lineWidth;
    this.ctx.beginPath();
    this.ctx.moveTo(x1, y1);
    this.ctx.lineTo(x2, y2);
    this.ctx.stroke();
  }

  /**
   * Draw a circle/ellipse
   */
  drawEllipse(x: number, y: number, radiusX: number, radiusY: number, options: {
    fillColor?: string;
    strokeColor?: string;
    lineWidth?: number;
  } = {}) {
    const { fillColor, strokeColor, lineWidth = 1 } = options;

    this.ctx.beginPath();
    this.ctx.ellipse(x, y, radiusX, radiusY, 0, 0, 2 * Math.PI);

    if (fillColor) {
      this.ctx.fillStyle = fillColor;
      this.ctx.fill();
    }

    if (strokeColor) {
      this.ctx.strokeStyle = strokeColor;
      this.ctx.lineWidth = lineWidth;
      this.ctx.stroke();
    }
  }

  /**
   * Draw text at a position
   */
  drawText(text: string, x: number, y: number, options: {
    color?: string;
    font?: string;
    align?: CanvasTextAlign;
    baseline?: CanvasTextBaseline;
  } = {}) {
    const {
      color = '#ffffff',
      font = '12px monospace',
      align = 'left',
      baseline = 'top'
    } = options;

    this.ctx.fillStyle = color;
    this.ctx.font = font;
    this.ctx.textAlign = align;
    this.ctx.textBaseline = baseline;
    this.ctx.fillText(text, x, y);
  }

  /**
   * Draw an image with transformations
   */
  drawImage(image: HTMLImageElement, x: number, y: number, options: {
    width?: number;
    height?: number;
    scale?: number;
    rotation?: number;
  } = {}) {
    const { width, height, scale = 1, rotation = 0 } = options;

    this.ctx.save();

    // Apply transformations
    this.ctx.translate(x, y);
    if (rotation) this.ctx.rotate(rotation);

    const drawWidth = (width || image.width) * scale;
    const drawHeight = (height || image.height) * scale;

    this.ctx.drawImage(image, -drawWidth / 2, -drawHeight / 2, drawWidth, drawHeight);

    this.ctx.restore();
  }

  /**
   * Draw deploy zones
   */
  drawDeployZone(deployZone: DeployZone, screenPos: ScreenPoint, isSelected: boolean = false) {
    const { ctx } = this;
    const width = this.transformer.applyZoom(deployZone.w);
    const height = this.transformer.applyZoom(deployZone.h);

    // Set color based on owner
    let fillColor: string;
    switch (deployZone.owner) {
      case 'player': fillColor = 'rgba(0, 150, 255, 0.4)'; break; // Blue for player
      case 'enemy': fillColor = 'rgba(255, 0, 0, 0.4)'; break; // Red for enemy
      default: fillColor = 'rgba(128, 128, 128, 0.4)'; break; // Gray for neutral
    }

    // Draw fill
    ctx.fillStyle = fillColor;
    ctx.fillRect(screenPos.x, screenPos.y, width, height);

    // Draw border
    if (isSelected) {
      ctx.strokeStyle = '#F0C111'; // Yellow for selection
      ctx.lineWidth = 2;
    } else {
      ctx.strokeStyle = '#666666';
      ctx.lineWidth = 1;
    }
    ctx.strokeRect(screenPos.x, screenPos.y, width, height);

    // Draw resize handles if selected
    if (isSelected) {
      const handleSize = 6;
      ctx.fillStyle = '#F0C111';
      const handles = [
        [screenPos.x, screenPos.y], // Top-left
        [screenPos.x + width, screenPos.y], // Top-right
        [screenPos.x + width, screenPos.y + height], // Bottom-right
        [screenPos.x, screenPos.y + height] // Bottom-left
      ];

      handles.forEach(([hx, hy]) => {
        ctx.fillRect(hx - handleSize/2, hy - handleSize/2, handleSize, handleSize);
      });
    }
  }

  /**
   * Draw a meeting stone or gold mine
   */
  drawPointElement(center: ScreenPoint, type: 'stone' | 'mine', isSelected: boolean = false) {
    const { ctx } = this;
    const size = type === 'mine' ? 6 : 4;
    const color = type === 'mine' ? '#FFD700' : '#8A2BE2'; // Gold for mines, purple for stones
    
    ctx.fillStyle = isSelected ? '#F0C111' : color;
    ctx.fillRect(center.x - size/2, center.y - size/2, size, size);
  }

  /**
   * Draw a lane path
   */
  drawLane(lane: Lane, points: ScreenPoint[], isSelected: boolean = false) {
    const { ctx } = this;
    
    if (points.length < 2) return;

    // Set color based on direction
    const color = lane.dir < 0 ? '#FF6B6B' : '#4ECDC4'; // Red for reverse, cyan for normal
    ctx.strokeStyle = isSelected ? '#F0C111' : color;
    ctx.lineWidth = isSelected ? 3 : 2;
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';

    ctx.beginPath();
    ctx.moveTo(points[0].x, points[0].y);
    for (let i = 1; i < points.length; i++) {
      ctx.lineTo(points[i].x, points[i].y);
    }
    ctx.stroke();

    // Draw direction arrow at the end
    if (points.length >= 2) {
      const last = points[points.length - 1];
      const prev = points[points.length - 2];
      const dx = last.x - prev.x;
      const dy = last.y - prev.y;
      const angle = Math.atan2(dy, dx);

      const arrowLength = 10;
      const arrowAngle = Math.PI / 6; // 30 degrees

      ctx.beginPath();
      ctx.moveTo(last.x, last.y);
      ctx.lineTo(
        last.x - arrowLength * Math.cos(angle - arrowAngle),
        last.y - arrowLength * Math.sin(angle - arrowAngle)
      );
      ctx.moveTo(last.x, last.y);
      ctx.lineTo(
        last.x - arrowLength * Math.cos(angle + arrowAngle),
        last.y - arrowLength * Math.sin(angle + arrowAngle)
      );
      ctx.stroke();
    }
  }

  /**
   * Draw an obstacle
   */
  drawObstacle(obstacle: Obstacle, screenPos: ScreenPoint, image?: HTMLImageElement, isSelected: boolean = false) {
    const { ctx } = this;
    const width = this.transformer.applyZoom(obstacle.width);
    const height = this.transformer.applyZoom(obstacle.height);

    if (image) {
      // Draw the actual image
      ctx.drawImage(image, screenPos.x, screenPos.y, width, height);
    } else {
      // Fallback to rectangle
      ctx.fillStyle = 'rgba(120, 80, 40, 0.6)'; // Brown
      ctx.fillRect(screenPos.x, screenPos.y, width, height);
    }

    if (isSelected) {
      // Draw selection border
      ctx.strokeStyle = '#F0C111';
      ctx.lineWidth = 2;
      ctx.strokeRect(screenPos.x, screenPos.y, width, height);

      // Draw resize handles
      const handleSize = 6;
      ctx.fillStyle = '#F0C111';
      const handles = [
        [screenPos.x, screenPos.y], // Top-left
        [screenPos.x + width, screenPos.y], // Top-right
        [screenPos.x + width, screenPos.y + height], // Bottom-right
        [screenPos.x, screenPos.y + height] // Bottom-left
      ];

      handles.forEach(([hx, hy]) => {
        ctx.fillRect(hx - handleSize/2, hy - handleSize/2, handleSize, handleSize);
      });
    }
  }

  /**
   * Draw a decorative element
   */
  drawDecorative(decorative: DecorativeElement, screenPos: ScreenPoint, image?: HTMLImageElement, isSelected: boolean = false) {
    const { ctx } = this;
    const width = this.transformer.applyZoom(decorative.width);
    const height = this.transformer.applyZoom(decorative.height);

    if (image) {
      ctx.drawImage(image, screenPos.x, screenPos.y, width, height);
    } else {
      ctx.fillStyle = 'rgba(200, 150, 200, 0.6)'; // Purple
      ctx.fillRect(screenPos.x, screenPos.y, width, height);
    }

    if (isSelected) {
      ctx.strokeStyle = '#F0C111';
      ctx.lineWidth = 2;
      ctx.strokeRect(screenPos.x, screenPos.y, width, height);

      // Resize handles
      const handleSize = 6;
      ctx.fillStyle = '#F0C111';
      const corners = [
        { x: screenPos.x, y: screenPos.y },
        { x: screenPos.x + width, y: screenPos.y },
        { x: screenPos.x + width, y: screenPos.y + height },
        { x: screenPos.x, y: screenPos.y + height }
      ];

      corners.forEach(({ x, y }) => {
        ctx.fillRect(x - handleSize/2, y - handleSize/2, handleSize, handleSize);
      });
    }
  }

  /**
   * Draw a base
   */
  drawBase(center: ScreenPoint, type: 'player' | 'enemy', isSelected: boolean = false) {
    const { ctx } = this;
    const baseSize = 96;

    // Draw base background
    const bgColor = type === 'player' ? 'rgba(0, 150, 255, 0.8)' : 'rgba(255, 0, 0, 0.8)';
    ctx.fillStyle = bgColor;
    ctx.fillRect(center.x - baseSize/2, center.y - baseSize/2, baseSize, baseSize);

    if (isSelected) {
      ctx.strokeStyle = '#F0C111';
      ctx.lineWidth = 3;
      ctx.strokeRect(center.x - baseSize/2, center.y - baseSize/2, baseSize, baseSize);
    }

    // Draw label
    const label = type === 'player' ? 'PLAYER BASE' : 'ENEMY BASE';
    const textColor = type === 'player' ? '#0096FF' : '#FF0000';
    
    ctx.fillStyle = 'rgba(0, 0, 0, 0.8)';
    ctx.fillRect(center.x - 45, center.y - baseSize/2 - 25, 90, 20);
    
    ctx.fillStyle = textColor;
    ctx.font = '12px monospace';
    ctx.textAlign = 'center';
    ctx.fillText(label, center.x, center.y - baseSize/2 - 10);
  }

  /**
   * Draw a frame border showing the map boundaries
   */
  drawFrame(center: ScreenPoint, width: number, height: number) {
    const { ctx } = this;
    const halfWidth = width / 2;
    const halfHeight = height / 2;

    ctx.strokeStyle = '#FFFFFF';
    ctx.lineWidth = 6;
    ctx.strokeRect(center.x - halfWidth, center.y - halfHeight, width, height);
  }

  /**
   * Draw a grid overlay
   */
  drawGrid(startX: number, startY: number, endX: number, endY: number, spacing: number = 20) {
    const { ctx } = this;
    ctx.strokeStyle = 'rgba(96, 96, 112, 0.3)';
    ctx.lineWidth = 1;

    ctx.beginPath();

    // Vertical lines
    for (let x = startX; x <= endX; x += spacing) {
      ctx.moveTo(x, startY);
      ctx.lineTo(x, endY);
    }

    // Horizontal lines
    for (let y = startY; y <= endY; y += spacing) {
      ctx.moveTo(startX, y);
      ctx.lineTo(endX, y);
    }

    ctx.stroke();
  }

  /**
   * Draw camera preview window content
   */
  drawCameraPreview(mapDef: MapDef, center: { x: number, y: number }, width: number, height: number) {
    const { ctx } = this;

    // Save current context state
    ctx.save();

    // Set up clipping for the preview window
    ctx.beginPath();
    ctx.rect(center.x - width/2, center.y - height/2, width, height);
    ctx.clip();

    // Scale the entire map to fit in the preview window
    const scaleX = width / (this.transformer.getCanvasBounds().right - this.transformer.getCanvasBounds().left);
    const scaleY = height / (this.transformer.getCanvasBounds().bottom - this.transformer.getCanvasBounds().top);
    const scale = Math.min(scaleX, scaleY);

    // Apply transform to center and scale the map
    ctx.translate(center.x - (width / scale) / 2, center.y - (height / scale) / 2);
    ctx.scale(scale, scale);

    // Render background if available
    if (this.currentBg) {
      const imgW = this.currentBg.width;
      const imgH = this.currentBg.height;
      const vh = this.canvas.height - 120; // Account for UI height

      // Calculate scaling to fit the image
      const sx = this.canvas.width / imgW;
      const sy = vh / imgH;
      const bgScale = Math.min(sx, sy);

      // Calculate extended dimensions
      const extendedW = imgW * bgScale * 1.2;
      const extendedH = imgH * bgScale * 1.2;

      // Calculate positioning
      const offsetX = (this.canvas.width - extendedW) / 2;
      const offsetY = 120 + (vh - extendedH) / 2;

      ctx.drawImage(this.currentBg, offsetX, offsetY, extendedW, extendedH);
    } else {
      // Draw checker pattern as fallback
      ctx.fillStyle = '#2a2a2a';
      ctx.fillRect(0, 120, this.canvas.width, this.canvas.height - 120);
    }

    // Draw a border to indicate preview area
    ctx.strokeStyle = 'rgba(255, 255, 255, 0.5)';
    ctx.lineWidth = 2;
    ctx.strokeRect(0, 120, this.canvas.width, this.canvas.height - 120);

    // Draw center crosshair
    const centerX = this.canvas.width / 2;
    const centerY = 120 + (this.canvas.height - 120) / 2;
    ctx.strokeStyle = 'rgba(0, 255, 0, 0.7)';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(centerX - 20, centerY);
    ctx.lineTo(centerX + 20, centerY);
    ctx.moveTo(centerX, centerY - 20);
    ctx.lineTo(centerX, centerY + 20);
    ctx.stroke();

    // Restore context
    ctx.restore();

    // Draw preview label
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.font = '12px monospace';
    ctx.textAlign = 'center';
    ctx.fillText('Preview Area', center.x, center.y + height/2 + 20);
    ctx.textAlign = 'left'; // Reset
  }

  /**
   * Save the current canvas state
   */
  save() {
    this.ctx.save();
  }

  /**
   * Restore the previous canvas state
   */
  restore() {
    this.ctx.restore();
  }

  /**
   * Set the global transform (useful for camera effects)
   */
  setTransform(a: number, b: number, c: number, d: number, e: number, f: number) {
    this.ctx.setTransform(a, b, c, d, e, f);
  }

  /**
   * Reset the transform to identity
   */
  resetTransform() {
    this.ctx.resetTransform();
  }

  /**
   * Resize the canvas
   */
  resize(width: number, height: number) {
    this.canvas.width = width;
    this.canvas.height = height;
  }
}
