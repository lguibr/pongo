// File: frontend/src/types/game.ts

// These interfaces should mirror the JSON structure sent by the Go backend

export interface BrickData {
  type: number; // Corresponds to utils.CellType (0: brick, 1: block, 2: empty)
  life: number;
  level: number;
}

export interface Cell {
  x: number;
  y: number;
  data: BrickData;
}

export type Grid = Cell[][];

export interface Canvas {
  grid: Grid;
  width: number;
  height: number;
  gridSize: number;
  canvasSize: number;
  cellSize: number;
}

export interface Player {
  index: number;
  id: string;
  canvas: Canvas; // Note: Backend sends the whole canvas per player, might optimize later
  color: [number, number, number];
  score: number;
}

export interface Paddle {
  x: number;
  y: number;
  width: number;
  height: number;
  index: number;
  direction: string; // "left", "right", or ""
  velocity: number;
}

export interface Ball {
  x: number;
  y: number;
  vx: number;
  vy: number;
  ax: number;
  ay: number;
  radius: number;
  id: number; // Usually a nanosecond timestamp
  ownerIndex: number;
  phasing: boolean;
  mass: number;
}

export interface GameState {
  canvas: Canvas;
  players: (Player | null)[]; // Array can have null entries
  paddles: (Paddle | null)[]; // Array can have null entries
  balls: (Ball | null)[]; // Array can have null entries
}

// Message sent from frontend to backend
export interface DirectionMessage {
  direction: "ArrowLeft" | "ArrowRight";
}

