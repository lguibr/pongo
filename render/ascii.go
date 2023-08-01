package render

import (
	"fmt"
	"math"
	"os"
	"strings"

	"os/exec"
	"runtime"

	"github.com/lguibr/pongo/types"
)

func ClearScreen() {
	var cmd *exec.Cmd
	switch os := runtime.GOOS; os {
	case "windows":
		cmd = exec.Command("cmd", "/c", "cls")
	default: // Unix-like system
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// ASCII characters for grayscale, from lighter to darker
const asciiChars = " .,:;i1tfLCG08@"

// Dividing factor to convert RGB color space to grayscale
const grayFactor = 255.0 / float64(len(asciiChars)-1)

// Grayscale conversion factors for RGB components
const (
	RFactor = 1
	GFactor = 1
	BFactor = 1
)

// rgbToGray converts an RGB pixel to grayscale using the luminosity method
func rgbToGray(pixel types.RGBPixel) uint8 {
	r := RFactor * float64(pixel.R)
	g := GFactor * float64(pixel.G)
	b := BFactor * float64(pixel.B)
	return uint8(r + g + b)
}

// grayToAscii maps a grayscale value to an ASCII character
func grayToAscii(gray uint8) string {
	index := int(float64(gray) / grayFactor)
	return string(asciiChars[index])
}

// rgbToAnsi converts an RGB pixel to an ANSI escape code for that color
func rgbToAnsi(pixel types.RGBPixel) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm", pixel.R, pixel.G, pixel.B)
}

// RenderToASCII converts a 2D slice of types.RGBPixels to an ASCII string
func RenderToASCII(pixels [][]types.RGBPixel, resolution int) string {
	height := len(pixels)
	if height == 0 {
		return ""
	}
	width := len(pixels[0])
	stepX, stepY := float64(width)/float64(resolution), float64(height)/float64(resolution)
	var ascii strings.Builder
	for y := 0.0; y < float64(height-1); y += stepY {
		for x := 0.0; x < float64(width-1); x += stepX {
			i, j := int(math.Round(x)), int(math.Round(y))
			pixel := pixels[j][i]
			gray := rgbToGray(pixel)
			// Convert pixel to colored ASCII character
			ansi := rgbToAnsi(pixel)
			ascii.WriteString(ansi + grayToAscii(gray) + "\033[0m") // Reset color after each character
			ascii.WriteString(ansi + grayToAscii(gray) + "\033[0m") // Reset color after each character
		}
		ascii.WriteString("\n")
	}
	return ascii.String()
}
