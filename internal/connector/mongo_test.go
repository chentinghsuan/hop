package connector

import (
	"testing"
)

func TestMaskMongoURI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "URI with username and password",
			input:    "mongodb://admin:superSecret123@localhost:27017/my_db?authSource=admin",
			expected: "mongodb://admin:******@localhost:27017/my_db?authSource=admin",
		},
		{
			name:     "URI with username only",
			input:    "mongodb://admin@localhost:27017/my_db",
			expected: "mongodb://admin@localhost:27017/my_db",
		},
		{
			name:     "URI without credentials",
			input:    "mongodb://localhost:27017/my_db",
			expected: "mongodb://localhost:27017/my_db",
		},
		{
			name:     "Invalid URI format returns original",
			input:    "://invalid-uri",
			expected: "://invalid-uri",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masked := maskMongoURI(tt.input)
			if masked != tt.expected {
				t.Errorf("maskMongoURI(%q) = %q, expected %q", tt.input, masked, tt.expected)
			}
		})
	}
}

func TestMongoConnectorMetadata(t *testing.T) {
	c := NewMongoConnector()
	if c.Type() != "mongo" {
		t.Errorf("expected type 'mongo', got %q", c.Type())
	}
	aliases := c.Aliases()
	foundAtlas := false
	for _, a := range aliases {
		if a == "atlas" {
			foundAtlas = true
			break
		}
	}
	if !foundAtlas {
		t.Errorf("expected alias 'atlas' in MongoConnector aliases")
	}
}

