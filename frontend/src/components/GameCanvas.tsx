
import React from 'react';
import { useWebSocket, Tile } from '../hooks/useWebSocket';

/**
 * GameCanvas renders the current game state as an SVG.
 * Relaies on vite.config.ts proxy for /subscribe.
 */
export const GameCanvas: React.FC = () => {
  // Use relative path; Vite dev server will proxy this WS call
  const tiles = useWebSocket('/subscribe');

  return (
    <svg width={800} height={600}>
      {tiles.map((tile: Tile) => (
        <rect
          key={tile.id}
          x={tile.x}
          y={tile.y}
          width={tile.size}
          height={tile.size}
          fill={tile.color}
        />
      ))}
    </svg>
  );
};


export default GameCanvas;