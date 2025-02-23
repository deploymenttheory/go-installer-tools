package xar

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/deploymenttheory/go-installer-tools/internal/logger"
	"howett.net/plist"
)

// PackageInfo represents the structure of a PKG's PackageInfo plist
type PackageInfo struct {
	Version         string       `plist:"version"`
	InstallLocation string       `plist:"install-location"`
	Identifier      string       `plist:"identifier"`
	Bundles         []BundleInfo `plist:"bundles"`
}

// BundleInfo represents a bundle in the PackageInfo plist
type BundleInfo struct {
	Path                       string `plist:"path"`
	ID                         string `plist:"id"`
	CFBundleShortVersionString string `plist:"CFBundleShortVersionString"`
	CFBundleDisplayName        string `plist:"CFBundleDisplayName"`
	CFBundleIdentifier         string `plist:"CFBundleIdentifier"`
	CFBundleName               string `plist:"CFBundleName"`
	LSMinimumSystemVersion     string `plist:"LSMinimumSystemVersion"`
}

func parsePackageInfoFile(rawData []byte) (*PKGInstallerMetadata, error) {
	// First detect format
	var dummy interface{}
	format, err := plist.Unmarshal(rawData, &dummy)
	if err == nil {
		logger.Debug("Detected plist format",
			"format", plist.FormatNames[format])

		// Create indented version for debugging
		indentedData, err := plist.MarshalIndent(dummy, format, "    ")
		if err == nil {
			logger.Debug("Raw plist content (indented)",
				"content", string(indentedData))
		}
	}

	var packageInfo PackageInfo
	decoder := plist.NewDecoder(bytes.NewReader(rawData))

	if err := decoder.Decode(&packageInfo); err != nil {
		return nil, fmt.Errorf("decode PackageInfo plist: %w", err)
	}

	// Log detailed bundle information
	for i, bundle := range packageInfo.Bundles {
		// Convert each bundle to XML for readable debug output
		bundleBytes, err := plist.MarshalIndent(bundle, plist.XMLFormat, "    ")
		if err == nil {
			logger.Debug("Bundle details",
				"index", i,
				"content", string(bundleBytes))
		}
	}

	name, identifier, version, packageIDs, displayName, bundleName, minOSVersion := getPackageInfo(&packageInfo)

	// Log the extracted metadata
	logger.Debug("Extracted package metadata",
		"name", name,
		"identifier", identifier,
		"version", version,
		"packageIDs", packageIDs,
		"displayName", displayName,
		"bundleName", bundleName,
		"minOSVersion", minOSVersion)

	metadata := &PKGInstallerMetadata{
		ApplicationTitle:              name,
		Version:                       version,
		PrimaryBundleIdentifier:       identifier,
		PackageIDs:                    packageIDs,
		DisplayName:                   displayName,
		MinimumOperatingSystemVersion: minOSVersion,
	}

	logger.Info("Successfully parsed PackageInfo file",
		"application title", metadata.ApplicationTitle,
		"version", metadata.Version,
		"identifier", metadata.PrimaryBundleIdentifier,
		"format", plist.FormatNames[decoder.Format])

	return metadata, nil
}

// sanitizeBundleString cleans and validates bundle-related strings
func sanitizeBundleString(s string) string {
	s = strings.TrimSpace(s)
	invalidChars := strings.NewReplacer(
		"\n", "",
		"\r", "",
		"\t", "",
	)
	s = invalidChars.Replace(s)
	return s
}

// getPackageInfo extracts metadata from a PKG's PackageInfo plist
func getPackageInfo(p *PackageInfo) (name string, identifier string, version string, packageIDs []string, displayName string, bundleName string, minOSVersion string) {
	packageIDSet := make(map[string]struct{}, 1)

	for _, bundle := range p.Bundles {
		installPath := bundle.Path
		if p.InstallLocation != "" {
			installPath = filepath.Join(p.InstallLocation, installPath)
		}
		installPath = strings.TrimPrefix(installPath, "/")
		installPath = strings.TrimPrefix(installPath, "./")

		if base, isValid := isValidAppFilePath(installPath); isValid {
			identifier = sanitizeBundleString(bundle.ID)
			name = base
			version = sanitizeBundleString(bundle.CFBundleShortVersionString)
			displayName = sanitizeBundleString(bundle.CFBundleDisplayName)
			bundleName = sanitizeBundleString(bundle.CFBundleName)
			minOSVersion = sanitizeBundleString(bundle.LSMinimumSystemVersion)
		}

		bundleID := sanitizeBundleString(bundle.ID)
		if bundleID != "" {
			packageIDSet[bundleID] = struct{}{}
		}
	}

	// Convert set to slice
	for id := range packageIDSet {
		packageIDs = append(packageIDs, id)
	}

	// Fallback to package-level version if no bundle version found
	if version == "" {
		version = sanitizeBundleString(p.Version)
	}

	// Fallback to package identifier if no bundle identifier found
	if identifier == "" {
		identifier = sanitizeBundleString(p.Identifier)
	}

	// Extract name from identifier if not found from bundles
	if name == "" {
		idParts := strings.Split(identifier, ".")
		if len(idParts) > 0 {
			name = idParts[len(idParts)-1]
		}
	}

	// Use identifier as package ID if no bundle IDs found
	if len(packageIDs) == 0 && identifier != "" {
		packageIDs = append(packageIDs, identifier)
	}

	return name, identifier, version, packageIDs, displayName, bundleName, minOSVersion
}

// isValidAppFilePath checks if the given input is a file name ending with .app
// or if it's in the "Applications" directory with a .app extension.
func isValidAppFilePath(input string) (string, bool) {
	dir, file := filepath.Split(input)

	if dir == "" && file == input {
		return file, true
	}

	if strings.HasSuffix(file, ".app") {
		if dir == "Applications/" {
			return file, true
		}
	}

	return "", false
}
