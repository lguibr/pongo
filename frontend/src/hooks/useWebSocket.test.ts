import { renderHook, act } from '@testing-library/react';
import { WebSocket, Server } from 'mock-socket';
import { useWebSocket } from './useWebSocket';
import { GameState } from '../types/game'; // Assuming GameState type definition exists

// Mock the global WebSocket
global.WebSocket = WebSocket;

const MOCK_SERVER_URL = 'ws://localhost:8080';

describe('useWebSocket Hook', () => {
  let mockServer: Server;

  beforeEach(() => {
    // Create a new mock server for each test
    mockServer = new Server(MOCK_SERVER_URL);
    // Basic connection acknowledgment
    mockServer.on('connection', socket => {
       console.log('Mock WS Server: Client connected');
       // Optionally send an initial state immediately on connection
        const initialState: GameState = { /* mock initial game state */
             canvas: { width: 800, height: 600, bricks: [] },
             players: {},
             balls: [],
             status: 'waiting',
             scores: {},
        };
       socket.send(JSON.stringify(initialState));
    });
  });

  afterEach(() => {
    // Close the mock server after each test
    mockServer.stop(() => {
         console.log('Mock WS Server: Stopped');
    });
  });

  it('should establish connection and update isConnected state', async () => {
    const { result, waitFor } = renderHook(() => useWebSocket(MOCK_SERVER_URL));

    await waitFor(() => expect(result.current.isConnected).toBe(true));
    console.log('Test: Connection established');
  });

  it('should receive messages and update gameState', async () => {
     const { result, waitFor } = renderHook(() => useWebSocket(MOCK_SERVER_URL));

     await waitFor(() => expect(result.current.isConnected).toBe(true));

     // Simulate server sending a message
     const testGameState: GameState = { /* mock updated game state */
         canvas: { width: 800, height: 600, bricks: [{ id: 1, x: 10, y: 10, hp: 1 }] },
         players: { 'player1': { id: 'player1', paddle: { x: 50, y: 580, width: 100, height: 10 } } },
         balls: [{ id: 'ball1', x: 100, y: 100, vx: 1, vy: -1, radius: 5 }],
         status: 'playing',
         scores: {'player1': 0},
     };

     act(() => {
       // Ensure mockServer.clients() has connected clients before sending
       if (mockServer.clients().length > 0) {
           mockServer.clients()[0].send(JSON.stringify(testGameState));
           console.log('Test: Mock server sent game state');
       } else {
           console.error('Test Error: No clients connected to mock server');
       }
     });

     // Wait for the hook to process the message and update state
     await waitFor(() => {
       expect(result.current.gameState).toEqual(testGameState);
     });
     console.log('Test: gameState updated correctly');
  });

  it('should send messages when sendMessage is called', async () => {
    const { result, waitFor } = renderHook(() => useWebSocket(MOCK_SERVER_URL));
    const messageToSend = { Direction: 'right' };
    let receivedMessage: string | null = null;

    mockServer.on('message', (message) => {
      console.log('Mock WS Server: Received message:', message);
      receivedMessage = message as string; // mock-socket usually sends strings
    });

    await waitFor(() => expect(result.current.isConnected).toBe(true));

    act(() => {
      result.current.sendMessage(messageToSend);
      console.log('Test: sendMessage called with:', messageToSend);
    });

    // Wait for the server to receive the message
    await waitFor(() => {
      expect(receivedMessage).not.toBeNull();
      expect(JSON.parse(receivedMessage!)).toEqual(messageToSend);
    });
    console.log('Test: Server received correct message');
  });

  it('should update isConnected to false on disconnection', async () => {
       const { result, waitFor } = renderHook(() => useWebSocket(MOCK_SERVER_URL));

       await waitFor(() => expect(result.current.isConnected).toBe(true));

       act(() => {
         // Simulate disconnection
         if (mockServer.clients().length > 0) {
             mockServer.clients()[0].close();
             console.log('Test: Simulated client disconnection');
         }
       });

       await waitFor(() => expect(result.current.isConnected).toBe(false));
       console.log('Test: isConnected is false after disconnect');
  });

});
