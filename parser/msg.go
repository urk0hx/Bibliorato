package parser

import (
	"Bibliorato/core"
	"Bibliorato/database"
	"fmt"

	"github.com/jaytaylor/html2text"
	msgparser "github.com/willthrom/outlook-msg-parser"
)

func init() {
	RegisterFileParser(".msg", ParseMSGAndStore)
}

// ParseMSGAndStore parses an Outlook .msg file, maps it to our Evidence struct,
// and inserts it into the Layered Integrity database.
func ParseMSGAndStore(filepath string) (*core.Evidence, error) {
	// Parse the MSG file
	msg, err := msgparser.ParseMsgFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MSG: %w", err)
	}

	// Prefer FromEmail, fallback to FromName
	from := msg.FromEmail
	if from == "" {
		from = msg.FromName
	}

	// Prefer TransportMessageHeaders, fallback to Headers
	headers := msg.TransportMessageHeaders
	if headers == "" {
		headers = msg.Headers
	}

	bodyText := msg.BodyPlainText
	if bodyText == "" && msg.BodyHTML != "" {
		var err error
		bodyText, err = html2text.FromString(msg.BodyHTML, html2text.Options{PrettyTables: true})
		if err != nil {
			fmt.Printf("Warning: failed to convert HTML to text in MSG: %v\n", err)
		}
	}

	evidence := &core.Evidence{
		Folder:   "Loaded Evidence",
		From:     from,
		To:       msg.ToDisplay,
		Date:     core.FormatDate(msg.Date),
		Subject:  msg.Subject,
		BodyText: bodyText,
		BodyHTML: msg.BodyHTML,
		Headers:  headers,
	}

	// Store in SQLite database
	id, err := database.InsertEvidence(evidence)
	if err != nil {
		return nil, fmt.Errorf("failed to store MSG evidence: %w", err)
	}

	// Extract attachment names
	for _, att := range msg.Attachments {
		if att.Name != "" {
			// msgparser does not easily expose binary data, using empty data for MSG for now.
			data := []byte{}
			database.InsertAttachment(&core.Attachment{
				EvidenceID: int(id),
				Filename:   att.Name,
				Size:       0,
				MD5:        "d41d8cd98f00b204e9800998ecf8427e",                                 // md5 of empty string
				SHA256:     "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // sha256 of empty string
			}, data)
		}
	}

	// Update the Evidence struct with the generated database ID
	evidence.ID = int(id)
	return evidence, nil
}
