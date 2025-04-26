// File: frontend/src/App.tsx
import { useEffect } from 'react';
import styled, { createGlobalStyle } from 'styled-components';
import GameCanvas from './components/GameCanvas';
import { useWebSocket } from './hooks/useWebSocket';
import { WEBSOCKET_URL } from './config';
import { DirectionMessage } from './types/game';

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


function App() {
  const { gameState, status, sendMessage } = useWebSocket(WEBSOCKET_URL);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (status !== 'open') return;

      let direction: DirectionMessage['direction'] | null = null;

      if (event.key === 'ArrowLeft') {
        direction = 'ArrowLeft';
      } else if (event.key === 'ArrowRight') {
        direction = 'ArrowRight';
      }

      if (direction) {
        sendMessage({ direction });
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [sendMessage, status]);

  return (
    <>
      <GlobalStyle />
      <AppContainer>
        <Header>
          <Logo src="/bitmap.png" alt="PonGo Logo" />
          <h1>PonGo</h1>
        </Header>
        <GameCanvas gameState={gameState} wsStatus={status} />
        {/* Instructions could also be floated or placed elsewhere */}
        {/* <p>Use Left/Right Arrow Keys to move the paddle.</p> */}
      </AppContainer>
    </>
  );
}

export default App;