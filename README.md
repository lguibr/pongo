# PonGo

![Coverage](https://img.shields.io/badge/Coverage-78.9%25-brightgreen)
![Unit-tests](https://img.shields.io/github/actions/workflow/status/lguibr/pongo/test.yml?label=UnitTests)
![Building](https://img.shields.io/github/actions/workflow/status/lguibr/pongo/build.yml?label=Build)
![Lint](https://img.shields.io/github/actions/workflow/status/lguibr/pongo/lint.yml?label=Lint)

# Pongo

Welcome to my Pong and Breaking Brick Game!

![ScreenShoot](pongogif.gif)

This game combines elements of the classic Pong game with the gameplay of breaking brick games. The backend of the game is written in Go and utilizes the Actor Model pattern to handle different game elements, such as player input, game state, paddle movement, and ball movement. The game has zero dependencies.

The core entities in the game include the Player, GameState, Paddle, Ball, Grid, and the WebSocket connection between the server and player.

## Getting Started

To run the game, you will need to have Go installed on your system. Once you have Go installed, clone the repository and run the following command in the project directory:

```
go run main.go
```

This will start the game server, on your localhost:3001.

## Gameplay

The goal of the game is to break all of the bricks on the grid while keeping the ball from falling past the paddle. The player controls the paddle by sending input to the server via WebSockets.

The game state is handled by a separate routine that receives the ball and paddle positions and evaluates all collisions. The next state of the game is then sent to the player's WebSocket for display on the game frontend.

The paddle movement is handled by a separate routine that processes the paddle position based on the user's last input direction and the paddle's velocity. This data is sent to the game routine via channels.

A separate go routine is responsible for processing the ball position and sending it to the game routine every 20 milliseconds. The game routine then processes collisions and returns a new velocity for the ball, which is used to update the ball's position and reflect it off of bricks or the paddle.

## Build

To build the game, you can use the following command:

```
go build
```

## Testing

To test the game, you can use the following command:

```
go test
```

_Note that you should be in the project folder where the game files are located before running these commands._
