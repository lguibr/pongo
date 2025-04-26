
import { useEffect, useState } from 'react';

export interface Tile {
  id: string;
  x: number;
  y: number;
  size: number;
  color: string;
}

/**
 * Custom hook to subscribe to game state via WebSocket.
 * Accepts either a full WS URL (ws:// or wss://) or a path (/subscribe).
 * Always returns an array (never null).
 */
export function useWebSocket(pathOrUrl: string): Tile[] {
  const [tiles, setTiles] = useState<Tile[]>([]);

  useEffect(() => {
    // Determine WebSocket URL:
    // - Full URL if starts with ws:// or wss://
    // - Otherwise use current host & protocol
    const url =
      pathOrUrl.startsWith('ws://') || pathOrUrl.startsWith('wss://')
        ? pathOrUrl
        : (() => {
          const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws';
          return `${protocol}://${window.location.host}${pathOrUrl}`;
        })();

    const ws = new WebSocket(url);

    ws.onopen = () => {
      console.log('WebSocket connection opened:', url);
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        if (Array.isArray(data.tiles)) {
          setTiles(data.tiles);
        }
      } catch (err) {
        console.error('Failed to parse WebSocket message', err);
      }
    };

    ws.onerror = (err) => {
      console.error('WebSocket error', err);
    };

    ws.onclose = (event) => {
      console.log('WebSocket connection closed', event.code);
    };

    return () => {
      ws.close();
    };
  }, [pathOrUrl]);

  return tiles;
}

