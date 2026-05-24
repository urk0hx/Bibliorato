/*
Plugin-Name: Trace Route Timeline
Plugin-Type: Enricher
Target-Format: All
Author: DFIR_Ninja
Description: Parses Received headers and displays a Hop Timeline Grid.
*/
package plugins

import (
	"Bibliorato/core"
	"bytes"
	"encoding/csv"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type TraceRoutePlugin struct{}

func init() {
	core.RegisterPlugin(&TraceRoutePlugin{})
}

func (p *TraceRoutePlugin) Name() string {
	return "Trace Route Timeline"
}

func (p *TraceRoutePlugin) Type() string {
	return "Enricher"
}

func (p *TraceRoutePlugin) TargetFormat() string {
	return "All"
}

func (p *TraceRoutePlugin) Description() string {
	return "Parses Received headers and displays a Hop Timeline Grid."
}

func (p *TraceRoutePlugin) Process(evidence *core.Evidence) ([]core.Enrichment, error) {
	// Simplified Received header parsing.
	lines := strings.Split(evidence.Headers, "\n")
	var received []string
	currentHeader := ""
	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		if strings.HasPrefix(lowerLine, "received:") {
			if currentHeader != "" {
				received = append(received, currentHeader)
			}
			currentHeader = line
		} else if (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) && currentHeader != "" {
			currentHeader += " " + strings.TrimSpace(line)
		} else {
			if currentHeader != "" && strings.HasPrefix(strings.ToLower(currentHeader), "received:") {
				received = append(received, currentHeader)
				currentHeader = ""
			}
		}
	}
	if currentHeader != "" && strings.HasPrefix(strings.ToLower(currentHeader), "received:") {
		received = append(received, currentHeader)
	}

	if len(received) == 0 {
		return nil, nil // No enrichments if no received headers
	}

	// We format the output as CSV to be easily rendered in a Table widget.
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write([]string{"Hop", "Server", "Time", "Delta"})

	var lastTime time.Time

	// Most email clients prepend newer headers at the top, so index 0 is newest.
	// We iterate backwards to start from the oldest hop.
	for i := len(received) - 1; i >= 0; i-- {
		hopNum := len(received) - i

		// Attempt to extract time (very basic regex)
		re := regexp.MustCompile(`;\s*([^;]*)$`)
		matches := re.FindStringSubmatch(received[i])
		var hopTime time.Time
		timeStr := "Unknown"
		if len(matches) > 1 {
			rawTime := strings.TrimSpace(matches[1])
			// Try various RFC formats
			formats := []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822}
			for _, f := range formats {
				parsed, err := time.Parse(f, rawTime)
				if err == nil {
					hopTime = parsed
					timeStr = core.FormatDate(hopTime)
					break
				}
			}
			if timeStr == "Unknown" {
				timeStr = rawTime
			}
		}

		// Attempt to extract server
		serverRe := regexp.MustCompile(`from\s+([^\s;]+)`)
		serverMatch := serverRe.FindStringSubmatch(received[i])
		server := "Unknown"
		if len(serverMatch) > 1 {
			server = strings.Trim(serverMatch[1], "()[]")
		}

		deltaStr := "-"
		if hopNum > 1 && !hopTime.IsZero() && !lastTime.IsZero() {
			delta := hopTime.Sub(lastTime)
			deltaStr = delta.String()
		}

		if !hopTime.IsZero() {
			lastTime = hopTime
		}

		w.Write([]string{fmt.Sprintf("%d", hopNum), server, timeStr, deltaStr})
	}
	w.Flush()

	enrichment := core.Enrichment{
		EvidenceID: evidence.ID,
		PluginName: p.Name(),
		Data:       buf.String(),
	}

	return []core.Enrichment{enrichment}, nil
}
