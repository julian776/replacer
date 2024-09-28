package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReplaceInLargeFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		search      string
		replace     string
		expected    string
		expectError bool
	}{
		{
			name:     "simple replacement",
			content:  "hello world",
			search:   "world",
			replace:  "gopher",
			expected: "hello gopher\n",
		},
		{
			name:     "multiple replacements",
			content:  "foo bar foo",
			search:   "foo",
			replace:  "baz",
			expected: "baz bar baz\n",
		},
		{
			name:     "no match",
			content:  "hello world",
			search:   "gopher",
			replace:  "world",
			expected: "hello world\n",
		},
		{
			name:     "empty content",
			content:  "",
			search:   "foo",
			replace:  "bar",
			expected: "",
		},
		{
			name:        "context timeout",
			content:     "hello world",
			search:      "world",
			replace:     "gopher",
			expected:    "hello world",
			expectError: true,
		},
		{
			name:        "large file",
			content:     buildLargeFile("a", 1024*1024),
			search:      "a",
			replace:     "b",
			expected:    buildLargeFile("b", 1024*1024),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file with the test content
			tempFile, err := os.CreateTemp("", "testfile")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tempFile.Name())

			if _, err := tempFile.WriteString(tt.content); err != nil {
				t.Fatalf("failed to write to temp file: %v", err)
			}
			tempFile.Close()

			// Set up context
			var ctx context.Context
			var cancel context.CancelFunc
			if tt.expectError {
				ctx, cancel = context.WithTimeout(context.Background(), 1*time.Nanosecond)
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), 1*time.Minute)
			}
			defer cancel()

			// Call the function
			err = replaceInLargeFile(ctx, tempFile.Name(), tt.search, tt.replace)

			// Check for expected errors
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected an error but got none")
				}
				return
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Read the file content back
			result, err := os.ReadFile(tempFile.Name())
			if err != nil {
				t.Fatalf("failed to read temp file: %v", err)
			}

			// Compare the result with the expected output
			if string(result) != tt.expected {
				t.Errorf("expected %q but got %q", tt.expected, string(result))
			}
		})
	}
}

func TestReplaceInFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		search      string
		replace     string
		expected    string
		expectError bool
	}{
		{
			name:     "simple replacement",
			content:  "hello world",
			search:   "world",
			replace:  "gopher",
			expected: "hello gopher",
		},
		{
			name:     "multiple replacements",
			content:  "foo bar foo",
			search:   "foo",
			replace:  "baz",
			expected: "baz bar baz",
		},
		{
			name:     "no match",
			content:  "hello world",
			search:   "gopher",
			replace:  "world",
			expected: "hello world",
		},
		{
			name:     "empty content",
			content:  "",
			search:   "foo",
			replace:  "bar",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file with the test content
			tempFile, err := os.CreateTemp("", "testfile")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tempFile.Name())

			if _, err := tempFile.WriteString(tt.content); err != nil {
				t.Fatalf("failed to write to temp file: %v", err)
			}
			tempFile.Close()

			// Call the function
			err = replaceInFile(tempFile.Name(), tt.search, tt.replace)

			// Check for unexpected errors
			if tt.expectError {
				if err == nil {
					t.Fatalf("expected an error but got none")
				}
				return
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Read the file content back
			result, err := os.ReadFile(tempFile.Name())
			if err != nil {
				t.Fatalf("failed to read temp file: %v", err)
			}

			// Compare the result with the expected output
			if strings.TrimSpace(string(result)) != tt.expected {
				t.Errorf("expected %q but got %q", tt.expected, string(result))
			}
		})
	}
}

func buildLargeFile(char string, size int) string {
	var sb strings.Builder
	skip := 50
	for i := 0; i < size; i += skip {
		sb.WriteString(strings.Repeat(char, skip))
		sb.WriteString("\n")
	}
	return sb.String()
}
