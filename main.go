package main

import (
	"Bibliorato/core"
	"Bibliorato/database"
	"Bibliorato/parser"
	_ "Bibliorato/plugins"
	"encoding/csv"
	"fmt"
	"image/color"
	"io"
	"log"
	"os"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var currentFolder string
var currentCount int
var triageTable *widget.Table
var appTabs *container.AppTabs
var tabButtons *fyne.Container
var tabContentArea *fyne.Container
var myWindow fyne.Window
var dbFilePath string
var statusBar *widget.Label

// Global tree state
var leftTree *widget.Tree
var treeDataMap = map[string][]string{
	"":                {"Loaded Evidence"},
	"Loaded Evidence": {},
}

// --- Table Resize Logic ---
type dragHandle struct {
	widget.BaseWidget
	col       int
	onDragged func(col int, dx float32)
	bg        *canvas.Rectangle
}

func newDragHandle(onDragged func(col int, dx float32)) *dragHandle {
	d := &dragHandle{onDragged: onDragged}
	d.bg = canvas.NewRectangle(theme.DisabledColor())
	d.ExtendBaseWidget(d)
	return d
}

func (d *dragHandle) CreateRenderer() fyne.WidgetRenderer {
	d.bg.SetMinSize(fyne.NewSize(2, 0))
	c := container.NewCenter(d.bg)
	return widget.NewSimpleRenderer(c)
}

func (d *dragHandle) MinSize() fyne.Size {
	return fyne.NewSize(10, 20)
}

func (d *dragHandle) Dragged(e *fyne.DragEvent) {
	if d.onDragged != nil {
		d.onDragged(d.col, e.Dragged.DX)
	}
}

func (d *dragHandle) DragEnd() {}

func (d *dragHandle) Cursor() desktop.Cursor {
	return desktop.HResizeCursor
}

func (d *dragHandle) MouseIn(e *desktop.MouseEvent) {
	d.bg.FillColor = theme.PrimaryColor()
	d.bg.Refresh()
}

func (d *dragHandle) MouseMoved(e *desktop.MouseEvent) {}

func (d *dragHandle) MouseOut() {
	d.bg.FillColor = theme.DisabledColor()
	d.bg.Refresh()
}

type headerCell struct {
	widget.BaseWidget
	label  *widget.Label
	handle *dragHandle
}

func newHeaderCell(onDragged func(col int, dx float32)) *headerCell {
	h := &headerCell{}
	h.label = widget.NewLabel("")
	h.label.TextStyle.Bold = true
	h.handle = newDragHandle(onDragged)
	h.ExtendBaseWidget(h)
	return h
}

func (h *headerCell) CreateRenderer() fyne.WidgetRenderer {
	c := container.NewBorder(nil, nil, nil, h.handle, h.label)
	return widget.NewSimpleRenderer(c)
}

// --- Dynamic Table Layout ---
type autoTableLayout struct {
	table     *widget.Table
	colWidths []float32
}

func (l *autoTableLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return l.table.MinSize()
}

func (l *autoTableLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	l.table.Resize(size)

	var usedWidth float32
	for i := 0; i < len(l.colWidths)-1; i++ {
		usedWidth += l.colWidths[i]
	}

	remaining := size.Width - usedWidth
	if remaining < 100 {
		remaining = 100
	}

	lastCol := len(l.colWidths) - 1
	if l.colWidths[lastCol] != remaining {
		l.colWidths[lastCol] = remaining
		l.table.SetColumnWidth(lastCol, remaining)
		l.table.Refresh()
	}
}

// --- End Table Resize Logic ---
func logMessage(msg string) {
	if statusBar != nil {
		statusBar.SetText(core.Sanitize(msg))
	}
	log.Println(msg)
}

type browserTabButton struct {
	widget.BaseWidget
	text     string
	selected bool
	onSelect func()
	onClose  func()

	label    *widget.Label
	closeBtn *widget.Button
	bg       *canvas.Rectangle
}

func newBrowserTabButton(text string, selected bool, hasClose bool, onSelect, onClose func()) *browserTabButton {
	b := &browserTabButton{
		text:     text,
		selected: selected,
		onSelect: onSelect,
		onClose:  onClose,
	}
	b.label = widget.NewLabel(text)
	b.label.Alignment = fyne.TextAlignCenter

	b.bg = canvas.NewRectangle(theme.ButtonColor())
	if selected {
		b.bg.FillColor = theme.SelectionColor()
	}

	if hasClose {
		b.closeBtn = widget.NewButtonWithIcon("", theme.CancelIcon(), onClose)
		b.closeBtn.Importance = widget.LowImportance
	}
	b.ExtendBaseWidget(b)
	return b
}

func (b *browserTabButton) CreateRenderer() fyne.WidgetRenderer {
	var c *fyne.Container
	if b.closeBtn != nil {
		c = container.NewBorder(nil, nil, nil, b.closeBtn, b.label)
	} else {
		c = container.NewBorder(nil, nil, nil, nil, b.label)
	}
	padded := container.NewPadded(c)
	return widget.NewSimpleRenderer(container.NewStack(b.bg, padded))
}

func (b *browserTabButton) Tapped(ev *fyne.PointEvent) {
	if b.onSelect != nil {
		b.onSelect()
	}
}

func (b *browserTabButton) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button == desktop.MouseButtonTertiary {
		if b.onClose != nil {
			b.onClose()
		}
	}
}

