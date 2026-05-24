/*
Plugin-Name: Link Mismatch Analyzer
Plugin-Type: Enricher
Target-Format: All
Author: Gemini
Description: Scans HTML bodies for links where the display text is a URL that differs from the actual href destination.
*/
package plugins

import (
	"Bibliorato/core"
	"bytes"
	"encoding/csv"
	"net/url"
	"regexp"
	"strings"
)

type LinkMismatchPlugin struct{}

func init() {
	core.RegisterPlugin(&LinkMismatchPlugin{})
}

func (p *LinkMismatchPlugin) Name() string {
	return "Link Mismatch Analyzer"
}

func (p *LinkMismatchPlugin) Type() string {
	return "Enricher"
}

func (p *LinkMismatchPlugin) TargetFormat() string {
	return "All"
}

func (p *LinkMismatchPlugin) Description() string {
	return "Scans HTML bodies for deceptive links where the display text is a URL that differs from the actual destination."
}

func (p *LinkMismatchPlugin) Process(evidence *core.Evidence) ([]core.Enrichment, error) {
	if evidence.BodyHTML == "" {
		return nil, nil
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write([]string{"Displayed URL", "Actual Destination"})

	// Regex to extract href and inner text from anchor tags
	// (?is) makes it case-insensitive and allows dot to match newlines
	re := regexp.MustCompile(`(?is)<a\s+[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	matches := re.FindAllStringSubmatch(evidence.BodyHTML, -1)

	// Regex to strip inner HTML tags from the display text
	tagStripper := regexp.MustCompile(`(?is)<[^>]+>`)
	mismatchCount := 0

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		href := strings.TrimSpace(match[1])
		rawText := match[2]

		// Strip any nested tags (e.g., <a><span>http://example.com</span></a>)
		cleanText := tagStripper.ReplaceAllString(rawText, "")
		cleanText = strings.TrimSpace(cleanText)

		// If the text is empty or too long, skip it to avoid noise
		if cleanText == "" || len(cleanText) > 2048 {
			continue
		}

		// Only check if the display text appears to be a URL
		lowerText := strings.ToLower(cleanText)
		if strings.HasPrefix(lowerText, "http://") || strings.HasPrefix(lowerText, "https://") || strings.HasPrefix(lowerText, "www.") {

			// Prepare strings for URL parsing
			parseableText := cleanText
			if strings.HasPrefix(lowerText, "www.") {
				parseableText = "http://" + cleanText // default scheme for parsing
			}

			// Some hrefs might be missing the scheme if they are malformed in phishing, but usually they have http/https.
			parseableHref := href
			if strings.HasPrefix(strings.ToLower(href), "www.") {
				parseableHref = "http://" + href
			}

			parsedTextURL, errText := url.Parse(parseableText)
			parsedHrefURL, errHref := url.Parse(parseableHref)

			if errText == nil && errHref == nil {
				// Unwrap Microsoft Safelinks
				if strings.HasSuffix(strings.ToLower(parsedHrefURL.Hostname()), "safelinks.protection.outlook.com") {
					if safelinkTarget := parsedHrefURL.Query().Get("url"); safelinkTarget != "" {
						if unwrapURL, err := url.Parse(safelinkTarget); err == nil {
							parsedHrefURL = unwrapURL
							href = safelinkTarget // Update the output to show the actual destination
						}
					}
				}

				// Normalize hostnames for comparison
				textHost := strings.TrimPrefix(parsedTextURL.Hostname(), "www.")
				hrefHost := strings.TrimPrefix(parsedHrefURL.Hostname(), "www.")

				// If both resolved a hostname and they don't match, flag it
				if textHost != "" && hrefHost != "" && textHost != hrefHost {
					w.Write([]string{cleanText, href})
					mismatchCount++
				}
			}
		}
	}

	if mismatchCount == 0 {
		return nil, nil // No enrichments if no deceptive links found
	}

	w.Flush()

	enrichment := core.Enrichment{
		EvidenceID: evidence.ID,
		PluginName: p.Name(),
		Data:       buf.String(),
	}

	return []core.Enrichment{enrichment}, nil
}
