package xar

import (
	"encoding/xml"
	"fmt"
)

// distributionXML represents the structure of a Distribution file using XML tags.
type distributionXML struct {
	XMLName xml.Name `xml:"installer-gui-script"`
	Title   string   `xml:"title"`
	Options []struct {
		HostArchitectures string `xml:"hostArchitectures,attr"`
	} `xml:"options"`
	AllowedOSVersions struct {
		OsVersions []struct {
			Min string `xml:"min,attr"`
		} `xml:"os-version"`
	} `xml:"allowed-os-versions"`
	PkgRefs []struct {
		ID            string `xml:"id,attr"`
		Version       string `xml:"version,attr"`
		BundleVersion *struct {
			Bundles []struct {
				CFBundleShortVersionString string `xml:"CFBundleShortVersionString,attr"`
				CFBundleVersion            string `xml:"CFBundleVersion,attr"`
				ID                         string `xml:"id,attr"`
				Path                       string `xml:"path,attr"`
			} `xml:"bundle"`
		} `xml:"bundle-version"`
	} `xml:"pkg-ref"`
	Product struct {
		ID      string `xml:"id,attr"`
		Version string `xml:"version,attr"`
	} `xml:"product"`
}

// parseDistributionFile decodes the distribution file using the XML parser and extracts:
// - Title (used for Name and DisplayName)
// - Host Architectures
// - Minimum OS Version
// - Unique Package IDs (an index of all App Bundle IDs)
// - For each pkg-ref with a bundle-version, every bundle with a CFBundleShortVersionString and id is added to AppBundles.
// - Primary Bundle Identifier and Installation Path are set from the first pkg-ref whose bundle id matches its pkg-ref id.
func parseDistributionFile(rawXML []byte) (*PKGInstallerMetadata, error) {
	var distXML distributionXML
	if err := xml.Unmarshal(rawXML, &distXML); err != nil {
		return nil, fmt.Errorf("unmarshal Distribution XML: %w", err)
	}

	// Extract title.
	title := distXML.Title

	// Extract host architectures from the first <options> element.
	hostArch := ""
	if len(distXML.Options) > 0 {
		hostArch = distXML.Options[0].HostArchitectures
	}

	// Extract the minimum OS version from the first <os-version> element.
	minOS := ""
	if len(distXML.AllowedOSVersions.OsVersions) > 0 {
		minOS = distXML.AllowedOSVersions.OsVersions[0].Min
	}

	// Collect all bundle instances from pkg-refs.
	var appBundles []AppBundle
	primaryBundleIdentifier := ""
	primaryBundlePath := ""
	primaryFound := false

	for _, pkg := range distXML.PkgRefs {
		if pkg.BundleVersion != nil {
			for _, bundle := range pkg.BundleVersion.Bundles {
				if bundle.CFBundleShortVersionString != "" && bundle.ID != "" {
					// Avoid duplicates in appBundles.
					duplicate := false
					for _, ab := range appBundles {
						if ab.ID == bundle.ID {
							duplicate = true
							break
						}
					}
					if !duplicate {
						appBundles = append(appBundles, AppBundle{
							ID:              bundle.ID,
							ShortVersion:    bundle.CFBundleShortVersionString,
							AppLocationPath: bundle.Path,
						})
					}
					// Set primary if not already set and the pkg-ref id equals the bundle id.
					if !primaryFound && pkg.ID == bundle.ID {
						primaryBundleIdentifier = pkg.ID
						primaryBundlePath = bundle.Path
						primaryFound = true
					}
				}
			}
		}
	}
	// If no primary was found, use the first pkg-ref id from the appBundles list.
	if !primaryFound && len(appBundles) > 0 {
		primaryBundleIdentifier = appBundles[0].ID
		primaryBundlePath = appBundles[0].AppLocationPath
	}

	// Build PackageIDs as an index of all unique App Bundle IDs.
	pkgIDsMap := map[string]bool{}
	var pkgIDs []string
	for _, ab := range appBundles {
		if ab.ID != "" && !pkgIDsMap[ab.ID] {
			pkgIDs = append(pkgIDs, ab.ID)
			pkgIDsMap[ab.ID] = true
		}
	}

	// Choose overall version: prefer the first AppBundle short version if available,
	// otherwise fall back to the product version.
	version := ""
	if len(appBundles) > 0 {
		version = appBundles[0].ShortVersion
	} else {
		version = distXML.Product.Version
	}

	meta := &PKGInstallerMetadata{
		ApplicationTitle:              title,
		Version:                       version,
		PrimaryBundleIdentifier:       primaryBundleIdentifier, // Primary Bundle Identifier
		PackageIDs:                    pkgIDs,                  // Unique index of App Bundle IDs
		MinimumOperatingSystemVersion: minOS,
		DisplayName:                   title,
		HostArchitectures:             hostArch,
		PrimaryBundlePath:             primaryBundlePath,
		AppBundles:                    appBundles,
	}

	return meta, nil
}
