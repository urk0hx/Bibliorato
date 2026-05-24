package parser

// ParseOSTAndStore is an alias for ParsePSTAndStore because they use the same underlying format and library.
func ParseOSTAndStore(filepath string) (map[string][]string, string, error) {
	return ParsePSTAndStore(filepath)
}

func init() {
	RegisterStoreParser(".ost", ParseOSTAndStore)
}