func (b *browserTabButton) MouseUp(ev *desktop.MouseEvent) {}

func (b *browserTabButton) MouseIn(ev *desktop.MouseEvent) {
	if !b.selected {
		b.bg.FillColor = theme.HoverColor()
		b.bg.Refresh()
	}
}

func (b *browserTabButton) MouseMoved(ev *desktop.MouseEvent) {}

func (b *browserTabButton) MouseOut() {
	if !b.selected {
		b.bg.FillColor = theme.ButtonColor()
		b.bg.Refresh()
	}
}

func refreshTabButtons() {
	if tabButtons == nil || appTabs == nil || tabContentArea == nil {
		return
	}
	tabButtons.Objects = nil
	selectedItem := appTabs.Selected()
	for i, item := range appTabs.Items {
		capturedItem := item
		capturedIndex := i
		isSelected := capturedItem == selectedItem

		var tabWidget fyne.CanvasObject
		if item.Text == "Triage List" {
			tabWidget = newBrowserTabButton(item.Text, isSelected, false, func() {
				selectTab(capturedItem)
			}, nil)
		} else {
			closeFunc := func() {
				appTabs.Remove(capturedItem)
				if len(appTabs.Items) > 0 {
					newIndex := capturedIndex
					if newIndex >= len(appTabs.Items) {
						newIndex = len(appTabs.Items) - 1
					}
					selectTab(appTabs.Items[newIndex])
				}
				refreshTabButtons()
			}
			tabWidget = newBrowserTabButton(item.Text, isSelected, true, func() {
				selectTab(capturedItem)
			}, closeFunc)
		}
		tabButtons.Add(tabWidget)
	}
	tabButtons.Refresh()
}

func selectTab(item *container.TabItem) {
	appTabs.Select(item)
	tabContentArea.Objects = []fyne.CanvasObject{item.Content}
	tabContentArea.Refresh()
	refreshTabButtons()
}

// --- Custom High-Contrast Theme ---
type customTheme struct {
	fyne.Theme
}

