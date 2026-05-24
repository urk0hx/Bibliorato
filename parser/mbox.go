package parser

import (
	"Bibliorato/core"
	"Bibliorato/database"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/emersion/go-mbox"
	"github.com/emersion/go-message/mail"
	"github.com/jaytaylor/html2text"
)

func init() {
	RegisterStoreParser(".mbox", ParseMBOXAndStore)
}

// ParseMBOXAndStore parses an MBOX file, storing its contents into the DB.
// It returns a tree mapping for the UI representing the MBOX file as a folder.
func ParseMBOXAndStore(path string) (map[string][]string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open MBOX: %w", err)
	}
	defer file.Close()

	// Using the base filename as the single folder for this MBOX
	filename := filepath.Base(path)
	treeData := make(map[string][]string)
	treeData[""] = []string{filename}
	treeData[filename] = []string{}

	mr := mbox.NewReader(file)

	for {
		msgReader, err := mr.NextMessage()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("Warning: failed to read MBOX message: %v\n", err)
			continue
		}

		m, err := mail.CreateReader(msgReader)
		if err != nil {
			fmt.Printf("Warning: failed to parse mail message: %v\n", err)
			continue
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

		// Read the body
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
				fmt.Printf("Warning: failed to convert HTML to text: %v\n", err)
			}
		}

		// Also extract raw headers by reading the original msgReader?
		// go-message gives structured headers, but we might want the raw ones.
		// For now we'll reconstruct them or leave blank, or try to get them via Fields().
		var headersStr string
		var headerFields []string
		fields := m.Header.Fields()
		for fields.Next() {
			headerFields = append(headerFields, fmt.Sprintf("%s: %s", fields.Key(), fields.Value()))
		}
		headersStr = strings.Join(headerFields, "\n")

		evidence := &core.Evidence{
			Folder:   filename,
			From:     fromStr,
			To:       strings.Join(toStr, ", "),
			Date:     core.FormatDate(date),
			Subject:  subject,
			BodyText: bodyText,
			BodyHTML: bodyHTML,
			Headers:  headersStr,
		}

		if id, err := database.InsertEvidence(evidence); err != nil {
			fmt.Printf("Error storing message: %v\n", err)
		} else {
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
		}
	}

	return treeData, filename, nil
}
