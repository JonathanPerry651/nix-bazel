package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	srcPath := flag.String("src", "", "Source file path")
	dstPath := flag.String("dst", "", "Destination file path")

	flag.Parse()

	if *srcPath == "" || *dstPath == "" {
		fmt.Fprintln(os.Stderr, "Error: -src and -dst are required")
		flag.Usage()
		os.Exit(1)
	}

	if err := copyFile(*srcPath, *dstPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error copying file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully copied %s to %s\n", *srcPath, *dstPath)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy content: %w", err)
	}

	// Ensure content is flushed to disk
	if err := destFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync destination file: %w", err)
	}

	return nil
}
