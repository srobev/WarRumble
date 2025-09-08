package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run fix_png_converter.go <png_file>")
		os.Exit(1)
	}

	inputFile := os.Args[1]

	// Open the PNG file
	file, err := os.Open(inputFile)
	if err != nil {
		fmt.Printf("Failed to open file %s: %v\n", inputFile, err)
		os.Exit(1)
	}
	defer file.Close()

	// Try to decode as PNG first
	var img image.Image
	img, err = png.Decode(file)
	if err != nil {
		fmt.Printf("Failed to decode PNG %s: %v\n", inputFile, err)
		os.Exit(1)
	}

	// Create output filename
	dir := filepath.Dir(inputFile)
	base := strings.TrimSuffix(filepath.Base(inputFile), filepath.Ext(inputFile))
	outputFile := filepath.Join(dir, base+"_fixed.png")

	// Create the output file
	out, err := os.Create(outputFile)
	if err != nil {
		fmt.Printf("Failed to create output file %s: %v\n", outputFile, err)
		os.Exit(1)
	}
	defer out.Close()

	// Encode as a standard PNG (this often fixes compatibility issues)
	err = png.Encode(out, img)
	if err != nil {
		fmt.Printf("Failed to encode PNG %s: %v\n", outputFile, err)
		os.Exit(1)
	}

	fmt.Printf("Converted %s to %s\n", inputFile, outputFile)
	fmt.Println("Try loading the _fixed.png version in the map editor")
}
