package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anaremore/clank/apps/cli/internal/api"
)

func TestParseEnvFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []api.EnvVarCreateRequest
	}{
		{
			name:    "simple key=value",
			content: "FOO=bar\nBAZ=qux\n",
			expected: []api.EnvVarCreateRequest{
				{Key: "FOO", Value: "bar"},
				{Key: "BAZ", Value: "qux"},
			},
		},
		{
			name:    "skip comments and blanks",
			content: "# This is a comment\n\nFOO=bar\n# Another comment\nBAZ=qux\n",
			expected: []api.EnvVarCreateRequest{
				{Key: "FOO", Value: "bar"},
				{Key: "BAZ", Value: "qux"},
			},
		},
		{
			name:    "strip double quotes",
			content: `DATABASE_URL="postgres://localhost/db"` + "\n",
			expected: []api.EnvVarCreateRequest{
				{Key: "DATABASE_URL", Value: "postgres://localhost/db"},
			},
		},
		{
			name:    "strip single quotes",
			content: "API_KEY='sk-123abc'\n",
			expected: []api.EnvVarCreateRequest{
				{Key: "API_KEY", Value: "sk-123abc"},
			},
		},
		{
			name:    "value with equals sign",
			content: "CONNECTION=host=localhost port=5432\n",
			expected: []api.EnvVarCreateRequest{
				{Key: "CONNECTION", Value: "host=localhost port=5432"},
			},
		},
		{
			name:     "empty file",
			content:  "",
			expected: nil,
		},
		{
			name:     "only comments",
			content:  "# comment 1\n# comment 2\n",
			expected: nil,
		},
		{
			name:    "whitespace around keys",
			content: "  FOO = bar  \n",
			expected: []api.EnvVarCreateRequest{
				{Key: "FOO", Value: " bar"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write temp .env file.
			dir := t.TempDir()
			path := filepath.Join(dir, ".env")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}

			vars, err := parseEnvFile(path)
			if err != nil {
				t.Fatalf("parseEnvFile() error = %v", err)
			}

			if len(vars) != len(tt.expected) {
				t.Fatalf("got %d vars, want %d", len(vars), len(tt.expected))
			}

			for i, v := range vars {
				if v.Key != tt.expected[i].Key {
					t.Errorf("var[%d].Key = %q, want %q", i, v.Key, tt.expected[i].Key)
				}
				if v.Value != tt.expected[i].Value {
					t.Errorf("var[%d].Value = %q, want %q", i, v.Value, tt.expected[i].Value)
				}
			}
		})
	}
}

func TestParseEnvFile_NotFound(t *testing.T) {
	_, err := parseEnvFile("/nonexistent/.env")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
