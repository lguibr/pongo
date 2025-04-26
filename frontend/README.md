
# PonGo Frontend

This directory contains the React frontend for the PonGo game.

## Tech Stack

*   React
*   TypeScript
*   Vite
*   Styled Components

## Getting Started

1.  **Navigate to the frontend directory:**
    ```bash
    cd frontend
    ```
2.  **Install dependencies:**
    ```bash
    npm install
    ```
3.  **Run the development server:**
    ```bash
    npm run dev
    ```
    This will typically start the frontend on `http://localhost:5173`.

4.  **Ensure the Go backend is running:**
    The backend server (usually on `http://localhost:3001`) must be running for the frontend to connect via WebSockets.

## Building for Production

```bash
npm run build
```

This will create a `dist` folder with the optimized production build.

## Related Modules

*   [PonGo Game](../game/README.md)
*   [Bollywood Actor Library](../bollywood/README.md)
*   [Main Project](../README.md)