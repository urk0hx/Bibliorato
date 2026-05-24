package parser

import (
	"Bibliorato/core"
	"Bibliorato/database"
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"os"
	"time"

	"github.com/jaytaylor/html2text"
	pst "github.com/mooijtech/go-pst/v6/pkg"
	"github.com/mooijtech/go-pst/v6/pkg/properties"
	"github.com/rotisserie/eris"
)

func init() {
	RegisterStoreParser(".pst", ParsePSTAndStore)
}

// ParsePSTAndStore parses a PST file, extracting folders and messages.
// It returns a mapping of the folder hierarchy (parent name -> child names) for the UI tree,
// and the root node name.
func ParsePSTAndStore(filepath string) (map[string][]string, string, error) {
	treeData := make(map[string][]string)

	reader, err := os.Open(filepath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open PST file: %w", err)
	}

	pstFile, err := pst.New(reader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to initialize PST parser: %w", err)
	}

	// Make sure we close and cleanup the PST reader when done
	defer func() {
		pstFile.Cleanup()
		reader.Close()
	}()

	rootFolder, err := pstFile.GetRootFolder()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get root folder: %w", err)
	}

	rootName := rootFolder.Name
	if rootName == "" {
		rootName = "Root"
	}

	// We establish the tree root
	treeData[""] = []string{rootName}
	treeData[rootName] = []string{}

	// Recursive walk to build the tree data and extract messages
	var walk func(folder *pst.Folder, parentName string) error
	walk = func(folder *pst.Folder, parentName string) error {
		currentName := folder.Name
		if currentName == "" {
			currentName = "Unnamed Folder"
		}

		// Map the tree hierarchy
		if currentName != rootName {
			// Avoid duplicate children entries
			found := false
			for _, child := range treeData[parentName] {
				if child == currentName {
					found = true
					break
				}
			}
			if !found {
				treeData[parentName] = append(treeData[parentName], currentName)
			}

			if _, exists := treeData[currentName]; !exists {
				treeData[currentName] = []string{}
			}
		}

		// Extract messages
		messageIterator, err := folder.GetMessageIterator()
		if eris.Is(err, pst.ErrMessagesNotFound) {
			// No messages in this folder, continue to subfolders
		} else if err != nil {
			return fmt.Errorf("failed to get messages: %w", err)
		} else {
			for messageIterator.Next() {
				message := messageIterator.Value()

				if messageProperties, ok := message.Properties.(*properties.Message); ok {
					subject := messageProperties.GetSubject()
					sender := messageProperties.GetSenderName()
					if sender == "" {
						sender = messageProperties.GetSenderEmailAddress()
					}
					displayTo := messageProperties.GetDisplayTo()
					bodyText := messageProperties.GetBody()
					bodyHTML := messageProperties.GetBodyHtml()

					if bodyText == "" && bodyHTML != "" {
						var err error
						bodyText, err = html2text.FromString(bodyHTML, html2text.Options{PrettyTables: true})
						if err != nil {
							fmt.Printf("Warning: failed to convert HTML to text in PST: %v\n", err)
						}
					}

					headers := messageProperties.GetTransportMessageHeaders()

					var dateStr string
					clientTime := messageProperties.GetClientSubmitTime()
					if clientTime != 0 {
						// go-pst returns Unix Nano epoch for dates
						t := time.Unix(0, clientTime)
						dateStr = core.FormatDate(t)
					}
					evidence := &core.Evidence{
						Folder:   currentName,
						From:     sender,
						To:       displayTo,
						Date:     dateStr,
						Subject:  subject,
						BodyText: bodyText,
						BodyHTML: bodyHTML,
						Headers:  headers,
					}

					// Insert evidence
					if id, err := database.InsertEvidence(evidence); err != nil {
						fmt.Printf("Error storing message: %v\n", err)
					} else {
						// Process attachments
						attachmentIterator, err := message.GetAttachmentIterator()
						if err == nil {
							for attachmentIterator.Next() {
								att := attachmentIterator.Value()
								var buf bytes.Buffer
								size, err := att.WriteTo(&buf)
								if err == nil {
									data := buf.Bytes()
									md5Hash := fmt.Sprintf("%x", md5.Sum(data))
									sha256Hash := fmt.Sprintf("%x", sha256.Sum256(data))
									name := att.GetAttachLongFilename()
									if name == "" {
										name = att.GetAttachFilename()
									}
									if name == "" {
										name = fmt.Sprintf("UNKNOWN_%d", att.Identifier)
									}
									database.InsertAttachment(&core.Attachment{
										EvidenceID: int(id),
										Filename:   name,
										Size:       size,
										MD5:        md5Hash,
										SHA256:     sha256Hash,
									}, data)
								}
							}
						}
					}
				}
			}
			if err := messageIterator.Err(); err != nil {
				return fmt.Errorf("message iterator error: %w", err)
			}
		}

		// Iterate subfolders
		subFolders, err := folder.GetSubFolders()
		if err != nil {
			return fmt.Errorf("failed to get subfolders: %w", err)
		}

		for _, subFolder := range subFolders {
			if err := walk(&subFolder, currentName); err != nil {
				return err
			}
		}

		return nil
	}

	if err := walk(&rootFolder, ""); err != nil {
		return nil, "", err
	}

	return treeData, rootName, nil
}
