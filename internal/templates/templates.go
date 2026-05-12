package templates

import (
	"encoding/json"
	"sort"
	"strings"
)

// Template describes a built-in devcontainer template shown in the TUI.
type Template struct {
	ID                string
	Name              string
	Description       string
	Tags              []string
	Image             string
	Features          map[string]map[string]any
	Extensions        []string
	Settings          map[string]any
	PostCreateCommand string
}

type devcontainerJSON struct {
	Name              string          `json:"name"`
	Image             string          `json:"image"`
	Features          map[string]any  `json:"features,omitempty"`
	Customizations    *customizations `json:"customizations,omitempty"`
	RemoteUser        string          `json:"remoteUser,omitempty"`
	PostCreateCommand string          `json:"postCreateCommand,omitempty"`
}

type customizations struct {
	VSCode *vsCodeCustomizations `json:"vscode,omitempty"`
}

type vsCodeCustomizations struct {
	Extensions []string       `json:"extensions,omitempty"`
	Settings   map[string]any `json:"settings,omitempty"`
}

// Catalog returns the built-in template catalog in display order.
func Catalog() []Template {
	items := []Template{
		{
			ID:          "base-ubuntu",
			Name:        "Base Ubuntu",
			Description: "General-purpose Ubuntu devcontainer with common utilities.",
			Tags:        []string{"base", "ubuntu", "linux"},
			Image:       "mcr.microsoft.com/devcontainers/base:ubuntu-24.04",
		},
		{
			ID:          "go",
			Name:        "Go",
			Description: "Go toolchain with Go extension and language server settings.",
			Tags:        []string{"go", "golang", "backend"},
			Image:       "mcr.microsoft.com/devcontainers/base:ubuntu-24.04",
			Features: featureMap(map[string]map[string]any{
				"ghcr.io/devcontainers/features/go:1": {"version": "latest"},
			}),
			Extensions: []string{"golang.Go"},
			Settings: map[string]any{
				"go.useLanguageServer":               true,
				"go.toolsManagement.checkForUpdates": "local",
			},
			PostCreateCommand: "go version",
		},
		{
			ID:          "node-typescript",
			Name:        "Node/TypeScript",
			Description: "Node.js and TypeScript development with ESLint support.",
			Tags:        []string{"node", "typescript", "javascript", "web"},
			Image:       "mcr.microsoft.com/devcontainers/typescript-node:1-22-bookworm",
			Extensions:  []string{"dbaeumer.vscode-eslint", "esbenp.prettier-vscode"},
			Settings: map[string]any{
				"typescript.tsdk": "node_modules/typescript/lib",
			},
			PostCreateCommand: "node --version && npm --version",
		},
		{
			ID:          "python",
			Name:        "Python",
			Description: "Python development with the official Python and Pylance extensions.",
			Tags:        []string{"python", "data", "backend"},
			Image:       "mcr.microsoft.com/devcontainers/python:1-3.12-bookworm",
			Extensions:  []string{"ms-python.python", "ms-python.vscode-pylance"},
			Settings: map[string]any{
				"python.defaultInterpreterPath": "/usr/local/bin/python",
			},
			PostCreateCommand: "python --version",
		},
		{
			ID:                "rust",
			Name:              "Rust",
			Description:       "Rust development with rust-analyzer and LLDB debugging support.",
			Tags:              []string{"rust", "systems", "backend"},
			Image:             "mcr.microsoft.com/devcontainers/rust:1-1-bookworm",
			Extensions:        []string{"rust-lang.rust-analyzer", "vadimcn.vscode-lldb"},
			PostCreateCommand: "rustc --version && cargo --version",
		},
		{
			ID:                "java",
			Name:              "Java",
			Description:       "Java development with Maven and the VS Code Java extension pack.",
			Tags:              []string{"java", "jvm", "maven", "backend"},
			Image:             "mcr.microsoft.com/devcontainers/java:1-21-bookworm",
			Extensions:        []string{"vscjava.vscode-java-pack"},
			PostCreateCommand: "java -version",
		},
		{
			ID:                "dotnet",
			Name:              ".NET",
			Description:       ".NET SDK development with the C# extension.",
			Tags:              []string{"dotnet", "csharp", "backend"},
			Image:             "mcr.microsoft.com/devcontainers/dotnet:1-8.0-bookworm",
			Extensions:        []string{"ms-dotnettools.csharp"},
			PostCreateCommand: "dotnet --info",
		},
		{
			ID:                "php",
			Name:              "PHP",
			Description:       "PHP development with Composer and PHP tooling extensions.",
			Tags:              []string{"php", "composer", "web"},
			Image:             "mcr.microsoft.com/devcontainers/php:1-8.3-bookworm",
			Extensions:        []string{"bmewburn.vscode-intelephense-client", "xdebug.php-debug"},
			PostCreateCommand: "php --version && composer --version",
		},
		{
			ID:                "ruby",
			Name:              "Ruby",
			Description:       "Ruby development with Ruby LSP support.",
			Tags:              []string{"ruby", "rails", "backend"},
			Image:             "mcr.microsoft.com/devcontainers/ruby:1-3.3-bookworm",
			Extensions:        []string{"shopify.ruby-lsp"},
			PostCreateCommand: "ruby --version && bundle --version",
		},
		{
			ID:                "cpp",
			Name:              "C/C++",
			Description:       "C and C++ development with compiler, CMake, and debugger tooling.",
			Tags:              []string{"c", "cpp", "cplusplus", "cmake", "systems"},
			Image:             "mcr.microsoft.com/devcontainers/cpp:1-debian-12",
			Extensions:        []string{"ms-vscode.cpptools", "ms-vscode.cmake-tools"},
			PostCreateCommand: "gcc --version && cmake --version",
		},
		{
			ID:          "nix-base",
			Name:        "Nix/Base",
			Description: "Ubuntu base image with Nix flakes enabled.",
			Tags:        []string{"nix", "flakes", "base"},
			Image:       "mcr.microsoft.com/devcontainers/base:ubuntu-24.04",
			Features: featureMap(map[string]map[string]any{
				"ghcr.io/devcontainers/features/nix:1": {
					"version":        "latest",
					"extraNixConfig": "experimental-features = nix-command flakes",
				},
			}),
			PostCreateCommand: "nix --version",
		},
	}
	return cloneCatalog(items)
}

