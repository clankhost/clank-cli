package initdetect

import (
	"bytes"
	"fmt"
	"text/template"
)

// templateData holds values injected into Dockerfile templates.
type templateData struct {
	Port     int
	BuildDir string
}

// GenerateDockerfile returns Dockerfile content for the detected project type.
func GenerateDockerfile(result *DetectionResult) (string, error) {
	var tmplStr string

	switch result.Type {
	case NodeSPA:
		tmplStr = nodeSPATemplate
	case NodeServer:
		tmplStr = nodeServerTemplate
	case PythonApp:
		tmplStr = pythonTemplate
	case StaticSite:
		tmplStr = staticSiteTemplate
	default:
		return "", fmt.Errorf("cannot generate Dockerfile for unknown project type")
	}

	tmpl, err := template.New("dockerfile").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	data := templateData{
		Port:     result.Port,
		BuildDir: result.BuildDir,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}