func (m customTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if variant == theme.VariantDark {
		switch name {
		case theme.ColorNameBackground:
			return color.NRGBA{R: 0x1e, G: 0x1e, B: 0x1e, A: 0xff} // Darker charcoal, not pitch black
		case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
			return color.NRGBA{R: 0x28, G: 0x28, B: 0x28, A: 0xff} // Neutral dark gray panel
		case theme.ColorNameHover:
			return color.NRGBA{R: 0x3a, G: 0x3a, B: 0x3a, A: 0xff} // Visible hover state for menus
		case theme.ColorNameSelection:
			return color.NRGBA{R: 0x2a, G: 0x52, B: 0x7a, A: 0xff} // Muted steel blue
		case theme.ColorNameButton:
			return color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xff}
		case theme.ColorNameHeaderBackground:
			return color.NRGBA{R: 0x2d, G: 0x2d, B: 0x2d, A: 0xff}
		case theme.ColorNameSeparator:
			return color.NRGBA{R: 0x44, G: 0x44, B: 0x44, A: 0xff} // More visible dividers
		}
	} else {
		// High-contrast Light
		switch name {
		case theme.ColorNameBackground:
			return color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 0xff}
		case theme.ColorNameSelection:
			return color.NRGBA{R: 0x00, G: 0x5f, B: 0xaf, A: 0xff}
		}
	}
	return m.Theme.Color(name, variant)
}