// Render returns the pretty-printed devcontainer.json for a template.
func (t Template) Render() (string, error) {
	data, err := t.JSON()
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

// JSON returns the pretty-printed devcontainer.json bytes for a template.
func (t Template) JSON() ([]byte, error) {
	config := devcontainerJSON{
		Name:              t.Name,
		Image:             t.Image,
		Features:          flattenFeatures(t.Features),
		RemoteUser:        "vscode",
		PostCreateCommand: t.PostCreateCommand,
	}
	if len(t.Extensions) > 0 || len(t.Settings) > 0 {
		config.Customizations = &customizations{
			VSCode: &vsCodeCustomizations{
				Extensions: append([]string(nil), t.Extensions...),
				Settings:   cloneAnyMap(t.Settings),
			},
		}
	}
	return json.MarshalIndent(config, "", "  ")
}

// Matches reports whether every query term matches the template metadata.
func (t Template) Matches(query string) bool {
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		return true
	}

	haystack := strings.ToLower(strings.Join([]string{
		t.ID,
		t.Name,
		t.Description,
		strings.Join(t.Tags, " "),
		t.Image,
	}, " "))
	for _, term := range terms {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}

// Filter returns catalog items matching query in display order.
func Filter(catalog []Template, query string) []Template {
	matches := make([]Template, 0, len(catalog))
	for _, item := range catalog {
		if item.Matches(query) {
			matches = append(matches, item)
		}
	}
	return matches
}

func featureMap(values map[string]map[string]any) map[string]map[string]any {
	return values
}

func flattenFeatures(features map[string]map[string]any) map[string]any {
	if len(features) == 0 {
		return nil
	}
	result := make(map[string]any, len(features))
	keys := make([]string, 0, len(features))
	for key := range features {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result[key] = cloneAnyMap(features[key])
	}
	return result
}

func cloneCatalog(items []Template) []Template {
	result := make([]Template, 0, len(items))
	for _, item := range items {
		item.Tags = append([]string(nil), item.Tags...)
		item.Extensions = append([]string(nil), item.Extensions...)
		item.Settings = cloneAnyMap(item.Settings)
		if len(item.Features) > 0 {
			item.Features = cloneFeatureMap(item.Features)
		}
		result = append(result, item)
	}
	return result
}

func cloneFeatureMap(values map[string]map[string]any) map[string]map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]map[string]any, len(values))
	for key, value := range values {
		result[key] = cloneAnyMap(value)
	}
	return result
}

func cloneAnyMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}
