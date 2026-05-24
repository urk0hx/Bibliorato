package parser

import (
	"Bibliorato/core"
	"fmt"
	"path/filepath"
	"strings"
)

// StoreParser defines a function signature for parsing a mail store (e.g. PST, MBOX).
// It returns a tree map representing the folder structure, the root node string, or an error.
type StoreParser func(filepath string) (map[string][]string, string, error)

// FileParser defines a function signature for parsing a single mail file (e.g. EML, MSG).
// It returns the parsed Evidence object or an error.
type FileParser func(filepath string) (*core.Evidence, error)

var (
	storeParsers = make(map[string]StoreParser)
	fileParsers  = make(map[string]FileParser)
)

// RegisterStoreParser maps an extension (e.g., ".pst") to its StoreParser function.
func RegisterStoreParser(ext string, parser StoreParser) {
	storeParsers[strings.ToLower(ext)] = parser
}

// RegisterFileParser maps an extension (e.g., ".eml") to its FileParser function.
func RegisterFileParser(ext string, parser FileParser) {
	fileParsers[strings.ToLower(ext)] = parser
}

// GetSupportedStoreExtensions returns a slice of all registered mail store extensions.
func GetSupportedStoreExtensions() []string {
	exts := make([]string, 0, len(storeParsers))
	for ext := range storeParsers {
		exts = append(exts, ext)
	}
	return exts
}

// GetSupportedFileExtensions returns a slice of all registered single mail file extensions.
func GetSupportedFileExtensions() []string {
	exts := make([]string, 0, len(fileParsers))
	for ext := range fileParsers {
		exts = append(exts, ext)
	}
	return exts
}

// ProcessStore executes the registered parser for the given store file.
func ProcessStore(path string) (map[string][]string, string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	parser, exists := storeParsers[ext]
	if !exists {
		return nil, "", fmt.Errorf("no parser registered for store extension: %s", ext)
	}
	return parser(path)
}

// ProcessFile executes the registered parser for the given single mail file.
func ProcessFile(path string) (*core.Evidence, error) {
	ext := strings.ToLower(filepath.Ext(path))
	parser, exists := fileParsers[ext]
	if !exists {
		return nil, fmt.Errorf("no parser registered for mail file extension: %s", ext)
	}
	return parser(path)
}
