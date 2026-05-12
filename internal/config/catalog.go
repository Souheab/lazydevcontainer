package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const featureCatalogURL = "https://containers.dev/features"

var featureRefPattern = regexp.MustCompile("`(ghcr\\.io/[^`]+)`")

// Feature describes a Dev Container Feature displayed by the editor.
type Feature struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// LoadFeatureCatalog loads the online containers.dev feature index with a local cache fallback.
func LoadFeatureCatalog(ctx context.Context) ([]Feature, error) {
	return loadFeatureCatalog(ctx, http.DefaultClient, defaultFeatureCachePath())
}

// SearchFeatures returns catalog items that match every query term.
func SearchFeatures(features []Feature, query string) []Feature {
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		return features
	}
	result := make([]Feature, 0, len(features))
	for _, feature := range features {
		haystack := strings.ToLower(strings.Join([]string{feature.ID, feature.Name, feature.Description}, " "))
		matched := true
		for _, term := range terms {
			if !strings.Contains(haystack, term) {
				matched = false
				break
			}
		}
		if matched {
			result = append(result, feature)
		}
	}
	return result
}

func loadFeatureCatalog(ctx context.Context, client *http.Client, cachePath string) ([]Feature, error) {
	features, err := fetchFeatureCatalog(ctx, client)
	if err == nil && len(features) > 0 {
		_ = writeFeatureCache(cachePath, features)
		return features, nil
	}

	cached, cacheErr := readFeatureCache(cachePath)
	if cacheErr == nil && len(cached) > 0 {
		return cached, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, cacheErr
}

func fetchFeatureCatalog(ctx context.Context, client *http.Client) ([]Feature, error) {
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, featureCatalogURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("feature catalog returned %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	return parseFeatureCatalog(string(data)), nil
}

func parseFeatureCatalog(html string) []Feature {
	matches := featureRefPattern.FindAllStringSubmatch(html, -1)
	seen := map[string]bool{}
	features := make([]Feature, 0, len(matches))
	for _, match := range matches {
		id := strings.TrimSpace(match[1])
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		features = append(features, Feature{ID: id, Name: featureName(id)})
	}
	sort.Slice(features, func(i, j int) bool {
		return features[i].ID < features[j].ID
	})
	return features
}

func featureName(id string) string {
	last := id
	if slash := strings.LastIndex(last, "/"); slash >= 0 {
		last = last[slash+1:]
	}
	if colon := strings.LastIndex(last, ":"); colon >= 0 {
		last = last[:colon]
	}
	last = strings.ReplaceAll(last, "-", " ")
	last = strings.ReplaceAll(last, "_", " ")
	return strings.Title(last)
}

func defaultFeatureCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "lazydc", "features.json")
}

func readFeatureCache(path string) ([]Feature, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var features []Feature
	if err := json.Unmarshal(data, &features); err != nil {
		return nil, err
	}
	return features, nil
}

func writeFeatureCache(path string, features []Feature) error {
	data, err := json.MarshalIndent(features, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
