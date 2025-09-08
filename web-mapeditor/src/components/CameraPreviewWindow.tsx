import React, { useRef, useEffect } from 'react';
import { CoordinateTransformer, ScreenPoint } from '../systems/CoordinateTransformer';
import { EditorState } from '../types/EditorState';
import { RenderEngine } from '../systems/RenderEngine';

interface CameraPreviewWindowProps {
  editorState: EditorState;
  transformer: CoordinateTransformer;
  renderEngine: RenderEngine;
  onToggle: () => void;
  onDrag: (deltaX: number, deltaY: number) => void;
  onResize: (newWidth: number, newHeight: number) => void;
}

/**
 * CameraPreviewWindow component displays a live preview of what the in-game camera would see
 * Positioned as an overlay on the main canvas
 */
export const CameraPreviewWindow: React.FC<CameraPreviewWindowProps> = ({
  editorState,
  transformer,
  onToggle,
  onDrag,
  onResize
}) => {
  // Get canvas bounds for positioning
  const getCanvasBounds = () => {
    if (!transformer) return { width: 800, height: 600 };
    const bounds = transformer.getCanvasBounds();
    return {
      width: bounds.right - bounds.left,
      height: bounds.bottom - bounds.top
    };
  };

  // Calculate preview window position and size
  const canvasBounds = getCanvasBounds();
  const previewWidth = editorState.frameWidth || 200;
  const previewHeight = editorState.frameHeight || 150;
  const previewX = canvasBounds.width - previewWidth - 20; // 20px from right edge
  const previewY = (canvasBounds.height - previewHeight) / 2; // Centered vertically

  return (
    <div
      className="camera-preview-window"
      style={{
        position: 'absolute',
        left: previewX + 'px',
        top: previewY + 'px',
        width: previewWidth + 'px',
        height: previewHeight + 'px',
        border: '2px solid #333',
        borderRadius: '4px',
        backgroundColor: 'rgba(0, 0, 0, 0.8)',
        backdropFilter: 'blur(2px)',
        display: editorState.showCameraPreview ? 'block' : 'none',
        zIndex: 100,
        overflow: 'hidden'
      }}
    >
      {/* Header bar */}
      <div
        className="preview-header"
        style={{
          height: '24px',
          backgroundColor: 'rgba(51, 51, 51, 0.9)',
          borderBottom: '1px solid #666',
          display: 'flex',
          alignItems: 'center',
          padding: '0 8px',
          userSelect: 'none',
          cursor: 'move'
        }}
      >
        <span style={{ color: '#fff', fontSize: '12px', fontWeight: 'bold' }}>
          Camera Preview
        </span>
        <button
          onClick={onToggle}
          style={{
            marginLeft: 'auto',
            background: 'none',
            border: 'none',
            color: '#ccc',
            cursor: 'pointer',
            fontSize: '16px',
            padding: '2px 6px',
            borderRadius: '2px'
          }}
          onMouseOver={(e) => (e.currentTarget.style.backgroundColor = '#555')}
          onMouseOut={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
        >
          ×
        </button>
      </div>

      {/* Preview viewport */}
      <div
        className="preview-viewport"
        style={{
          width: '100%',
          height: 'calc(100% - 24px)',
          backgroundColor: '#1a1a2e',
          border: '1px solid #666',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: '#fff',
          fontSize: '14px',
          textAlign: 'center'
        }}
      >
        <div>
          <div>📷 Camera View</div>
          <div style={{ fontSize: '10px', color: '#ccc', marginTop: '4px' }}>
            Frame: {editorState.frameWidth}x{editorState.frameHeight}
          </div>
          <div style={{ fontSize: '10px', color: '#ccc' }}>
            Zoom: {editorState.cameraZoom.toFixed(2)}x
          </div>
        </div>
      </div>
    </div>
  );
};
