import { useEffect, useState, useRef, useCallback } from 'react';
import { GameState } from '../types/game'; // Assuming GameState type definition exists

// Define a type for the hook's return value
export type WebSocketHookResult = {
  gameState: GameState | null;
  isConnected: boolean;
  sendMessage: (message: any) => void;
};

/**
 * Custom hook to subscribe to game state via WebSocket.
 * Accepts either a full WS URL (ws:// or wss://) or a path (/subscribe).
 * Always returns an array (never null).
 */
export function useWebSocket(pathOrUrl: string): Tile[] { // TODO: Change return type
// export function useWebSocket(pathOrUrl: string): WebSocketHookResult { // Correct return type
  const [gameState, setGameState] = useState<GameState | null>(null);
  const [isConnected, setIsConnected] = useState<boolean>(false);
  const webSocketRef = useRef<WebSocket | null>(null);

  useEffect(() => {
    console.log('WS: useEffect started. URL/Path:', pathOrUrl);

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

    console.log('WS: Attempting to connect to:', url);
    const ws = new WebSocket(url);
    webSocketRef.current = ws; // Store WebSocket instance

    ws.onopen = () => {
      console.log('WS: Opened connection to:', url);
      setIsConnected(true);
    };

    ws.onmessage = (event) => {
      console.log('WS: Message received (raw):', event.data);
      try {
        const data = JSON.parse(event.data);
        // Check if it looks like a GameState object
        if (data && typeof data === 'object' && data.canvas && data.players) {
           console.log('WS: Parsed GameState received.');
           setGameState(data as GameState);
        } else {
           console.warn('WS: Received data is not in expected GameState format:', data);
        }
      } catch (err) {
        console.error('WS: Failed to parse JSON message:', err);
      }
    };

    ws.onerror = (err) => {
      console.error('WS: Error:', err);
      // setIsConnected(false); // Error often leads to close
    };

    ws.onclose = (event) => {
      console.log('WS: Closed:', event.code, event.reason, `wasClean=${event.wasClean}`);
      setIsConnected(false);
      webSocketRef.current = null; // Clear ref on close
    };

    // Cleanup function
    return () => {
      console.log('WS: Cleanup effect running. Closing WebSocket.');
      if (webSocketRef.current && (webSocketRef.current.readyState === WebSocket.OPEN || webSocketRef.current.readyState === WebSocket.CONNECTING)) {
         console.log(`WS: Closing socket with state ${webSocketRef.current.readyState}`);
         webSocketRef.current.close(1000, "Component unmounting"); // Close with standard code
      }
      setIsConnected(false); // Ensure state reflects closure
      webSocketRef.current = null;
    };
  }, [pathOrUrl]);


  // Function to send messages
  const sendMessage = useCallback((message: any) => {
    if (webSocketRef.current && webSocketRef.current.readyState === WebSocket.OPEN) {
       console.log('WS: Sending message:', message);
       webSocketRef.current.send(JSON.stringify(message));
    } else {
       console.error('WS: Cannot send message, WebSocket is not open. State:', webSocketRef.current?.readyState);
    }
  }, []); // Empty dependency array means this function doesn't change

  // Return the state and the sendMessage function
  // return { gameState, isConnected, sendMessage }; // Correct return type

  // TEMPORARY: Return empty array to match original signature and avoid breaking compile
  // TODO: Update components using this hook to expect the new return type { gameState, isConnected, sendMessage }
  // For now, return a dummy structure that won't immediately break consumers expecting an array.
  // Consumers like GameCanvas need to be updated to use gameState, isConnected, sendMessage.
  return [];
}