func main() {
	// Use an actual temporary file to handle massive DBs and allow Live Export
	tmpFile, err := os.CreateTemp("", "bibliorato-*.db")
	if err != nil {
		log.Fatalf("Failed to create temp db file: %v", err)
	}
	dbFilePath = tmpFile.Name()
	tmpFile.Close() // Just needed the name, SQLite will manage it

	err = database.InitDB(dbFilePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		database.CloseDB()
		os.Remove(dbFilePath)
	}()

	myApp := app.New()
	myApp.Settings().SetTheme(&customTheme{Theme: theme.DefaultTheme()})
	myWindow = myApp.NewWindow("Bibliorato - Mail Analyzer")
	myWindow.Resize(fyne.NewSize(1024, 768))

	statusBar = widget.NewLabel("Ready")
	statusBg := canvas.NewRectangle(color.NRGBA{R: 0x1f, G: 0x3b, B: 0x57, A: 0xff}) // Muted slate blue
	statusContainer := container.NewStack(statusBg, container.NewPadded(statusBar))

	// --- Main Menu ---
	openStoreItem := fyne.NewMenuItem("Open Mail Store...", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, myWindow)
				return
			}
			if reader == nil {
				return // User cancelled
			}
			defer reader.Close()
			logMessage(fmt.Sprintf("Loading store: %s", reader.URI().Path()))
			loadStore(reader.URI().Path())
		}, myWindow)
		fd.SetFilter(storage.NewExtensionFileFilter(parser.GetSupportedStoreExtensions()))
		fd.Resize(fyne.NewSize(800, 600))
		fd.Show()
	})

	addFileItem := fyne.NewMenuItem("Add Mail File...", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, myWindow)
				return
			}
			if reader == nil {
				return // User cancelled
			}
			defer reader.Close()
			logMessage(fmt.Sprintf("Adding file: %s", reader.URI().Path()))
			addFile(reader.URI().Path())
		}, myWindow)
		fd.SetFilter(storage.NewExtensionFileFilter(parser.GetSupportedFileExtensions()))
		fd.Resize(fyne.NewSize(800, 600))
		fd.Show()
	})

	exportDBItem := fyne.NewMenuItem("Live Export DB...", func() {
		fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
			if err != nil {
				dialog.ShowError(err, myWindow)
				return
			}
			if writer == nil {
				return
			}
			defer writer.Close()

			src, err := os.Open(dbFilePath)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to open source db: %w", err), myWindow)
				return
			}
			defer src.Close()

			_, err = io.Copy(writer, src)
			if err != nil {
				dialog.ShowError(fmt.Errorf("failed to export db: %w", err), myWindow)
			} else {
				logMessage("Database exported successfully.")
				dialog.ShowInformation("Export Complete", "Live Export of SQLite database successful.", myWindow)
			}
		}, myWindow)
		fd.SetFileName("bibliorato-export.sqlite")
		fd.Resize(fyne.NewSize(800, 600))
		fd.Show()
	})

	mainMenu := fyne.NewMainMenu(fyne.NewMenu("Analyzer", openStoreItem, addFileItem, exportDBItem))
	myWindow.SetMainMenu(mainMenu)

	// --- Left Pane: File/Folder Tree ---
	leftTree = widget.NewTreeWithStrings(treeDataMap)
	leftTree.OnSelected = func(uid string) {
		currentFolder = uid
		count, err := database.GetEvidenceCount(currentFolder)
		if err == nil {
			currentCount = count
			triageTable.Refresh()
		}
	}
	leftTree.Hide()

	// --- Right Pane: Custom Tabs ---
	appTabs = container.NewAppTabs()
	// We don't use standard appTabs display, but we use it as a data structure.
	tabButtons = container.NewHBox()
	tabContentArea = container.NewStack()

	// Tab 1: Triage List (Table)
	var colWidths = []float32{50, 200, 200, 300, 150}

	triageTable = widget.NewTable(
		func() (int, int) {
			rows := currentCount
			if rows < 50 {
				rows = 50 // Minimum rows for infinite spreadsheet feel
			}
			return rows, 5
		}, // 5 columns: ID, From, To, Subject, Date
		func() fyne.CanvasObject {
			bg := canvas.NewRectangle(color.Transparent)
			lbl := widget.NewLabel("")
			lbl.Truncation = fyne.TextTruncateEllipsis
			return container.NewStack(bg, lbl)
		},
		func(i widget.TableCellID, o fyne.CanvasObject) {
			stack := o.(*fyne.Container)
			bg := stack.Objects[0].(*canvas.Rectangle)
			lbl := stack.Objects[1].(*widget.Label)

			// Row Striping
			if i.Row%2 == 0 {
				if fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantDark {
					bg.FillColor = color.NRGBA{R: 0x25, G: 0x25, B: 0x2a, A: 0xff} // subtle dark stripe
				} else {
					bg.FillColor = color.NRGBA{R: 0xe5, G: 0xe5, B: 0xe5, A: 0xff} // subtle light stripe
				}
			} else {
				bg.FillColor = color.Transparent
			}
			bg.Refresh()

			if i.Row < currentCount {
				email, err := database.GetEvidenceByIndex(i.Row, currentFolder)
				if err == nil && email != nil {
					switch i.Col {
					case 0:
						lbl.SetText(fmt.Sprintf("%d", email.ID))
					case 1:
						lbl.SetText(core.Sanitize(email.From))
					case 2:
						lbl.SetText(core.Sanitize(email.To))
					case 3:
						lbl.SetText(core.Sanitize(email.Subject))
					case 4:
						lbl.SetText(core.Sanitize(email.Date))
					}
				}
			} else {
				lbl.SetText("")
			}
		})

	triageTable.ShowHeaderRow = true
	triageTable.CreateHeader = func() fyne.CanvasObject {
		return newHeaderCell(func(col int, dx float32) {
			colWidths[col] += dx
			if colWidths[col] < 30 {
				colWidths[col] = 30 // Minimum width
			}
			triageTable.SetColumnWidth(col, colWidths[col])
			triageTable.Refresh()
		})
	}
	triageTable.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		if id.Row == -1 {
			cell := o.(*headerCell)
			cell.handle.col = id.Col
			switch id.Col {
			case 0:
				cell.label.SetText("ID")
				cell.handle.Show()
			case 1:
				cell.label.SetText("From")
				cell.handle.Show()
			case 2:
				cell.label.SetText("To")
				cell.handle.Show()
			case 3:
				cell.label.SetText("Subject")
				cell.handle.Show()
			case 4:
				cell.label.SetText("Date")
				// Hide handle for the last column since it stretches automatically
				cell.handle.Hide()
			}
		}
	}
	for i, w := range colWidths {
		triageTable.SetColumnWidth(i, w)
	}

	triageTable.OnSelected = func(id widget.TableCellID) {
		if id.Row < currentCount {
			email, err := database.GetEvidenceByIndex(id.Row, currentFolder)
			if err == nil && email != nil {
				openEmailDetailTab(email)
			}
		}
		triageTable.Unselect(id)
	}

	tableLayout := &autoTableLayout{table: triageTable, colWidths: colWidths}
	tableContainer := container.New(tableLayout, triageTable)

	triageTab := container.NewTabItem("Triage List", tableContainer)
	appTabs.Append(triageTab)
	selectTab(triageTab)
	refreshTabButtons()

	// --- Main Split View ---
	tabScroll := container.NewHScroll(tabButtons)

	toggleButton := widget.NewButtonWithIcon("", theme.MenuIcon(), func() {
		if leftTree.Visible() {
			leftTree.Hide()
		} else {
			leftTree.Show()
		}
	})
	toggleButton.Importance = widget.LowImportance
	tabHeader := container.NewBorder(nil, nil, toggleButton, nil, tabScroll)

	rightPane := container.NewBorder(tabHeader, nil, nil, nil, tabContentArea)
	mainSplit := container.NewHSplit(leftTree, rightPane)
	mainSplit.SetOffset(0.2)

	myWindow.SetContent(container.NewBorder(nil, statusContainer, nil, nil, mainSplit))

	myWindow.ShowAndRun()
}

