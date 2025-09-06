package openapi

import (
	"github.com/zainokta/openapi-gen/parser"
	"regexp"
	"strings"
)

// RouteMetadata represents custom metadata for routes
type RouteMetadata struct {
	Tags        string `json:"tags,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Description string `json:"description,omitempty"`
}

// OverrideManager manages custom metadata overrides
type OverrideManager struct {
	pathOverrides    map[string]RouteMetadata // Exact path matches
	tagOverrides     map[string][]string      // Tag-level overrides
	patternOverrides []PatternOverride        // Pattern-based overrides
}

// PatternOverride represents a pattern-based override
type PatternOverride struct {
	Pattern     string
	Metadata    RouteMetadata
	CompiledReg *regexp.Regexp
}

// NewOverrideManager creates a new override manager
func NewOverrideManager() *OverrideManager {
	return &OverrideManager{
		pathOverrides:    make(map[string]RouteMetadata),
		tagOverrides:     make(map[string][]string),
		patternOverrides: make([]PatternOverride, 0),
	}
}

// Override sets custom metadata for a specific path
func (om *OverrideManager) Override(method, path string, metadata RouteMetadata) {
	key := om.createPathKey(method, path)
	om.pathOverrides[key] = metadata
}

// OverrideTags sets custom tag for a specific tag
func (om *OverrideManager) OverrideTags(originalTag string, newTag string) {
	om.tagOverrides[originalTag] = []string{newTag}
}

// OverridePattern sets custom metadata for paths matching a pattern
func (om *OverrideManager) OverridePattern(pattern string, metadata RouteMetadata) error {
	// Convert pattern to regex
	// Support patterns like: "*/login", "/api/*/auth", "GET /api/*/login"
	regexPattern := om.convertPatternToRegex(pattern)

	compiledReg, err := regexp.Compile(regexPattern)
	if err != nil {
		return err
	}

	override := PatternOverride{
		Pattern:     pattern,
		Metadata:    metadata,
		CompiledReg: compiledReg,
	}

	om.patternOverrides = append(om.patternOverrides, override)
	return nil
}

// GetMetadata retrieves metadata with override precedence: Path > Pattern > Algorithm
func (om *OverrideManager) GetMetadata(method, path string, algorithmicMetadata parser.ParsedRoute) RouteMetadata {
	result := RouteMetadata{
		Tags:        algorithmicMetadata.Tag,
		Summary:     algorithmicMetadata.Summary,
		Description: algorithmicMetadata.Description,
	}

	// 1. Check for pattern-based overrides first (most flexible)
	if patternMetadata := om.getPatternMetadata(method, path); patternMetadata != nil {
		om.mergeMetadata(&result, *patternMetadata)
	}

	// 2. Check for exact path overrides (highest priority)
	key := om.createPathKey(method, path)
	if pathMetadata, exists := om.pathOverrides[key]; exists {
		om.mergeMetadata(&result, pathMetadata)
	}

	// 3. Apply tag-level overrides
	if newTags, exists := om.tagOverrides[algorithmicMetadata.Tag]; exists {
		if len(newTags) > 0 {
			result.Tags = newTags[0]
		}
	}

	return result
}

// getPatternMetadata checks if any pattern matches the given method and path
func (om *OverrideManager) getPatternMetadata(method, path string) *RouteMetadata {
	searchString := method + " " + path

	// Check patterns in order of registration
	for _, override := range om.patternOverrides {
		if override.CompiledReg.MatchString(searchString) || override.CompiledReg.MatchString(path) {
			return &override.Metadata
		}
	}

	return nil
}

// mergeMetadata merges override metadata into result (non-empty values override)
func (om *OverrideManager) mergeMetadata(result *RouteMetadata, override RouteMetadata) {
	if len(override.Tags) > 0 {
		result.Tags = override.Tags
	}
	if override.Summary != "" {
		result.Summary = override.Summary
	}
	if override.Description != "" {
		result.Description = override.Description
	}
}

// createPathKey creates a unique key for method+path combination
func (om *OverrideManager) createPathKey(method, path string) string {
	return strings.ToUpper(method) + " " + path
}

// convertPatternToRegex converts simple patterns to regex
func (om *OverrideManager) convertPatternToRegex(pattern string) string {
	// Handle method prefixes
	if strings.Contains(pattern, " ") {
		parts := strings.SplitN(pattern, " ", 2)
		method := parts[0]
		pathPattern := parts[1]
		pathRegex := om.pathPatternToRegex(pathPattern)
		// Remove the ^ and $ anchors from path regex since we'll add them for the full pattern
		pathRegex = strings.TrimPrefix(pathRegex, "^")
		pathRegex = strings.TrimSuffix(pathRegex, "$")
		return "^" + strings.ToUpper(method) + " " + pathRegex + "$"
	}

	// Just path pattern
	return om.pathPatternToRegex(pattern)
}

// pathPatternToRegex converts path patterns to regex
func (om *OverrideManager) pathPatternToRegex(pathPattern string) string {
	// Manually escape special regex characters except *
	result := ""
	for _, char := range pathPattern {
		switch char {
		case '*':
			result += ".*" // Keep * as wildcard
		case '.', '^', '$', '(', ')', '[', ']', '{', '}', '|', '+', '?', '\\':
			result += "\\" + string(char) // Escape special regex chars
		default:
			result += string(char)
		}
	}

	// Add anchors for exact matching
	return "^" + result + "$"
}

// GetOverrideStats returns statistics about current overrides
func (om *OverrideManager) GetOverrideStats() map[string]int {
	return map[string]int{
		"path_overrides":    len(om.pathOverrides),
		"tag_overrides":     len(om.tagOverrides),
		"pattern_overrides": len(om.patternOverrides),
	}
}

// ListOverrides returns all current overrides for debugging
func (om *OverrideManager) ListOverrides() map[string]interface{} {
	return map[string]interface{}{
		"paths":    om.pathOverrides,
		"tags":     om.tagOverrides,
		"patterns": om.extractPatternStrings(),
	}
}

// extractPatternStrings extracts pattern strings for debugging
func (om *OverrideManager) extractPatternStrings() []string {
	var patterns []string
	for _, override := range om.patternOverrides {
		patterns = append(patterns, override.Pattern)
	}
	return patterns
}
