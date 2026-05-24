package core

import (
	"bufio"
	"os"
	"runtime"
	"strings"
)

// PluginMetadata holds the UI metadata parsed from the plugin's source file comment.
type PluginMetadata struct {
	Name         string
	Type         string
	TargetFormat string
	Author       string
	Description  string
}

// RegisteredPlugin links the executable plugin instance with its parsed metadata.
type RegisteredPlugin struct {
	Instance Plugin
	Metadata PluginMetadata
}

// PluginRegistry holds the registered plugins.
var PluginRegistry = make(map[string]*RegisteredPlugin)

// RegisterPlugin adds a plugin to the global registry and parses its metadata.
func RegisterPlugin(p Plugin) {
	meta := PluginMetadata{
		Name:         p.Name(),
		Type:         p.Type(),
		TargetFormat: p.TargetFormat(),
		Description:  p.Description(),
	}

	// Attempt to find the caller's file to parse the block comment
	_, file, _, ok := runtime.Caller(1)
	if ok {
		parsedMeta, err := parsePluginMetadata(file)
		if err == nil {
			// Override with parsed metadata if available
			if parsedMeta.Name != "" {
				meta.Name = parsedMeta.Name
			}
			if parsedMeta.Type != "" {
				meta.Type = parsedMeta.Type
			}
			if parsedMeta.TargetFormat != "" {
				meta.TargetFormat = parsedMeta.TargetFormat
			}
			if parsedMeta.Description != "" {
				meta.Description = parsedMeta.Description
			}
			if parsedMeta.Author != "" {
				meta.Author = parsedMeta.Author
			}
		}
	}

	PluginRegistry[p.Name()] = &RegisteredPlugin{
		Instance: p,
		Metadata: meta,
	}
}

// parsePluginMetadata reads the source file and extracts metadata from the block comment.
func parsePluginMetadata(filePath string) (PluginMetadata, error) {
	var meta PluginMetadata
	file, err := os.Open(filePath)
	if err != nil {
		return meta, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inBlock := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "/*") {
			inBlock = true
			continue
		}

		if inBlock {
			if strings.HasSuffix(line, "*/") {
				break
			}

			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])

				switch key {
				case "Plugin-Name":
					meta.Name = val
				case "Plugin-Type":
					meta.Type = val
				case "Target-Format":
					meta.TargetFormat = val
				case "Author":
					meta.Author = val
				case "Description":
					meta.Description = val
				}
			}
		} else if strings.HasPrefix(line, "package ") {
			// Stop if we hit the package declaration and haven't found the block
			break
		}
	}

	return meta, scanner.Err()
}
