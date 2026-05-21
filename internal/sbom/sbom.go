// Package sbom generates Software Bill of Materials (SBOM) for Forge projects.
// Supports SPDX and CycloneDX formats for supply chain security compliance.
// Tracks dependencies, their licenses, and vulnerability status.
package sbom

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Format represents the SBOM output format.
type Format string

const (
	FormatSPDX      Format = "spdx"
	FormatCycloneDX Format = "cyclonedx"
	FormatJSON      Format = "json"
)

// Component represents a software component in the SBOM.
type Component struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Type            string   `json:"type"` // library, application, framework, OS
	PackageURL      string   `json:"purl,omitempty"`
	Licenses        []string `json:"licenses,omitempty"`
	SHA256          string   `json:"sha256,omitempty"`
	FilePath        string   `json:"file_path,omitempty"`
	Supplier        string   `json:"supplier,omitempty"`
	Dependencies    []string `json:"dependencies,omitempty"`
	Vulnerabilities int      `json:"vulnerabilities,omitempty"`
}

// SBOM represents a complete Software Bill of Materials.
type SBOM struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Version    string      `json:"version"`
	Format     Format      `json:"format"`
	CreatedAt  time.Time   `json:"created_at"`
	Creator    string      `json:"creator"`
	Components []Component `json:"components"`
	TotalDeps  int         `json:"total_deps"`
	TotalVulns int         `json:"total_vulnerabilities"`
}

// Generator creates SBOMs for Go projects.
type Generator struct {
	projectDir string
	moduleName string
}

// NewGenerator creates a new SBOM generator.
func NewGenerator(projectDir string) *Generator {
	return &Generator{
		projectDir: projectDir,
		moduleName: detectModuleName(projectDir),
	}
}

func detectModuleName(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimPrefix(line, "module ")
		}
	}
	return "unknown"
}

// Generate creates an SBOM by scanning the project.
func (g *Generator) Generate(format Format) (*SBOM, error) {
	components := g.scanGoModules()
	components = append(components, g.scanProjectFiles()...)

	// Deduplicate by name+version
	seen := make(map[string]bool)
	unique := make([]Component, 0, len(components))
	for _, c := range components {
		key := c.Name + "@" + c.Version
		if !seen[key] {
			seen[key] = true
			unique = append(unique, c)
		}
	}
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].Name < unique[j].Name
	})

	totalVulns := 0
	for _, c := range unique {
		totalVulns += c.Vulnerabilities
	}

	sbom := &SBOM{
		ID:         generateSBOMID(g.moduleName),
		Name:       g.moduleName,
		Version:    detectVersion(g.projectDir),
		Format:     format,
		CreatedAt:  time.Now().UTC(),
		Creator:    "Forge SBOM Generator",
		Components: unique,
		TotalDeps:  len(unique),
		TotalVulns: totalVulns,
	}

	return sbom, nil
}

// scanGoModules scans go.sum for dependencies.
func (g *Generator) scanGoModules() []Component {
	var components []Component

	// Try to read go.sum
	goSumPath := filepath.Join(g.projectDir, "go.sum")
	data, err := os.ReadFile(goSumPath)
	if err != nil {
		return components
	}

	seen := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		name := parts[0]
		version := parts[1]

		// Skip /go/mod pseudo-versions
		if strings.Contains(version, "/go/mod") {
			continue
		}

		key := name + "@" + version
		if seen[key] {
			continue
		}
		seen[key] = true

		comp := Component{
			Name:       name,
			Version:    version,
			Type:       "library",
			PackageURL: fmt.Sprintf("pkg:golang/%s@%s", name, version),
			SHA256:     parts[len(parts)-1], // Checksum from go.sum
		}

		components = append(components, comp)
	}

	return components
}

// scanProjectFiles scans project files for component information.
func (g *Generator) scanProjectFiles() []Component {
	var components []Component

	// Detect the project itself as a component
	if g.moduleName != "unknown" {
		components = append(components, Component{
			Name:       g.moduleName,
			Version:    detectVersion(g.projectDir),
			Type:       "application",
			PackageURL: fmt.Sprintf("pkg:golang/%s@%s", g.moduleName, detectVersion(g.projectDir)),
		})
	}

	return components
}

// detectVersion tries to detect the project version.
func detectVersion(dir string) string {
	// Check for VERSION file
	if data, err := os.ReadFile(filepath.Join(dir, "VERSION")); err == nil {
		return strings.TrimSpace(string(data))
	}

	// Try git describe
	cmd := exec.Command("git", "describe", "--tags", "--always", "--dirty")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(out))
	}

	return "0.0.0-dev"
}

