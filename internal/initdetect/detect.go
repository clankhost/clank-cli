package initdetect

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ProjectType identifies the detected project type.
type ProjectType int

const (
	Unknown ProjectType = iota
	NodeSPA
	NodeServer
	PythonApp
	StaticSite
)

// String returns a human-readable name for the project type.
func (p ProjectType) String() string {
	switch p {
	case NodeSPA:
		return "Node.js SPA"
	case NodeServer:
		return "Node.js Server"
	case PythonApp:
		return "Python"
	case StaticSite:
		return "Static Site"
	default:
		return "Unknown"
	}
}

// DetectionResult holds the results of project type detection.
type DetectionResult struct {
	Type            ProjectType
	Name            string // from package.json "name" or directory name
	Port            int    // 80 for SPA/static, 8080 for servers
	HealthCheckPath string // "/" for SPA/static, "/health" for servers
	BuildDir        string // "dist", "build", "out" — for SPA only
	Framework       string // "vite", "next", "cra", "express", etc.
}

// packageJSON is a minimal representation of package.json.
type packageJSON struct {
	Name            string            `json:"name"`
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func (p *packageJSON) hasDep(name string) bool {
	if _, ok := p.Dependencies[name]; ok {
		return true
	}
	if _, ok := p.DevDependencies[name]; ok {
		return true
	}
	return false
}

// Detect inspects a directory and determines the project type.
// Mirrors the detection logic in apps/api/app/infrastructure/build/dockerfile_generator.py.
func Detect(dir string) *DetectionResult {
	// Default name from directory.
	name := filepath.Base(dir)

	// Try package.json first.
	pkgPath := filepath.Join(dir, "package.json")
	pkg := readPackageJSON(pkgPath)

	if pkg != nil {
		if pkg.Name != "" {
			name = pkg.Name
		}

		// Check for SPA frameworks (most specific match first).
		if result := detectNodeSPA(pkg, name); result != nil {
			return result
		}

		// Check for Node server (has "start" script).
		if _, ok := pkg.Scripts["start"]; ok {
			framework := "node"
			if pkg.hasDep("express") {
				framework = "express"
			} else if pkg.hasDep("fastify") {
				framework = "fastify"
			} else if pkg.hasDep("koa") {
				framework = "koa"
			}
			return &DetectionResult{
				Type:            NodeServer,
				Name:            name,
				Port:            8080,
				HealthCheckPath: "/health",
				Framework:       framework,
			}
		}
	}

	// Check for Python project.
	if fileExists(filepath.Join(dir, "requirements.txt")) || fileExists(filepath.Join(dir, "pyproject.toml")) {
		framework := "python"
		if fileExists(filepath.Join(dir, "requirements.txt")) {
			data, _ := os.ReadFile(filepath.Join(dir, "requirements.txt"))
			content := string(data)
			if contains(content, "fastapi") {
				framework = "fastapi"
			} else if contains(content, "flask") {
				framework = "flask"
			} else if contains(content, "django") {
				framework = "django"
			}
		}
		return &DetectionResult{
			Type:            PythonApp,
			Name:            name,
			Port:            8080,
			HealthCheckPath: "/health",
			Framework:       framework,
		}
	}

	// Check for static site (index.html in root).
	if fileExists(filepath.Join(dir, "index.html")) {
		return &DetectionResult{
			Type:            StaticSite,
			Name:            name,
			Port:            80,
			HealthCheckPath: "/",
			Framework:       "static",
		}
	}

	return &DetectionResult{
		Type:            Unknown,
		Name:            name,
		Port:            8080,
		HealthCheckPath: "/health",
	}
}

func detectNodeSPA(pkg *packageJSON, name string) *DetectionResult {
	spaMarkers := []string{"vite", "next", "@vitejs/plugin-react", "react-scripts", "@vue/cli-service", "nuxt"}
	isSPA := false
	for _, marker := range spaMarkers {
		if pkg.hasDep(marker) {
			isSPA = true
			break
		}
	}
	if !isSPA {
		return nil
	}

	// Must have a "build" script.
	if _, ok := pkg.Scripts["build"]; !ok {
		return nil
	}

	// Determine build output directory and framework.
	buildDir := "dist" // Vite, Vue CLI default
	framework := "vite"

	if pkg.hasDep("next") {
		buildDir = "out"
		framework = "next"
	} else if pkg.hasDep("react-scripts") {
		buildDir = "build"
		framework = "cra"
	} else if pkg.hasDep("@vue/cli-service") {
		framework = "vue-cli"
	} else if pkg.hasDep("nuxt") {
		framework = "nuxt"
	}

	return &DetectionResult{
		Type:            NodeSPA,
		Name:            name,
		Port:            80,
		HealthCheckPath: "/",
		BuildDir:        buildDir,
		Framework:       framework,
	}
}

func readPackageJSON(path string) *packageJSON {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	return &pkg
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (len(s) >= len(substr)) && (s != "" && substr != "") && (indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
