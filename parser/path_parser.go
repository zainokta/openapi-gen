package parser

import (
	"regexp"
	"strings"
)

// PathParser handles pure algorithmic path parsing with no manual mappings
type PathParser struct {
	commonPrefixes []string
	paramPattern   *regexp.Regexp
	versionPattern *regexp.Regexp
}

// NewPathParser creates a new path parser
func NewPathParser() *PathParser {
	return &PathParser{
		commonPrefixes: []string{"api", "v1", "v2", "v3", "v4"},
		paramPattern:   regexp.MustCompile(`:[^/]+`), // Matches :param patterns
		versionPattern: regexp.MustCompile(`^v\d+$`), // Matches version patterns like v1, v2
	}
}

// ParsedRoute contains pure algorithmic parsed route metadata
type ParsedRoute struct {
	Tag         string
	Summary     string
	Description string
	Segments    []string
	CleanPath   string
}

// ParseRoute parses a route using pure algorithm - no manual mappings
func (p *PathParser) ParseRoute(method, path string) ParsedRoute {
	segments := p.extractMeaningfulSegments(path)
	cleanPath := p.cleanPath(path)

	return ParsedRoute{
		Tag:         p.generateTag(segments),
		Summary:     p.generateSummary(method, segments),
		Description: p.generateDescription(method, segments),
		Segments:    segments,
		CleanPath:   cleanPath,
	}
}

// extractMeaningfulSegments extracts meaningful segments using pure algorithm
func (p *PathParser) extractMeaningfulSegments(path string) []string {
	// Remove leading/trailing slashes
	cleanPath := strings.Trim(path, "/")

	// Handle root path
	if cleanPath == "" {
		return []string{"root"}
	}

	// Split into segments
	allSegments := strings.Split(cleanPath, "/")
	var meaningful []string

	for _, segment := range allSegments {
		// Skip empty segments
		if segment == "" {
			continue
		}

		// Skip common prefixes
		if p.isCommonPrefix(segment) {
			continue
		}

		// Skip parameters (starting with :)
		if strings.HasPrefix(segment, ":") {
			continue
		}

		// Skip version patterns
		if p.versionPattern.MatchString(segment) {
			continue
		}

		meaningful = append(meaningful, segment)
	}

	// If no meaningful segments found, use the last segment
	if len(meaningful) == 0 && len(allSegments) > 0 {
		meaningful = []string{allSegments[len(allSegments)-1]}
	}

	return meaningful
}

// isCommonPrefix checks if a segment is a common prefix
func (p *PathParser) isCommonPrefix(segment string) bool {
	segmentLower := strings.ToLower(segment)
	for _, prefix := range p.commonPrefixes {
		if segmentLower == prefix {
			return true
		}
	}
	return false
}

// cleanPath returns a clean version of the path without parameters
func (p *PathParser) cleanPath(path string) string {
	// Remove parameters like :id, :token, etc. and replace with placeholder
	cleaned := p.paramPattern.ReplaceAllString(path, "{param}")
	return cleaned
}

// generateTag generates tag from segments using pure algorithm
func (p *PathParser) generateTag(segments []string) string {
	if len(segments) == 0 {
		return "system"
	}

	// Use the first meaningful segment as tag
	tag := strings.ToLower(segments[0])

	// Clean up the tag (remove hyphens, underscores)
	tag = strings.ReplaceAll(tag, "-", "")
	tag = strings.ReplaceAll(tag, "_", "")

	return tag
}

// generateSummary generates summary using pure algorithm
func (p *PathParser) generateSummary(method string, segments []string) string {
	// Get method action
	methodAction := p.getMethodAction(method)

	// Convert segments to title case
	var titleSegments []string
	for _, segment := range segments {
		titleSegments = append(titleSegments, p.toTitleCase(segment))
	}

	if len(titleSegments) == 0 {
		return methodAction + " Root"
	}

	// Join method action with segments
	return methodAction + " " + strings.Join(titleSegments, " ")
}

// generateDescription generates description using pure algorithm
func (p *PathParser) generateDescription(method string, segments []string) string {
	summary := p.generateSummary(method, segments)
	return summary + " operation"
}

// getMethodAction returns the action verb for HTTP methods
func (p *PathParser) getMethodAction(method string) string {
	switch strings.ToUpper(method) {
	case "GET":
		return "Get"
	case "POST":
		return "Create"
	case "PUT":
		return "Update"
	case "PATCH":
		return "Modify"
	case "DELETE":
		return "Delete"
	case "HEAD":
		return "Check"
	case "OPTIONS":
		return "Options"
	default:
		return p.toTitleCase(strings.ToLower(method))
	}
}

// toTitleCase converts string to title case with basic rules
func (p *PathParser) toTitleCase(s string) string {
	if len(s) == 0 {
		return s
	}

	// Handle hyphens and underscores
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")

	// Split by spaces and title case each word
	words := strings.Fields(s)
	var titleWords []string

	for _, word := range words {
		if len(word) > 0 {
			titleWords = append(titleWords, strings.ToUpper(string(word[0]))+strings.ToLower(word[1:]))
		}
	}

	return strings.Join(titleWords, " ")
}

// GenerateHandlerName generates handler name using pure algorithm
func (p *PathParser) GenerateHandlerName(method, path string) string {
	segments := p.extractMeaningfulSegments(path)

	var parts []string
	parts = append(parts, p.toTitleCase(strings.ToLower(method)))

	for _, segment := range segments {
		// Remove hyphens/underscores and title case
		cleanSegment := strings.ReplaceAll(segment, "-", "")
		cleanSegment = strings.ReplaceAll(cleanSegment, "_", "")
		parts = append(parts, p.toTitleCase(cleanSegment))
	}

	return strings.Join(parts, "")
}
