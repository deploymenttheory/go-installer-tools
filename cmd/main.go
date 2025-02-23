package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"os"

	"github.com/deploymenttheory/go-installer-tools/internal/logger"
	"github.com/deploymenttheory/go-installer-tools/internal/pkg/xar"
	"github.com/deploymenttheory/go-installer-tools/internal/reader"
)

func main() {
	// Parse command line flags.
	pkgPath := flag.String("pkg", "", "Path to the .pkg file to analyze")
	checkSig := flag.Bool("check-signature", false, "Check if the package is signed")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	// Initialize logger with specified level.
	if err := logger.Init(*logLevel); err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	if *pkgPath == "" {
		logger.Fatal("Please provide a path to a .pkg file using the -pkg flag")
	}

	// Open the package file.
	file, err := os.Open(*pkgPath)
	if err != nil {
		logger.Fatal("Error opening file",
			"error", err,
			"path", *pkgPath,
		)
	}
	defer file.Close()

	// Create a TempFileReader.
	tfr := &reader.TempFileReader{Reader: file}

	// Extract metadata.
	metadata, err := xar.ExtractXARMetadata(tfr)
	if err != nil {
		logger.Fatal("Error extracting metadata",
			"error", err,
			"path", *pkgPath,
		)
	}

	// Print the package report.
	fmt.Printf("\nPackage Analysis Report\n")
	fmt.Printf("=====================\n\n")

	fmt.Printf("Main Package\n")
	fmt.Printf("-----------\n")
	fmt.Printf("Name: %s\n", metadata.Name)
	fmt.Printf("Display Name: %s\n", metadata.DisplayName)
	fmt.Printf("Bundle Name: %s\n", metadata.BundleName)
	fmt.Printf("Version: %s\n", metadata.Version)
	fmt.Printf("Primary Bundle Identifier: %s\n", metadata.PrimaryBundleIdentifier)
	fmt.Printf("Minimum supported macOS Version: %s\n", metadata.MinimumOperatingSystemVersion)
	fmt.Printf("Package IDs: %v\n", metadata.PackageIDs)
	fmt.Printf("Supported Architecture(s): %s\n", metadata.HostArchitectures)
	fmt.Printf("Primary Bundle Path: %s\n", metadata.PrimaryBundlePath)
	fmt.Printf("PKG Size in MB: %.2f\n", metadata.PkgSizeMB)
	fmt.Printf("SHA256: %s\n", base64.StdEncoding.EncodeToString(metadata.SHA256Sum))
	fmt.Printf("MD5: %s\n", base64.StdEncoding.EncodeToString(metadata.MD5Sum))
	fmt.Printf("SHA1: %s\n", base64.StdEncoding.EncodeToString(metadata.SHA1Sum))

	// If any AppBundles were extracted, list them.
	if len(metadata.AppBundles) > 0 {
		fmt.Printf("\nApp Bundles\n")
		fmt.Printf("-----------\n")
		for i, ab := range metadata.AppBundles {
			fmt.Printf("Bundle %d:\n", i+1)
			fmt.Printf("  App Bundle ID: %s\n", ab.ID)
			fmt.Printf("  CFBundleShortVersionString: %s\n", ab.ShortVersion)
			fmt.Printf("  App Location Path: %s\n", ab.AppLocationPath)
		}
	}

	// If signature check was requested, perform and print results.
	if *checkSig {
		// Rewind the file for signature check.
		if _, err := file.Seek(0, 0); err != nil {
			logger.Fatal("Error rewinding file",
				"error", err,
				"path", *pkgPath,
			)
		}
		fmt.Printf("\nSignature Check\n")
		fmt.Printf("--------------\n")
		err := xar.CheckPKGSignature(file)
		switch err {
		case nil:
			fmt.Printf("Status: Signed ✓\n")
		case xar.ErrNotSigned:
			fmt.Printf("Status: Not signed ✗\n")
		case xar.ErrInvalidType:
			fmt.Printf("Status: Invalid XAR package ✗\n")
		default:
			fmt.Printf("Status: Error checking signature: %v ✗\n", err)
		}
	}

	fmt.Println() // Add final newline
}
