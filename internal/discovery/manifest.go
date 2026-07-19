package discovery

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"
)

type ProjectDeclaration struct {
	ManifestPath string   `json:"manifestPath"`
	Path         string   `json:"path"`
	Name         string   `json:"name,omitempty"`
	Kind         string   `json:"kind"`
	Dependencies []string `json:"dependencies,omitempty"`
	Confidence   string   `json:"confidence"`
}

type pomProject struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Packaging  string `xml:"packaging"`
	Modules    struct {
		Module []string `xml:"module"`
	} `xml:"modules"`
	Dependencies struct {
		Dependency []struct {
			GroupID    string `xml:"groupId"`
			ArtifactID string `xml:"artifactId"`
		} `xml:"dependency"`
	} `xml:"dependencies"`
}

var gradleIncludeRE = regexp.MustCompile(`(?m)\binclude\s*(?:\(\s*)?([^\n\)]*)`)
var gradleQuoteRE = regexp.MustCompile(`['"](:[^'"]+)['"]`)
var gradleIncludedBuildRE = regexp.MustCompile(`(?m)\bincludeBuild\s*\(\s*['"]([^'"]+)['"]`)
var gradleDependencyRE = regexp.MustCompile(`(?m)\b(?:implementation|api|compileOnly|runtimeOnly|testImplementation)\s*\(?\s*['"]([^'"]+)['"]`)

func parseManifest(path, content string) ([]ProjectDeclaration, []string) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, "pom.xml"):
		return parsePOM(path, content)
	case strings.HasSuffix(lower, "settings.gradle"), strings.HasSuffix(lower, "settings.gradle.kts"):
		return parseGradleSettings(path, content)
	case strings.HasSuffix(lower, "package.json"):
		return parsePackageManifest(path, content)
	case strings.HasSuffix(lower, "go.mod"):
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "module ") {
				return []ProjectDeclaration{{ManifestPath: path, Path: ".", Name: strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "module ")), Kind: "go-module", Confidence: "high"}}, nil
			}
		}
	case strings.HasSuffix(lower, "go.work"):
		var projects []ProjectDeclaration
		inUseBlock := false
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "use (") {
				inUseBlock = true
				continue
			}
			if inUseBlock && line == ")" {
				inUseBlock = false
				continue
			}
			value := ""
			if strings.HasPrefix(line, "use ") {
				value = strings.TrimSpace(strings.TrimPrefix(line, "use "))
			} else if inUseBlock && line != "" && !strings.HasPrefix(line, "//") {
				value = strings.Fields(line)[0]
			}
			if value != "" {
				projects = append(projects, ProjectDeclaration{ManifestPath: path, Path: value, Name: value, Kind: "go-workspace", Confidence: "high"})
			}
		}
		return projects, nil
	case strings.HasSuffix(lower, "pyproject.toml"):
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "name =") {
				return []ProjectDeclaration{{ManifestPath: path, Path: ".", Name: strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "name =")), "\"'"), Kind: "python-project", Confidence: "medium"}}, nil
			}
		}
	case strings.HasSuffix(lower, "build.gradle"), strings.HasSuffix(lower, "build.gradle.kts"):
		var projects []ProjectDeclaration
		for _, match := range gradleDependencyRE.FindAllStringSubmatch(content, -1) {
			projects = append(projects, ProjectDeclaration{ManifestPath: path, Path: ".", Name: match[1], Kind: "gradle-dependency", Dependencies: []string{match[1]}, Confidence: "medium"})
		}
		return projects, nil
	case strings.HasSuffix(lower, "csproj"), strings.HasSuffix(lower, "fsproj"):
		return parseDotNetProject(path, content)
	}
	return nil, nil
}

func parsePOM(path, content string) ([]ProjectDeclaration, []string) {
	var project pomProject
	if err := xml.Unmarshal([]byte(content), &project); err != nil {
		return nil, []string{fmt.Sprintf("%s: malformed Maven XML: %v", path, err)}
	}
	projects := []ProjectDeclaration{{ManifestPath: path, Path: ".", Name: project.ArtifactID, Kind: "maven-project", Confidence: "high"}}
	for _, dependency := range project.Dependencies.Dependency {
		if dependency.GroupID != "" || dependency.ArtifactID != "" {
			projects[0].Dependencies = append(projects[0].Dependencies, dependency.GroupID+":"+dependency.ArtifactID)
		}
	}
	for _, module := range project.Modules.Module {
		module = strings.TrimSpace(module)
		if module == "" {
			continue
		}
		projects = append(projects, ProjectDeclaration{ManifestPath: path, Path: module, Name: module, Kind: "maven-module", Confidence: "high"})
	}
	return projects, nil
}

func parseGradleSettings(path, content string) ([]ProjectDeclaration, []string) {
	var projects []ProjectDeclaration
	for _, match := range gradleIncludeRE.FindAllStringSubmatch(content, -1) {
		for _, quote := range gradleQuoteRE.FindAllStringSubmatch(match[1], -1) {
			projectPath := strings.TrimPrefix(strings.TrimSpace(quote[1]), ":")
			projectPath = strings.ReplaceAll(projectPath, ":", "/")
			projects = append(projects, ProjectDeclaration{ManifestPath: path, Path: projectPath, Name: projectPath, Kind: "gradle-project", Confidence: "high"})
		}
	}
	for _, match := range gradleIncludedBuildRE.FindAllStringSubmatch(content, -1) {
		projects = append(projects, ProjectDeclaration{ManifestPath: path, Path: match[1], Name: match[1], Kind: "gradle-included-build", Confidence: "high"})
	}
	return projects, nil
}

func parsePackageManifest(path, content string) ([]ProjectDeclaration, []string) {
	var value struct {
		Name       string `json:"name"`
		Workspaces any    `json:"workspaces"`
	}
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		return nil, []string{fmt.Sprintf("%s: malformed package JSON: %v", path, err)}
	}
	projects := []ProjectDeclaration{{ManifestPath: path, Path: ".", Name: value.Name, Kind: "node-project", Confidence: "high"}}
	var workspaceValues []any
	switch raw := value.Workspaces.(type) {
	case []any:
		workspaceValues = raw
	case map[string]any:
		if packages, ok := raw["packages"].([]any); ok {
			workspaceValues = packages
		}
	}
	for _, item := range workspaceValues {
		if workspace, ok := item.(string); ok {
			projects = append(projects, ProjectDeclaration{ManifestPath: path, Path: workspace, Name: workspace, Kind: "node-workspace", Confidence: "high"})
		}
	}
	return projects, nil
}

func parseDotNetProject(path, content string) ([]ProjectDeclaration, []string) {
	var project struct {
		References []struct {
			Include string `xml:"Include,attr"`
		} `xml:"ItemGroup>ProjectReference"`
	}
	if err := xml.Unmarshal([]byte(content), &project); err != nil {
		return nil, []string{fmt.Sprintf("%s: malformed .NET XML: %v", path, err)}
	}
	projects := []ProjectDeclaration{{ManifestPath: path, Path: ".", Kind: "dotnet-project", Confidence: "high"}}
	for _, ref := range project.References {
		if ref.Include != "" {
			projects = append(projects, ProjectDeclaration{ManifestPath: path, Path: ref.Include, Name: ref.Include, Kind: "dotnet-reference", Confidence: "high"})
		}
	}
	return projects, nil
}