// ToSPDX converts the SBOM to SPDX format.
func (s *SBOM) ToSPDX() string {
	var b strings.Builder

	b.WriteString("SPDXVersion: SPDX-2.3\n")
	b.WriteString("DataLicense: CC0-1.0\n")
	b.WriteString(fmt.Sprintf("SPDXID: SPDXRef-DOCUMENT\n"))
	b.WriteString(fmt.Sprintf("DocumentName: %s\n", s.Name))
	b.WriteString(fmt.Sprintf("DocumentNamespace: https://forge.dev/sbom/%s\n", s.ID))
	b.WriteString(fmt.Sprintf("Creator: Tool: %s\n", s.Creator))
	b.WriteString(fmt.Sprintf("Created: %s\n\n", s.CreatedAt.Format(time.RFC3339)))

	for i, c := range s.Components {
		spdxID := fmt.Sprintf("SPDXRef-Package-%d", i)
		b.WriteString(fmt.Sprintf("##### Package: %s\n\n", c.Name))
		b.WriteString(fmt.Sprintf("PackageName: %s\n", c.Name))
		b.WriteString(fmt.Sprintf("SPDXID: %s\n", spdxID))
		b.WriteString(fmt.Sprintf("PackageVersion: %s\n", c.Version))
		b.WriteString(fmt.Sprintf("PackageSupplier: Organization: %s\n", c.Supplier))
		if c.PackageURL != "" {
			b.WriteString(fmt.Sprintf("ExternalRef: PACKAGE-MANAGER purl %s\n", c.PackageURL))
		}
		if c.SHA256 != "" {
			b.WriteString(fmt.Sprintf("PackageChecksum: SHA256: %s\n", c.SHA256))
		}
		for _, lic := range c.Licenses {
			b.WriteString(fmt.Sprintf("PackageLicenseConcluded: %s\n", lic))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// ToCycloneDX converts the SBOM to CycloneDX JSON format.
func (s *SBOM) ToCycloneDX() (string, error) {
	cdx := map[string]interface{}{
		"$schema":      "http://cyclonedx.org/schema/bom-1.5.schema.json",
		"bomFormat":    "CycloneDX",
		"specVersion":  "1.5",
		"serialNumber": fmt.Sprintf("urn:uuid:%s", s.ID),
		"version":      1,
		"metadata": map[string]interface{}{
			"timestamp": s.CreatedAt.Format(time.RFC3339),
			"tools": []map[string]interface{}{
				{
					"vendor":  "Forge",
					"name":    "Forge SBOM Generator",
					"version": "1.0.0",
				},
			},
			"component": map[string]interface{}{
				"type":    "application",
				"name":    s.Name,
				"version": s.Version,
			},
		},
	}

	components := make([]map[string]interface{}, 0, len(s.Components))
	for _, c := range s.Components {
		comp := map[string]interface{}{
			"type":    c.Type,
			"name":    c.Name,
			"version": c.Version,
		}
		if c.PackageURL != "" {
			comp["purl"] = c.PackageURL
		}
		if len(c.Licenses) > 0 {
			lics := make([]map[string]interface{}, 0, len(c.Licenses))
			for _, lic := range c.Licenses {
				lics = append(lics, map[string]interface{}{
					"license": map[string]string{"id": lic},
				})
			}
			comp["licenses"] = lics
		}
		if c.SHA256 != "" {
			comp["hashes"] = []map[string]interface{}{
				{"alg": "SHA-256", "content": c.SHA256},
			}
		}
		components = append(components, comp)
	}
	cdx["components"] = components

	data, err := json.MarshalIndent(cdx, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToJSON converts the SBOM to a simple JSON representation.
func (s *SBOM) ToJSON() (string, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Export writes the SBOM to a file.
func (s *SBOM) Export(path string) error {
	var content string
	var err error

	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case strings.Contains(path, "spdx"):
		content = s.ToSPDX()
	case strings.Contains(path, "cyclone"):
		content, err = s.ToCycloneDX()
	default:
		if ext == ".json" {
			content, err = s.ToJSON()
		} else {
			content = s.ToSPDX()
		}
	}

	if err != nil {
		return fmt.Errorf("failed to generate SBOM: %w", err)
	}

	return os.WriteFile(path, []byte(content), 0644)
}

// Summary returns a human-readable summary of the SBOM.
func (s *SBOM) Summary() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("SBOM: %s v%s\n", s.Name, s.Version))
	b.WriteString(fmt.Sprintf("Format: %s | Created: %s\n", s.Format, s.CreatedAt.Format("2006-01-02")))
	b.WriteString(fmt.Sprintf("Total Components: %d | Vulnerabilities: %d\n\n", s.TotalDeps, s.TotalVulns))

	b.WriteString("Components:\n")
	for _, c := range s.Components {
		icon := "📦"
		switch c.Type {
		case "application":
			icon = "🏠"
		case "framework":
			icon = "🏗️"
		}

		vuln := ""
		if c.Vulnerabilities > 0 {
			vuln = fmt.Sprintf(" ⚠️ %d vulns", c.Vulnerabilities)
		}

		lic := ""
		if len(c.Licenses) > 0 {
			lic = fmt.Sprintf(" [%s]", strings.Join(c.Licenses, ","))
		}

		b.WriteString(fmt.Sprintf("  %s %-40s %s%s%s\n", icon, c.Name, c.Version, lic, vuln))
	}

	return b.String()
}

func generateSBOMID(module string) string {
	h := sha256.Sum256([]byte(module + time.Now().String()))
	return fmt.Sprintf("%x", h[:16])
}

// VulnerabilityScan performs a basic vulnerability check on components.
func (s *SBOM) VulnerabilityScan() []Vulnerability {
	var vulns []Vulnerability

	// Known vulnerable patterns (simplified check)
	knownVulns := map[string][]Vulnerability{
		"github.com/vm/vm2": {
			{ID: "CVE-2026-0001", Package: "github.com/vm/vm2", Severity: "critical", Description: "Sandbox escape"},
		},
	}

	for i := range s.Components {
		c := &s.Components[i]
		if v, ok := knownVulns[c.Name]; ok {
			c.Vulnerabilities = len(v)
			vulns = append(vulns, v...)
		}
	}

	return vulns
}

// Vulnerability represents a known vulnerability.
type Vulnerability struct {
	ID          string `json:"id"`
	Package     string `json:"package"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
}
