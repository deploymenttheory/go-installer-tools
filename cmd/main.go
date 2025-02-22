package main

import (
	"flag"
	"os"

	"github.com/deploymenttheory/go-installer-tools/internal/logger"
	"github.com/deploymenttheory/go-installer-tools/internal/pkg/xar"
	"github.com/deploymenttheory/go-installer-tools/internal/reader"
)

func main() {
	// Initialize logger with info level
	if err := logger.Init("info"); err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	defer logger.Sync()

	// Parse command line flags
	pkgPath := flag.String("pkg", "", "Path to the .pkg file to analyze")
	checkSig := flag.Bool("check-signature", false, "Check if the package is signed")
	flag.Parse()

	if *pkgPath == "" {
		logger.Fatal("Please provide a path to a .pkg file using the -pkg flag")
	}

	// Open the package file
	file, err := os.Open(*pkgPath)
	if err != nil {
		logger.Fatal("Error opening file",
			"error", err,
			"path", *pkgPath,
		)
	}
	defer file.Close()

	// Create a TempFileReader
	tfr := &reader.TempFileReader{Reader: file}

	// Extract metadata
	metadata, err := xar.ExtractXARMetadata(tfr)
	if err != nil {
		logger.Fatal("Error extracting metadata",
			"error", err,
			"path", *pkgPath,
		)
	}

	// Log the metadata with new fields
	logger.Info("Package metadata extracted successfully",
		"name", metadata.Name,
		"displayName", metadata.DisplayName,
		"bundleName", metadata.BundleName,
		"version", metadata.Version,
		"bundleIdentifier", metadata.BundleIdentifier,
		"minimumSystemVersion", metadata.MinimumSystemVersion,
		"packageIDs", metadata.PackageIDs,
		"sha256", metadata.SHASum,
		"extension", metadata.Extension,
	)

	// If signature check was requested
	if *checkSig {
		// Rewind the file for signature check
		if _, err := file.Seek(0, 0); err != nil {
			logger.Fatal("Error rewinding file",
				"error", err,
				"path", *pkgPath,
			)
		}

		err := xar.CheckPKGSignature(file)
		switch err {
		case nil:
			logger.Info("Package is signed ✓")
		case xar.ErrNotSigned:
			logger.Warn("Package is not signed ✗")
		case xar.ErrInvalidType:
			logger.Error("Not a valid XAR package ✗")
		default:
			logger.Fatal("Error checking signature",
				"error", err,
				"path", *pkgPath,
			)
		}
	}
}
