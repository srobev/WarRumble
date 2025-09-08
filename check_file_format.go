package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run check_file_format.go <file>")
		os.Exit(1)
	}

	filename := os.Args[1]

	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Failed to open file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Read first 16 bytes to check file signature
	buffer := make([]byte, 16)
	n, err := file.Read(buffer)
	if err != nil {
		fmt.Printf("Failed to read file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("File: %s\n", filepath.Base(filename))
	fmt.Printf("First %d bytes (hex): ", n)
	for i := 0; i < n; i++ {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Printf("%02x", buffer[i])
	}
	fmt.Println()

	// Check common file signatures
	if n >= 8 {
		if buffer[0] == 0x89 && buffer[1] == 0x50 && buffer[2] == 0x4E && buffer[3] == 0x47 {
			fmt.Println("File signature suggests: PNG")
		} else if buffer[0] == 0xFF && buffer[1] == 0xD8 {
			fmt.Println("File signature suggests: JPEG")
		} else if buffer[0] == 0x42 && buffer[1] == 0x4D {
			fmt.Println("File signature suggests: BMP")
		} else if buffer[0] == 0x47 && buffer[1] == 0x49 && buffer[2] == 0x46 {
			fmt.Println("File signature suggests: GIF")
		} else {
			fmt.Println("File signature: Unknown format")
		}
	}

	// Get file size
	stat, err := file.Stat()
	if err == nil {
		fmt.Printf("File size: %d bytes\n", stat.Size())
	}
}
