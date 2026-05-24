/*
Plugin-Name: Authentication Analyzer
Plugin-Type: Enricher
Target-Format: All
Author: Gemini
Description: Parses Authentication-Results headers for SPF, DKIM, and DMARC status.
*/
package plugins

import (
	"Bibliorato/core"
	"bytes"
	"encoding/csv"
	"regexp"
	"strings"
)

type AuthAnalyzerPlugin struct{}

func init() {
	core.RegisterPlugin(&AuthAnalyzerPlugin{})
}

func (p *AuthAnalyzerPlugin) Name() string {
	return "Authentication Analyzer"
}

func (p *AuthAnalyzerPlugin) Type() string {
	return "Enricher"
}

func (p *AuthAnalyzerPlugin) TargetFormat() string {
	return "All"
}

func (p *AuthAnalyzerPlugin) Description() string {
	return "Parses Authentication-Results headers for SPF, DKIM, and DMARC status."
}

func (p *AuthAnalyzerPlugin) Process(evidence *core.Evidence) ([]core.Enrichment, error) {
	lines := strings.Split(evidence.Headers, "\n")
	var authHeaders []string
	currentHeader := ""
	isAuth := false

	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		// Check if it's the start of a new header
		if strings.Contains(lowerLine, ":") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			if isAuth && currentHeader != "" {
				authHeaders = append(authHeaders, currentHeader)
			}
			if strings.HasPrefix(lowerLine, "authentication-results:") {
				isAuth = true
				currentHeader = line
			} else {
				isAuth = false
				currentHeader = ""
			}
		} else if isAuth && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			// Continuation line for the current header
			currentHeader += " " + strings.TrimSpace(line)
		}
	}
	// Catch the last one if it was an auth header
	if isAuth && currentHeader != "" {
		authHeaders = append(authHeaders, currentHeader)
	}

	if len(authHeaders) == 0 {
		return nil, nil // No enrichments if no auth headers found
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write([]string{"Server", "SPF", "DKIM", "DMARC"})

	serverRe := regexp.MustCompile(`(?i)authentication-results:\s*([^;]+)`)
	spfRe := regexp.MustCompile(`(?i)\bspf=([a-zA-Z0-9]+)`)
	dkimRe := regexp.MustCompile(`(?i)\bdkim=([a-zA-Z0-9]+)`)
	dmarcRe := regexp.MustCompile(`(?i)\bdmarc=([a-zA-Z0-9]+)`)

	for _, header := range authHeaders {
		server := "Unknown"
		spf := "None"
		dkim := "None"
		dmarc := "None"

		if match := serverRe.FindStringSubmatch(header); len(match) > 1 {
			server = strings.TrimSpace(match[1])
		}
		if match := spfRe.FindStringSubmatch(header); len(match) > 1 {
			spf = match[1]
		}
		if match := dkimRe.FindStringSubmatch(header); len(match) > 1 {
			dkim = match[1]
		}
		if match := dmarcRe.FindStringSubmatch(header); len(match) > 1 {
			dmarc = match[1]
		}

		w.Write([]string{server, spf, dkim, dmarc})
	}
	w.Flush()

	enrichment := core.Enrichment{
		EvidenceID: evidence.ID,
		PluginName: p.Name(),
		Data:       buf.String(),
	}

	return []core.Enrichment{enrichment}, nil
}
