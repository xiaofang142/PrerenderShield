package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Broken extraction with defer inside loop (original buggy version)
func brokenExtractZIP(filePath, destDir string) error {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	fmt.Println("Broken extraction (with defer in loop):")
	for _, file := range reader.File {
		destFilePath := filepath.Join(destDir, file.Name)
		fmt.Printf("  Processing: %s -> %s\n", file.Name, destFilePath)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destFilePath, file.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destFilePath), 0755); err != nil {
			return err
		}

		destFile, err := os.OpenFile(destFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		defer destFile.Close() // THIS IS THE BUG! defer inside loop

		zipFile, err := file.Open()
		if err != nil {
			return err
		}
		defer zipFile.Close() // THIS IS THE BUG! defer inside loop

		if _, err := io.Copy(destFile, zipFile); err != nil {
			return err
		}
		fmt.Printf("  Copied %d bytes from %s to %s\n", file.FileInfo().Size(), file.Name, destFilePath)
	}

	return nil
}

// Fixed extraction with immediate close (our fix)
func fixedExtractZIP(filePath, destDir string) error {
	reader, err := zip.OpenReader(filePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	fmt.Println("Fixed extraction (with immediate close):")
	for _, file := range reader.File {
		destFilePath := filepath.Join(destDir, file.Name)
		fmt.Printf("  Processing: %s -> %s\n", file.Name, destFilePath)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destFilePath, file.Mode()); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destFilePath), 0755); err != nil {
			return err
		}

		destFile, err := os.OpenFile(destFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}

		zipFile, err := file.Open()
		if err != nil {
			destFile.Close() // Ensure file is closed
			return err
		}

		if _, err := io.Copy(destFile, zipFile); err != nil {
			zipFile.Close()
			destFile.Close()
			return err
		}

		// Immediately close files instead of defer
		zipFile.Close()
		destFile.Close()
		fmt.Printf("  Copied %d bytes from %s to %s\n", file.FileInfo().Size(), file.Name, destFilePath)
	}

	return nil
}

func main() {
	// Test ZIP file path
	zipFilePath := "./test-structured.zip"
	
	// Test 1: Broken extraction
	fmt.Println("=== Test 1: Broken Extraction (defer in loop) ===")
	err := brokenExtractZIP(zipFilePath, "./test_broken")
	if err != nil {
		fmt.Printf("Broken extraction error: %v\n", err)
	} else {
		fmt.Println("Broken extraction completed successfully!")
		// Check what was actually extracted
		fmt.Println("Files in test_broken:")
		files, _ := os.ReadDir("./test_broken")
		for _, f := range files {
			fmt.Printf("  - %s\n", f.Name())
		}
	}
	fmt.Println()

	// Test 2: Fixed extraction
	fmt.Println("=== Test 2: Fixed Extraction (immediate close) ===")
	err = fixedExtractZIP(zipFilePath, "./test_fixed")
	if err != nil {
		fmt.Printf("Fixed extraction error: %v\n", err)
	} else {
		fmt.Println("Fixed extraction completed successfully!")
		// Check what was actually extracted
		fmt.Println("Files in test_fixed:")
		files, _ := os.ReadDir("./test_fixed")
		for _, f := range files {
			fmt.Printf("  - %s\n", f.Name())
		}
		// Check subdirectory
		subFiles, _ := os.ReadDir("./test_fixed/test-dir")
		fmt.Println("Files in test_fixed/test-dir:")
		for _, f := range subFiles {
			fmt.Printf("  - %s\n", f.Name())
		}
	}

	// Cleanup
	os.RemoveAll("./test_broken")
	os.RemoveAll("./test_fixed")
}
