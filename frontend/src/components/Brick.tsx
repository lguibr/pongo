// File: frontend/src/components/Brick.tsx
import styled from 'styled-components';
import { Cell, BrickData } from '../types/game';
import { CELL_SIZE } from '../config';

// Google Colors
const BRICK_COLORS = [
  '#DB4437', // Red (Level 1)
  '#F4B400', // Yellow (Level 2)
  '#0F9D58', // Darker Green (Level 3+) - Add more if needed
];
const BORDER_COLOR = '#444'; // Dark grey border

interface BrickProps {
  $cellData: Cell;
}

const getBrickColor = (data: BrickData): string => {
  if (data.life <= 0) return 'transparent'; // Should not happen if filtered, but safe
  const levelIndex = Math.max(0, data.level - 1); // Level 1 -> index 0
  return BRICK_COLORS[Math.min(levelIndex, BRICK_COLORS.length - 1)]; // Use last color for higher levels
};

const BrickComponent = styled.div<BrickProps>`
  position: absolute;
  background-color: ${(props) => getBrickColor(props.$cellData.data)};
  width: ${CELL_SIZE}px;
  height: ${CELL_SIZE}px;
  left: ${(props) => props.$cellData.x * CELL_SIZE}px;
  top: ${(props) => props.$cellData.y * CELL_SIZE}px;
  border: 1px solid ${BORDER_COLOR};
  box-sizing: border-box; /* Include border in size */
  opacity: ${(props) => Math.max(0.3, props.$cellData.data.life / Math.max(1, props.$cellData.data.level))}; /* Fade as life decreases */
`;

export default BrickComponent;
