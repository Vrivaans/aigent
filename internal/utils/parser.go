package utils

import (
	"regexp"
	"strings"
)

type ExtractedArtifact struct {
	ID      string
	Type    string
	Format  string
	Title   string
	Content string
}

// ExtractArtifacts parses <artifact ...>...</artifact> tags from input string.
// Returns the input text without those tags, and a slice of extracted artifacts.
func ExtractArtifacts(input string) (string, []ExtractedArtifact) {
	// Match any <artifact ...> ... </artifact>
	re := regexp.MustCompile(`(?s)<artifact\s+([^>]+?)>(.*?)</artifact>`)
	matches := re.FindAllStringSubmatch(input, -1)

	var artifacts []ExtractedArtifact
	cleanText := input

	// Regex for attributes
	attrRe := regexp.MustCompile(`([a-zA-Z0-9_-]+)="([^"]*)"`)

	for _, match := range matches {
		if len(match) >= 3 {
			fullTag := match[0]
			attrStr := match[1]
			content := strings.TrimSpace(match[2])

			// Extract attributes
			attrs := make(map[string]string)
			attrMatches := attrRe.FindAllStringSubmatch(attrStr, -1)
			for _, am := range attrMatches {
				if len(am) >= 3 {
					attrs[am[1]] = am[2]
				}
			}

			// Add to list
			artifacts = append(artifacts, ExtractedArtifact{
				ID:      attrs["id"],
				Type:    attrs["type"],
				Format:  attrs["format"],
				Title:   attrs["title"],
				Content: content,
			})

			// Remove tag from clean text
			cleanText = strings.Replace(cleanText, fullTag, "", 1)
		}
	}

	return strings.TrimSpace(cleanText), artifacts
}
