// File: frontend/src/App.tsx
import { useEffect, useState, useCallback } from 'react';
import styled, { createGlobalStyle } from 'styled-components';
import useWebSocket, { ReadyState } from 'react-use-websocket'; // Import from the library
import GameCanvas from './components/GameCanvas';
import { WEBSOCKET_URL } from './config';
import { DirectionMessage, GameState } from './types/game';

const GlobalStyle = createGlobalStyle`
  * {
    box-sizing: border-box; /* More predictable sizing */
  }

  html, body, #root {
    height: 100%; /* Allow children to use percentage heights */
    margin: 0;
    padding: 0;
    overflow: hidden; /* Prevent scrollbars if canvas slightly overflows */
  }

  body {
    background-color: #1a1a1a; /* Slightly lighter dark background */
    color: white;
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen',
      'Ubuntu', 'Cantarell', 'Fira Sans', 'Droid Sans', 'Helvetica Neue',
      sans-serif;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
    display: flex; /* Use flexbox for centering */
    justify-content: center;
    align-items: center;
  }
`;

const AppContainer = styled.div`
  text-align: center;
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  justify-content: center; /* Center canvas vertically */
  align-items: center; /* Center canvas horizontally */
  padding: 10px; /* Small padding around the edges */
`;

const Header = styled.header`
    display: flex;
    align-items: center;
    justify-content: center;
    margin-bottom: 10px; /* Space below header */
    color: #E0E0E0; /* Light grey text */
`;

const Logo = styled.img`
    height: 40px; /* Adjust size as needed */
    margin-right: 15px;
`;

const Instructions = styled.p`
  margin-top: 15px;
  color: #aaa;
  font-size: 0.9em;
`;


function App() {
  const [gameState, setGameState] = useState<GameState | null>(null);

  // Use the react-use-websocket hook
  const { sendMessage, lastMessage, readyState } = useWebSocket(WEBSOCKET_URL, {
    onOpen: () => console.log('WS: Connection Opened'),
    onClose: () => {
      console.log('WS: Connection Closed');
      setGameState(null); // Clear state on close
    },
    onError: (event) => console.error('WS: Error:', event),
    shouldReconnect: (closeEvent) => true, // Automatically reconnect
    reconnectInterval: 3000, // Reconnect attempt interval
    // Filter out non-JSON messages or messages not conforming to GameState
    filter: (message) => {
      try {
        const data = JSON.parse(message.data);
        return (data && typeof data === 'object' && data.canvas && data.players && data.paddles && data.balls);
      } catch (e) {
        return false; // Ignore non-JSON messages
      }
    },
  });

  // Process incoming messages
  useEffect(() => {
    if (lastMessage !== null) {
      try {
        const data = JSON.parse(lastMessage.data);
        setGameState(data as GameState);
      } catch (e) {
        console.error("WS: Failed to parse last message:", e);
      }
    }
  }, [lastMessage]);

  // Map ReadyState to our status string type
  const mapReadyStateToStatus = (state: ReadyState): 'connecting' | 'open' | 'closing' | 'closed' | 'error' => {
    switch (state) {
      case ReadyState.CONNECTING: return 'connecting';
      case ReadyState.OPEN: return 'open';
      case ReadyState.CLOSING: return 'closing';
      case ReadyState.CLOSED: return 'closed';
      case ReadyState.UNINSTANTIATED: return 'connecting'; // Treat as connecting initially
      default: return 'closed'; // Default to closed
    }
  };

  const connectionStatus = mapReadyStateToStatus(readyState);

  // Memoize the sendMessage function provided by the hook
  const sendDirection = useCallback((direction: DirectionMessage['direction']) => {
    if (connectionStatus === 'open') {
      const message: DirectionMessage = { direction };
      sendMessage(JSON.stringify(message));
    }
  }, [sendMessage, connectionStatus]);

  // Handle keyboard input
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (connectionStatus !== 'open') return;

      if (event.key === 'ArrowLeft') {
        sendDirection('ArrowLeft');
      } else if (event.key === 'ArrowRight') {
        sendDirection('ArrowRight');
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [sendDirection, connectionStatus]); // Depend on the memoized send function and status

  return (
    <>
      <GlobalStyle />
      <AppContainer>
        <Header>
          <Logo src="/bitmap.png" alt="PonGo Logo" />
          <h1>PonGo</h1>
        </Header>
        {/* Pass state and mapped status down */}
        <GameCanvas gameState={gameState} wsStatus={connectionStatus} />
        <Instructions>Use Left/Right Arrow Keys to move the paddle.</Instructions>
      </AppContainer>
    </>
  );
}

export default App;