func loadStore(path string) {
	err := database.ClearDB()
	if err != nil {
		logMessage(fmt.Sprintf("Warning: Failed to clear previous data: %v", err))
	}

	currentFolder = ""
	currentCount = 0
	if leftTree != nil {
		leftTree.Unselect(currentFolder)
	}
	if triageTable != nil {
		triageTable.Refresh()
	}

	if appTabs != nil && len(appTabs.Items) > 1 {
		appTabs.Items = appTabs.Items[:1]
		selectTab(appTabs.Items[0])
		refreshTabButtons()
	}

	newTree, _, err := parser.ProcessStore(path)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to process store: %w", err), myWindow)
		return
	}

	for k := range treeDataMap {
		delete(treeDataMap, k)
	}
	for k, v := range newTree {
		treeDataMap[k] = v
	}
	leftTree.Refresh()

	currentFolder = ""
	count, err := database.GetEvidenceCount(currentFolder)
	if err == nil {
		currentCount = count
		triageTable.Refresh()
	}

	logMessage("Store loaded successfully.")
}

func addFile(path string) {
	_, err := parser.ProcessFile(path)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to process mail file: %w", err), myWindow)
		return
	}

	if _, ok := treeDataMap["Loaded Evidence"]; !ok {
		treeDataMap[""] = append(treeDataMap[""], "Loaded Evidence")
		treeDataMap["Loaded Evidence"] = []string{}
	}

	if leftTree != nil {
		leftTree.Refresh()
	}

	count, err := database.GetEvidenceCount(currentFolder)
	if err == nil {
		currentCount = count
		if triageTable != nil {
			triageTable.Refresh()
		}
	}

	logMessage("Mail file added successfully.")
}

