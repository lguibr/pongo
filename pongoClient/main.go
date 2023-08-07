package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/lguibr/asciiring/helpers"
	"golang.org/x/net/websocket"
	"golang.org/x/sys/unix"
)

type DirectionMessage struct {
	Direction string `json:"direction"`
}

func setRawMode(fileDescriptor uintptr) (*unix.Termios, error) {
	terminalSettings, err := unix.IoctlGetTermios(int(fileDescriptor), unix.TCGETS)
	if err != nil {
		return nil, err
	}
	savedTerminalSettings := *terminalSettings
	terminalSettings.Iflag &^= unix.BRKINT | unix.ICRNL | unix.INPCK | unix.ISTRIP | unix.IXON
	terminalSettings.Oflag &^= unix.OPOST
	terminalSettings.Lflag &^= unix.ECHO | unix.ICANON | unix.IEXTEN | unix.ISIG
	terminalSettings.Cflag &^= unix.CSIZE | unix.PARENB
	terminalSettings.Cflag |= unix.CS8
	terminalSettings.Oflag |= unix.ONLCR

	if err := unix.IoctlSetTermios(int(fileDescriptor), unix.TCSETS, terminalSettings); err != nil {
		return nil, err
	}
	return &savedTerminalSettings, nil
}

func formatASCII(ascii string, width int) string {
	var formatted strings.Builder

	lineCount := 0
	for _, runeValue := range ascii {
		if runeValue == '\n' {
			lineCount = 0
		} else {
			lineCount++
		}

		if lineCount <= width {
			formatted.WriteRune(runeValue)
		} else if runeValue != ' ' {
			formatted.WriteRune(runeValue)
			formatted.WriteRune('\n')
			lineCount = 1
		}
	}

	return formatted.String()
}

func main() {
	websocketConnection, err := websocket.Dial("ws://localhost:3001/subscribe", "", "http://localhost/")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer websocketConnection.Close()
	go func() {
		helpers.ClearScreen()
		fmt.Println("Received from server:")

		for {
			message := make([]byte, 0)
			buffer := make([]byte, 64) // This can be adjusted depending on the expected size of the messages
			for {
				size, err := websocketConnection.Read(buffer)
				if err != nil {
					fmt.Println("Error reading from server:", err)
					return
				}
				message = append(message, buffer[:size]...)
				if size < len(buffer) {
					// We've read the entire message
					break
				}
			}
			fmt.Print(string(message))
		}
	}()

	savedTerminalSettings, err := setRawMode(os.Stdin.Fd())
	if err != nil {
		fmt.Println("Error setting raw mode:", err)
		return
	}
	defer unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TCSETS, savedTerminalSettings)

	interruptSignalChannel := make(chan os.Signal)
	signal.Notify(interruptSignalChannel, os.Interrupt)
	go func() {
		<-interruptSignalChannel
		unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TCSETS, savedTerminalSettings)
		os.Exit(0)
	}()

	for {
		var singleByteBuffer []byte = make([]byte, 1)
		os.Stdin.Read(singleByteBuffer)
		var directionMessage DirectionMessage
		switch singleByteBuffer[0] {
		case 'd', 'D':
			directionMessage = DirectionMessage{Direction: "ArrowRight"}
		case 'a', 'A':
			directionMessage = DirectionMessage{Direction: "ArrowLeft"}
		case 'q', 'Q', 'c', 'C':
			fmt.Println("Quitting game")
			unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TCSETS, savedTerminalSettings)
			os.Exit(0)
		default:
			directionMessage = DirectionMessage{Direction: "None"}

		}

		jsonMessage, err := json.Marshal(directionMessage)
		if err != nil {
			fmt.Println("Error marshalling message:", err)
			return
		}

		if _, err := websocketConnection.Write(jsonMessage); err != nil {
			fmt.Println("Error sending to server:", err)
			return
		}
	}
}
