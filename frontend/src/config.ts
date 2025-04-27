// File: frontend/src/config.ts
// Ensure this points directly to your backend's WebSocket endpoint
export const WEBSOCKET_URL = "ws://localhost:3001/subscribe";
export const CANVAS_SIZE = 576; // Must match backend utils.CanvasSize
export const CELL_SIZE = CANVAS_SIZE / 12; // Must match backend utils.CellSize calculation