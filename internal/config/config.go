package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ConfigCandidate is a devcontainer.json location supported by the Dev Container spec.
type ConfigCandidate struct {
	Path   string
	Exists bool
}

// ConfigDocument is the editable devcontainer.json document plus preserved fields.
type ConfigDocument struct {
	Path              string
	Name              string
	Image             string
	Features          map[string]map[string]any
	RemoteUser        string
	Extensions        []string
	Settings          map[string]any
	PostCreateCommand string
	Raw               map[string]any
}

// Discover returns existing config files in spec precedence order, or the default target when none exist.
func Discover(root string) ([]ConfigCandidate, error) {
	if strings.TrimSpace(root) == "" {
		root = "."
	}

	candidates := []string{
		filepath.Join(root, ".devcontainer", "devcontainer.json"),
		filepath.Join(root, ".devcontainer.json"),
	}

	nestedRoot := filepath.Join(root, ".devcontainer")
	entries, err := os.ReadDir(nestedRoot)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		var nested []string
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			nested = append(nested, filepath.Join(nestedRoot, entry.Name(), "devcontainer.json"))
		}
		sort.Strings(nested)
		candidates = append(candidates, nested...)
	}

	var found []ConfigCandidate
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		switch {
		case err == nil && !info.IsDir():
			found = append(found, ConfigCandidate{Path: candidate, Exists: true})
		case err == nil:
			continue
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return nil, err
		}
	}

	if len(found) > 0 {
		return found, nil
	}
	return []ConfigCandidate{{Path: candidates[0]}}, nil
}

// Load reads a config document. Missing files produce an empty editable document at path.
func Load(path string) (ConfigDocument, error) {
	doc := ConfigDocument{
		Path:     path,
		Features: map[string]map[string]any{},
		Settings: map[string]any{},
		Raw:      map[string]any{},
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return doc, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return doc, nil
	}
	if err := json.Unmarshal(data, &doc.Raw); err != nil {
		return doc, fmt.Errorf("parse devcontainer.json: %w", err)
	}

	doc.Name, _ = doc.Raw["name"].(string)
	doc.Image, _ = doc.Raw["image"].(string)
	doc.RemoteUser, _ = doc.Raw["remoteUser"].(string)
	doc.PostCreateCommand, _ = doc.Raw["postCreateCommand"].(string)
	doc.Features = readFeatures(doc.Raw["features"])
	doc.Extensions, doc.Settings = readVSCode(doc.Raw["customizations"])
	return doc, nil
}

// Save writes a config document as pretty JSON.
func Save(doc ConfigDocument) error {
	if strings.TrimSpace(doc.Path) == "" {
		return errors.New("missing devcontainer path")
	}
	data, err := doc.JSON()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(doc.Path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(doc.Path, data, 0o644)
}

// JSON returns the pretty-printed devcontainer.json bytes for the document.
func (doc ConfigDocument) JSON() ([]byte, error) {
	out := cloneAnyMap(doc.Raw)
	setString(out, "name", doc.Name)
	setString(out, "image", doc.Image)
	setString(out, "remoteUser", doc.RemoteUser)
	setString(out, "postCreateCommand", doc.PostCreateCommand)

	if len(doc.Features) == 0 {
		delete(out, "features")
	} else {
		out["features"] = cloneFeatureMap(doc.Features)
	}

	applyVSCode(out, doc.Extensions, doc.Settings)
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func setString(values map[string]any, key string, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		delete(values, key)
		return
	}
	values[key] = value
}

func readFeatures(value any) map[string]map[string]any {
	result := map[string]map[string]any{}
	values, ok := value.(map[string]any)
	if !ok {
		return result
	}
	for id, rawOptions := range values {
		switch options := rawOptions.(type) {
		case map[string]any:
			result[id] = cloneAnyMap(options)
		case nil:
			result[id] = map[string]any{}
		default:
			result[id] = map[string]any{"version": options}
		}
	}
	return result
}

func readVSCode(value any) ([]string, map[string]any) {
	customizations, ok := value.(map[string]any)
	if !ok {
		return nil, map[string]any{}
	}
	vscode, ok := customizations["vscode"].(map[string]any)
	if !ok {
		return nil, map[string]any{}
	}

	extensions := []string{}
	for _, item := range anySlice(vscode["extensions"]) {
		if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
			extensions = append(extensions, value)
		}
	}
	settings, _ := vscode["settings"].(map[string]any)
	return extensions, cloneAnyMap(settings)
}

func applyVSCode(out map[string]any, extensions []string, settings map[string]any) {
	rawCustomizations, _ := out["customizations"].(map[string]any)
	customizations := cloneAnyMap(rawCustomizations)
	rawVSCode, _ := customizations["vscode"].(map[string]any)
	vscode := cloneAnyMap(rawVSCode)

	cleanExtensions := cleanStrings(extensions)
	if len(cleanExtensions) == 0 {
		delete(vscode, "extensions")
	} else {
		values := make([]any, 0, len(cleanExtensions))
		for _, extension := range cleanExtensions {
			values = append(values, extension)
		}
		vscode["extensions"] = values
	}

	if len(settings) == 0 {
		delete(vscode, "settings")
	} else {
		vscode["settings"] = cloneAnyMap(settings)
	}

	if len(vscode) == 0 {
		delete(customizations, "vscode")
	} else {
		customizations["vscode"] = vscode
	}
	if len(customizations) == 0 {
		delete(out, "customizations")
	} else {
		out["customizations"] = customizations
	}
}

func anySlice(value any) []any {
	switch values := value.(type) {
	case []any:
		return values
	default:
		return nil
	}
}

func cleanStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func cloneAnyMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func cloneFeatureMap(values map[string]map[string]any) map[string]any {
	result := make(map[string]any, len(values))
	for key, options := range values {
		result[key] = cloneAnyMap(options)
	}
	return result
}