func openEmailDetailTab(email *core.Evidence) {
	tabName := fmt.Sprintf("Email %d", email.ID)
	for _, item := range appTabs.Items {
		if item.Text == tabName {
			selectTab(item)
			return
		}
	}

	loadingProgressBar := widget.NewProgressBarInfinite()
	loadingLabel := widget.NewLabel("Rendering email content...")
	loadingContainer := container.NewCenter(container.NewVBox(loadingLabel, loadingProgressBar))

	contentStack := container.NewStack(loadingContainer)

	var currentLoadSeq int

	bodyFormatSelect := widget.NewSelect([]string{"Text", "HTML"}, func(s string) {
		currentLoadSeq++
		seq := currentLoadSeq

		contentStack.Objects = []fyne.CanvasObject{loadingContainer}
		contentStack.Refresh()
		logMessage(fmt.Sprintf("Loading %s view for email %d...", s, email.ID))

		go func(format string, expectedSeq int) {
			headerText := fmt.Sprintf(
				"# %s\n\n**From:**\n%s\n\n**To:**\n%s\n\n**Date:**\n%s\n\n---\n\n### Headers\n\n```\n%s\n```\n\n---\n\n### Body\n\n",
				core.EscapeMarkdown(core.Sanitize(email.Subject)),
				core.EscapeMarkdown(core.Sanitize(email.From)),
				core.EscapeMarkdown(core.Sanitize(email.To)),
				core.EscapeMarkdown(core.Sanitize(email.Date)),
				email.Headers,
			)

			var fullText string
			if format == "HTML" {
				fullText = headerText + core.Sanitize(email.BodyText)
			} else {
				rawCode := email.BodyHTML
				if rawCode == "" {
					rawCode = email.BodyText
				}
				fullText = headerText + "```text\n" + core.Sanitize(rawCode) + "\n```"
			}

			newLabel := widget.NewRichTextFromMarkdown(fullText)
			newLabel.Wrapping = fyne.TextWrapWord
			newScroll := container.NewVScroll(newLabel)

			if currentLoadSeq != expectedSeq {
				return // A newer request was fired
			}

			fyne.Do(func() {
				if currentLoadSeq != expectedSeq {
					return
				}
				contentStack.Objects = []fyne.CanvasObject{newScroll}
				contentStack.Refresh()
				logMessage("Ready")
			})
		}(s, seq)
	})
	bodyFormatSelect.SetSelected("Text")

	evidenceContainer := container.NewBorder(container.NewHBox(layout.NewSpacer(), widget.NewLabel("View:"), bodyFormatSelect), nil, nil, nil, contentStack)

	attachments, err := database.GetAttachmentsByEvidenceID(email.ID)
	if err != nil {
		logMessage(fmt.Sprintf("Error retrieving attachments: %v", err))
	}

	attachmentTray := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Attachments", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	if len(attachments) == 0 {
		attachmentTray.Add(widget.NewLabel("None"))
	} else {
		for _, att := range attachments {
			attLabel := widget.NewLabel(fmt.Sprintf("File: %s\nSize: %d bytes\nMD5: %s\nSHA256: %s", core.Sanitize(att.Filename), att.Size, att.MD5, att.SHA256))
			attLabel.Wrapping = fyne.TextWrapWord

			currentAtt := att
			exportBtn := widget.NewButton("Safe Export", func() {
				fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
					if err != nil {
						dialog.ShowError(err, myWindow)
						return
					}
					if writer == nil {
						return
					}
					defer writer.Close()
					data, err := database.GetAttachmentData(currentAtt.ID)
					if err == nil {
						writer.Write(data)
					} else {
						dialog.ShowError(err, myWindow)
					}
				}, myWindow)
				fd.Resize(fyne.NewSize(800, 600))
				fd.Show()
			})

			row := container.NewBorder(nil, nil, nil, exportBtn, attLabel)
			attachmentTray.Add(row)
		}
	}

	var leftEvidence fyne.CanvasObject
	if len(attachments) == 0 {
		leftEvidence = container.NewBorder(nil, attachmentTray, nil, nil, evidenceContainer)
	} else {
		attachmentScroll := container.NewVScroll(attachmentTray)
		split := container.NewVSplit(evidenceContainer, attachmentScroll)
		split.Offset = 0.75
		leftEvidence = split
	}

	enrichmentTabs := container.NewAppTabs()
	enrichmentTabs.Append(container.NewTabItem("Info", widget.NewLabel("Waiting for execution...")))

	runButton := widget.NewButton("Run Enrichers", func() {
		enrichmentTabs.Items = nil
		for _, plugin := range core.PluginRegistry {
			enrichments, err := plugin.Instance.Process(email)
			if err == nil && len(enrichments) > 0 {
				for _, en := range enrichments {
					r := csv.NewReader(strings.NewReader(en.Data))
					csvData, err := r.ReadAll()
					if err != nil {
						logMessage(fmt.Sprintf("Error parsing enrichment CSV: %v", err))
						continue
					}

					if len(csvData) > 0 {
						headers := csvData[0]
						dataRows := csvData[1:]
						rows := len(dataRows)
						cols := len(headers)

						colWidths := make([]float32, cols)
						for c := 0; c < cols; c++ {
							colWidths[c] = 150
						}

						table := widget.NewTable(
							func() (int, int) { return rows, cols },
							func() fyne.CanvasObject {
								bg := canvas.NewRectangle(color.Transparent)
								lbl := widget.NewLabel("Template Data")
								lbl.Truncation = fyne.TextTruncateEllipsis
								return container.NewStack(bg, lbl)
							},
							func(i widget.TableCellID, o fyne.CanvasObject) {
								stack := o.(*fyne.Container)
								bg := stack.Objects[0].(*canvas.Rectangle)
								lbl := stack.Objects[1].(*widget.Label)

								if i.Row%2 == 0 {
									if fyne.CurrentApp().Settings().ThemeVariant() == theme.VariantDark {
										bg.FillColor = color.NRGBA{R: 0x25, G: 0x25, B: 0x2a, A: 0xff}
									} else {
										bg.FillColor = color.NRGBA{R: 0xe5, G: 0xe5, B: 0xe5, A: 0xff}
									}
								} else {
									bg.FillColor = color.Transparent
								}
								bg.Refresh()

								if i.Row < len(dataRows) && i.Col < len(dataRows[i.Row]) {
									lbl.SetText(core.Sanitize(dataRows[i.Row][i.Col]))
								} else {
									lbl.SetText("")
								}
							},
						)

						table.ShowHeaderRow = true
						table.CreateHeader = func() fyne.CanvasObject {
							return newHeaderCell(func(col int, dx float32) {
								colWidths[col] += dx
								if colWidths[col] < 30 {
									colWidths[col] = 30
								}
								table.SetColumnWidth(col, colWidths[col])
								table.Refresh()
							})
						}
						table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
							if id.Row == -1 && id.Col < len(headers) {
								cell := o.(*headerCell)
								cell.handle.col = id.Col
								cell.label.SetText(headers[id.Col])
								if id.Col == len(headers)-1 {
									cell.handle.Hide()
								} else {
									cell.handle.Show()
								}
							}
						}

						for i, w := range colWidths {
							table.SetColumnWidth(i, w)
						}

						table.OnSelected = func(id widget.TableCellID) {
							if id.Row < len(dataRows) && id.Col < len(dataRows[id.Row]) {
								fullText := dataRows[id.Row][id.Col]
								entry := widget.NewEntry()
								entry.MultiLine = true
								entry.Wrapping = fyne.TextWrapWord
								entry.SetText(fullText)

								content := container.NewScroll(entry)
								d := dialog.NewCustom("Cell Detail", "Close", content, myWindow)
								d.Resize(fyne.NewSize(500, 300))
								d.Show()
							}
							table.Unselect(id)
						}

						tableLayout := &autoTableLayout{table: table, colWidths: colWidths}
						tableContainer := container.New(tableLayout, table)
						enrichmentTabs.Append(container.NewTabItem(plugin.Metadata.Name, tableContainer))
					}
				}
			}
		}

		if len(enrichmentTabs.Items) == 0 {
			enrichmentTabs.Append(container.NewTabItem("Info", widget.NewLabel("No enrichments available for this email.")))
		}
		enrichmentTabs.Refresh()
	})

	enrichmentContainer := container.NewBorder(runButton, nil, nil, nil, enrichmentTabs)
	detailSplit := container.NewHSplit(leftEvidence, enrichmentContainer)
	detailSplit.SetOffset(0.6)

	newTab := container.NewTabItem(tabName, detailSplit)
	appTabs.Append(newTab)
	selectTab(newTab)
	refreshTabButtons()
}
