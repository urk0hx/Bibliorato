package parser

import (
	"Bibliorato/core"
	"Bibliorato/database"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/emersion/go-message/mail"
	"github.com/jaytaylor/html2text"
)

func init() {
	RegisterFileParser(".eml", ParseEMLAndStore)
}

// ParseEMLAndStore parses an EML file, maps it to our Evidence struct,
// and inserts it into the Layered Integrity database.
func ParseEMLAndStore(filepath string) (*core.Evidence, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open EML: %w", err)
	}
	defer file.Close()

	m, err := mail.CreateReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse mail message: %w", err)
	}

	subject, err := m.Header.Subject()
	if err != nil {
		fmt.Printf("Warning: failed to parse subject: %v\n", err)
	}

	var fromStr string
	if fromList, err := m.Header.AddressList("From"); err == nil && len(fromList) > 0 {
		fromStr = fromList[0].Address
	}

	var toStr []string
	if toList, err := m.Header.AddressList("To"); err == nil {
		for _, t := range toList {
			toStr = append(toStr, t.Address)
		}
	}

	date, err := m.Header.Date()
	if err != nil {
		fmt.Printf("Warning: failed to parse date: %v\n", err)
	}

	var bodyText string
	var bodyHTML string
	type pendingAttachment struct {
		name string
		data []byte
	}
	var attachments []pendingAttachment

	for {
		p, err := m.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Printf("Warning: failed to read message part: %v\n", err)
			break
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, err := h.ContentType()
			if err != nil {
				continue
			}
			if strings.HasPrefix(contentType, "text/plain") {
				b, err := io.ReadAll(p.Body)
				if err == nil {
					bodyText = string(b)
				}
			} else if strings.HasPrefix(contentType, "text/html") {
				b, err := io.ReadAll(p.Body)
				if err == nil {
					bodyHTML = string(b)
				}
			}
		case *mail.AttachmentHeader:
			filename, err := h.Filename()
			if err == nil && filename != "" {
				data, err := io.ReadAll(p.Body)
				if err == nil {
					attachments = append(attachments, pendingAttachment{name: filename, data: data})
				}
			}
		}
	}

	if bodyText == "" && bodyHTML != "" {
		bodyText, err = html2text.FromString(bodyHTML, html2text.Options{PrettyTables: true})
		if err != nil {
			fmt.Printf("Warning: failed to convert HTML to text in EML: %v\n", err)
		}
	}

	var headersStr string
	var headerFields []string
	fields := m.Header.Fields()
	for fields.Next() {
		headerFields = append(headerFields, fmt.Sprintf("%s: %s", fields.Key(), fields.Value()))
	}
	headersStr = strings.Join(headerFields, "\n")

	evidence := &core.Evidence{
		Folder:   "Loaded Evidence",
		From:     fromStr,
		To:       strings.Join(toStr, ", "),
		Date:     core.FormatDate(date),
		Subject:  subject,
		BodyText: bodyText,
		BodyHTML: bodyHTML,
		Headers:  headersStr,
	}

	id, err := database.InsertEvidence(evidence)
	if err != nil {
		return nil, fmt.Errorf("failed to store EML evidence: %w", err)
	}

	for _, att := range attachments {
		size := int64(len(att.data))
		md5Hash := fmt.Sprintf("%x", md5.Sum(att.data))
		sha256Hash := fmt.Sprintf("%x", sha256.Sum256(att.data))
		database.InsertAttachment(&core.Attachment{
			EvidenceID: int(id),
			Filename:   att.name,
			Size:       size,
			MD5:        md5Hash,
			SHA256:     sha256Hash,
		}, att.data)
	}

	evidence.ID = int(id)
	return evidence, nil
}
