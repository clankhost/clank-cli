package initdetect

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectViteSPA(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"name": "my-vite-app",
		"scripts": {"build": "vite build", "dev": "vite"},
		"dependencies": {},
		"devDependencies": {"vite": "^5.0.0", "@vitejs/plugin-react": "^4.0.0"}
	}`)

	result := Detect(dir)
	if result.Type != NodeSPA {
		t.Errorf("expected NodeSPA, got %v", result.Type)
	}
	if result.BuildDir != "dist" {
		t.Errorf("expected BuildDir 'dist', got %q", result.BuildDir)
	}
	if result.Port != 80 {
		t.Errorf("expected port 80, got %d", result.Port)
	}
	if result.HealthCheckPath != "/" {
		t.Errorf("expected health check '/', got %q", result.HealthCheckPath)
	}
	if result.Name != "my-vite-app" {
		t.Errorf("expected name 'my-vite-app', got %q", result.Name)
	}
}

func TestDetectNextSPA(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"name": "next-app",
		"scripts": {"build": "next build"},
		"dependencies": {"next": "^14.0.0", "react": "^18.0.0"}
	}`)

	result := Detect(dir)
	if result.Type != NodeSPA {
		t.Errorf("expected NodeSPA, got %v", result.Type)
	}
	if result.BuildDir != "out" {
		t.Errorf("expected BuildDir 'out', got %q", result.BuildDir)
	}
	if result.Framework != "next" {
		t.Errorf("expected framework 'next', got %q", result.Framework)
	}
}

func TestDetectCRA(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"name": "cra-app",
		"scripts": {"build": "react-scripts build", "start": "react-scripts start"},
		"dependencies": {"react": "^18.0.0"},
		"devDependencies": {"react-scripts": "5.0.0"}
	}`)

	result := Detect(dir)
	if result.Type != NodeSPA {
		t.Errorf("expected NodeSPA, got %v", result.Type)
	}
	if result.BuildDir != "build" {
		t.Errorf("expected BuildDir 'build', got %q", result.BuildDir)
	}
	if result.Framework != "cra" {
		t.Errorf("expected framework 'cra', got %q", result.Framework)
	}
}

func TestDetectNodeServer(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"name": "express-api",
		"scripts": {"start": "node server.js"},
		"dependencies": {"express": "^4.0.0"}
	}`)

	result := Detect(dir)
	if result.Type != NodeServer {
		t.Errorf("expected NodeServer, got %v", result.Type)
	}
	if result.Port != 8080 {
		t.Errorf("expected port 8080, got %d", result.Port)
	}
	if result.HealthCheckPath != "/health" {
		t.Errorf("expected health check '/health', got %q", result.HealthCheckPath)
	}
	if result.Framework != "express" {
		t.Errorf("expected framework 'express', got %q", result.Framework)
	}
}

func TestDetectPython(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "flask==3.0.0\ngunicorn\n")

	result := Detect(dir)
	if result.Type != PythonApp {
		t.Errorf("expected PythonApp, got %v", result.Type)
	}
	if result.Port != 8080 {
		t.Errorf("expected port 8080, got %d", result.Port)
	}
	if result.Framework != "flask" {
		t.Errorf("expected framework 'flask', got %q", result.Framework)
	}
}

func TestDetectPythonPyproject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", `[project]
name = "myapp"
`)

	result := Detect(dir)
	if result.Type != PythonApp {
		t.Errorf("expected PythonApp, got %v", result.Type)
	}
}

func TestDetectStaticSite(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.html", "<html><body>Hello</body></html>")

	result := Detect(dir)
	if result.Type != StaticSite {
		t.Errorf("expected StaticSite, got %v", result.Type)
	}
	if result.Port != 80 {
		t.Errorf("expected port 80, got %d", result.Port)
	}
}

func TestDetectUnknown(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "README.md", "# Just a readme")

	result := Detect(dir)
	if result.Type != Unknown {
		t.Errorf("expected Unknown, got %v", result.Type)
	}
}

func TestGenerateDockerfileViteSPA(t *testing.T) {
	result := &DetectionResult{
		Type:     NodeSPA,
		BuildDir: "dist",
		Port:     80,
	}
	content, err := GenerateDockerfile(result)
	if err != nil {
		t.Fatalf("GenerateDockerfile failed: %v", err)
	}
	if !contains(content, "nginx:alpine") {
		t.Error("expected nginx in SPA Dockerfile")
	}
	if !contains(content, "/app/dist") {
		t.Error("expected /app/dist in SPA Dockerfile")
	}
}

func TestGenerateDockerfileNodeServer(t *testing.T) {
	result := &DetectionResult{
		Type: NodeServer,
		Port: 8080,
	}
	content, err := GenerateDockerfile(result)
	if err != nil {
		t.Fatalf("GenerateDockerfile failed: %v", err)
	}
	if !contains(content, "npm start") {
		t.Error("expected 'npm start' in Node server Dockerfile")
	}
	if !contains(content, "EXPOSE 8080") {
		t.Error("expected 'EXPOSE 8080'")
	}
}

func TestGenerateDockerfilePython(t *testing.T) {
	result := &DetectionResult{
		Type: PythonApp,
		Port: 8080,
	}
	content, err := GenerateDockerfile(result)
	if err != nil {
		t.Fatalf("GenerateDockerfile failed: %v", err)
	}
	if !contains(content, "python:3.12-slim") {
		t.Error("expected python:3.12-slim in Python Dockerfile")
	}
	if !contains(content, "gunicorn") {
		t.Error("expected gunicorn in Python Dockerfile")
	}
}

func TestGenerateDockerfileUnknownFails(t *testing.T) {
	result := &DetectionResult{Type: Unknown}
	_, err := GenerateDockerfile(result)
	if err == nil {
		t.Error("expected error for Unknown type")
	}
}
