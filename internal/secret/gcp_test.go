package secret

import (
	"testing"
)

func TestExtractJSONField(t *testing.T) {
	jsonData := `{"username": "dbuser", "password": "superSecretPassword123", "port": 3306}`

	// 1. Extract password field
	val, err := extractJSONField(jsonData, "password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "superSecretPassword123" {
		t.Fatalf("expected 'superSecretPassword123', got %q", val)
	}

	// 2. Extract username field
	val, err = extractJSONField(jsonData, "username")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "dbuser" {
		t.Fatalf("expected 'dbuser', got %q", val)
	}

	// 3. Extract non-existent field
	_, err = extractJSONField(jsonData, "non_existent")
	if err == nil {
		t.Fatalf("expected error for non-existent field, got nil")
	}

	// 4. Empty field returns raw
	val, err = extractJSONField("raw-text-secret", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "raw-text-secret" {
		t.Fatalf("expected 'raw-text-secret', got %q", val)
	}
}

func TestBuildSecretResourceName(t *testing.T) {
	tests := []struct {
		name     string
		item     string
		version  string
		project  string
		expected string
	}{
		{
			name:     "Simple secret name with project",
			item:     "my-db-pass",
			version:  "latest",
			project:  "my-prj",
			expected: "projects/my-prj/secrets/my-db-pass/versions/latest",
		},
		{
			name:     "Already full resource name without version",
			item:     "projects/my-prj/secrets/my-db-pass",
			version:  "2",
			project:  "",
			expected: "projects/my-prj/secrets/my-db-pass/versions/2",
		},
		{
			name:     "Already full resource name with version",
			item:     "projects/my-prj/secrets/my-db-pass/versions/3",
			version:  "latest",
			project:  "",
			expected: "projects/my-prj/secrets/my-db-pass/versions/3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := buildSecretResourceName(tt.item, tt.version, tt.project)
			if res != tt.expected {
				t.Fatalf("buildSecretResourceName() = %q, expected %q", res, tt.expected)
			}
		})
	}
}
