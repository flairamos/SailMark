package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
	"unicode"

	"gioui.org/app"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

type findMatch struct {
	Block int
	Start int
	End   int
}

type modalState struct {
	pending string // "open" or "close"
}

type dialogResult struct {
	kind string
	path string
	err  error
}

type App struct {
	w      *app.Window
	fonts  *fontSet
	images map[string]*imageEntry

	doc     *Document
	path    string
	docOpen bool
	dirty   bool

	openBtn  widget.Clickable
	saveBtn  widget.Clickable
	closeBtn widget.Clickable
	findBtn  widget.Clickable

	welcomeOpen widget.Clickable

	blockEditor      widget.Editor
	editIdx          int
	editRect         image.Rectangle
	editFocusPending bool

	findOpen      bool
	findEd        widget.Editor
	findPrev      widget.Clickable
	findNextBtn   widget.Clickable
	findClose     widget.Clickable
	findQuery     string
	findMatches   []findMatch
	findCur       int
	findFocusWait bool

	tocOpen    bool
	tocBtn     widget.Clickable
	tocClose   widget.Clickable
	tocList    widget.List
	tocClicks  []widget.Clickable
	tocEntries []tocEntry
	tocTops    []int
	tocCurrent int
	tocShown   int
	tocJump    int

	viewportClick  gesture.Click
	viewportScroll gesture.Scroll
	globalClick    gesture.Click
	scroll         int
	contentH       int
	blockRects     []image.Rectangle
	viewportRect   image.Rectangle

	pendingScrollBlock int

	dialogCh   chan dialogResult
	dialogBusy bool

	modal        *modalState
	modalSave    widget.Clickable
	modalDiscard widget.Clickable
	modalCancel  widget.Clickable
	modalClick   gesture.Click

	status         string
	statusColorKey int
	lastStatus     time.Time

	// Heights are cached per block so off-screen blocks can be positioned
	// without laying them out. A zero entry means "not measured yet" and is
	// filled in with an estimate.
	blockHeights []int
	blockTops    []int
	heightsWidth int
	heightsDoc   *Document

	// anchorBlock/anchorOffset remember where the reader is, so toggling the
	// outline or resizing the window does not move the text under them.
	anchorBlock   int
	anchorOffset  int
	anchorPending bool

	wordCount      int
	wordCountValid bool
}

func main() {
	// A long Markdown file used to let the Go heap grow to a gigabyte or more
	// because every block was laid out on every frame. The viewport now only
	// lays out what is visible, and this soft limit keeps the garbage collector
	// from ever letting the process balloon again.
	debug.SetMemoryLimit(baseMemLimit)
	a := &App{
		editIdx:            -1,
		findCur:            -1,
		pendingScrollBlock: -1,
		tocCurrent:         -1,
		tocJump:            -1,
		dialogCh:           make(chan dialogResult, 1),
	}
	a.configureEditors()
	if len(os.Args) > 1 {
		if err := a.openPath(os.Args[1]); err != nil {
			a.setStatus("打开失败: "+err.Error(), 1)
			fmt.Fprintln(os.Stderr, err)
		}
	}
	go a.run()
	app.Main()
}

// baseMemLimit is the soft heap budget for a small document. Opening a large
// file raises it so the collector never fights a live set it cannot shrink.
const baseMemLimit = 128 << 20

// memoryLimitFor scales the heap budget with the source size: roughly one
// hundred bytes of retained layout data per source byte, with the base budget
// as the floor.
func memoryLimitFor(sourceLen int) int64 {
	limit := int64(baseMemLimit)
	if extra := int64(sourceLen) * 96; extra > limit {
		limit = extra
	}
	return limit
}

func (a *App) run() {
	a.w = new(app.Window)
	a.w.Option(
		app.Title("SailMark"),
		app.Size(unit.Dp(1000), unit.Dp(620)),
		app.MinSize(unit.Dp(620), unit.Dp(440)),
	)
	fonts, err := loadFonts()
	if err != nil {
		fmt.Fprintln(os.Stderr, "font error:", err)
	}
	a.fonts = fonts
	if a.docOpen {
		a.updateTitle()
	}
	var ops op.Ops
	for {
		e := a.w.Event()
		switch e := e.(type) {
		case app.DestroyEvent:
			if e.Err != nil {
				fmt.Fprintln(os.Stderr, e.Err)
			}
			os.Exit(0)
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (a *App) configureEditors() {
	a.blockEditor.SingleLine = false
	a.blockEditor.LineHeight = unit.Sp(20)
	a.blockEditor.LineHeightScale = 1.55
	a.blockEditor.WrapPolicy = text.WrapGraphemes
	a.blockEditor.Submit = false

	a.findEd.SingleLine = true
	a.findEd.Submit = true
	a.findEd.LineHeight = unit.Sp(18)
}

func (a *App) Layout(gtx layout.Context) layout.Dimensions {
	if a.fonts == nil {
		// A font loading failure should not leave a blank, undebuggable window.
		fillRect(gtx.Ops, image.Rectangle{Max: gtx.Constraints.Max}, colBackground)
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	a.pollDialog()
	a.handleShortcuts(gtx)
	area := clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops)
	a.globalClick.Add(gtx.Ops)
	dims := layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return a.layoutMain(gtx)
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if a.modal != nil {
				a.layoutModal(gtx)
			}
			return layout.Dimensions{Size: gtx.Constraints.Max}
		}),
	)
	area.Pop()
	a.handleGlobalClick(gtx)
	return dims
}

func (a *App) pollDialog() {
	select {
	case r := <-a.dialogCh:
		a.dialogBusy = false
		if r.err != nil {
			a.setStatus("文件对话框失败: "+r.err.Error(), 1)
			a.invalidate()
			return
		}
		if r.path == "" {
			a.setStatus("已取消", 0)
			a.invalidate()
			return
		}
		switch r.kind {
		case "open":
			if err := a.openPath(r.path); err != nil {
				a.setStatus("打开失败: "+err.Error(), 1)
				a.invalidate()
			}
		case "save":
			a.finishSavePath(r.path)
		}
	default:
	}
}

func (a *App) handleGlobalClick(gtx layout.Context) {
	modalOpen := a.modal != nil
	for {
		ev, ok := a.globalClick.Update(gtx.Source)
		if !ok {
			break
		}
		if modalOpen || ev.Kind != gesture.KindPress || a.editIdx < 0 {
			continue
		}
		// Clicks inside the document viewport are handled by the viewport's
		// own gesture (which knows how to switch blocks). Everywhere else,
		// such as the menu or find bar, ends the current edit.
		if !ev.Position.In(a.viewportRect) {
			a.commitEdit()
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (a *App) layoutMain(gtx layout.Context) layout.Dimensions {
	fillRect(gtx.Ops, image.Rectangle{Max: gtx.Constraints.Max}, colBackground)
	children := []layout.FlexChild{
		layout.Rigid(a.layoutMenu),
	}
	if a.findOpen {
		children = append(children, layout.Rigid(a.layoutFindBar))
	}
	children = append(children,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if !a.tocOpen {
				return a.layoutViewport(gtx)
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(a.layoutTOC),
				layout.Flexed(1, a.layoutViewport),
			)
		}),
		layout.Rigid(a.layoutStatusBar),
	)
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	return dims
}

func (a *App) handleShortcuts(gtx layout.Context) {
	if a.dialogBusy {
		return
	}
	filters := []event.Filter{
		key.Filter{Name: "O", Required: key.ModShortcut},
		key.Filter{Name: "S", Required: key.ModShortcut},
		key.Filter{Name: "W", Required: key.ModShortcut},
		key.Filter{Name: "F", Required: key.ModShortcut},
		key.Filter{Name: "G", Required: key.ModShortcut},
		key.Filter{Name: "T", Required: key.ModShortcut},
		key.Filter{Name: key.NameF3},
		key.Filter{Name: key.NameEscape},
		key.Filter{Name: key.NameReturn, Required: key.ModCtrl},
	}
	for {
		ev, ok := gtx.Event(filters...)
		if !ok {
			break
		}
		ke, ok := ev.(key.Event)
		if !ok || ke.State != key.Press {
			continue
		}
		if a.modal != nil {
			if ke.Name == key.NameEscape {
				a.closeModal()
			}
			continue
		}
		switch ke.Name {
		case "O":
			a.requestOpen()
		case "S":
			a.saveDocument()
		case "W":
			a.requestClose()
		case "F":
			a.toggleFind()
		case "G":
			a.findNext(1)
		case "T":
			a.toggleTOC()
		case key.NameF3:
			a.findNext(1)
		case key.NameEscape:
			if a.findOpen {
				a.closeFind()
			} else if a.editIdx >= 0 {
				a.commitEdit()
			} else if a.tocOpen {
				a.tocOpen = false
				a.invalidate()
			}
		case key.NameReturn:
			if a.editIdx >= 0 {
				a.commitEdit()
			}
		}
	}
}

func (a *App) setStatus(msg string, c int) {
	a.status = msg
	a.statusColorKey = c
	a.lastStatus = time.Now()
}

func (a *App) currentFileName() string {
	if a.path == "" {
		return "未命名.md"
	}
	return filepath.Base(a.path)
}

func (a *App) updateTitle() {
	if a.w == nil {
		return
	}
	title := "SailMark"
	if a.docOpen {
		title += " - " + a.currentFileName()
	}
	if a.dirty {
		title = "● " + title
	}
	a.w.Option(app.Title(title))
}

func (a *App) openPath(path string) error {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := strings.TrimPrefix(string(data), "\ufeff")
	a.doc = parseDocument(text)
	debug.SetMemoryLimit(memoryLimitFor(len(data)))
	a.resetLayoutCache()
	a.invalidateWordCount()
	a.path = path
	a.docOpen = true
	a.dirty = false
	a.editIdx = -1
	a.editRect = image.Rectangle{}
	a.editFocusPending = false
	a.scroll = 0
	a.contentH = 0
	a.pendingScrollBlock = -1
	a.findOpen = false
	a.findMatches = nil
	a.findQuery = ""
	a.findEd.SetText("")
	a.findCur = -1
	a.blockRects = a.blockRects[:0]
	a.images = nil
	a.tocJump = -1
	a.rebuildTOC()
	a.setStatus("已打开 "+filepath.Base(path), 0)
	a.updateTitle()
	a.invalidate()
	// Parsing and the first layout leave transient buffers behind; hand the
	// freed pages back to the OS instead of keeping a large working set.
	go debug.FreeOSMemory()
	return nil
}

func (a *App) startOpenDialog() {
	if a.dialogBusy {
		return
	}
	a.dialogBusy = true
	a.setStatus("正在选择文件…", 0)
	a.invalidate()
	go func() {
		// The common file dialog creates its own modal message loop. Running
		// it on a dedicated, locked OS thread (with no cross-thread owner)
		// keeps it away from Gio's window thread and avoids the deadlock that
		// a cross-thread hwndOwner can cause.
		runtime.LockOSThread()
		path, err := openFileDialog("打开 Markdown 文件")
		a.dialogCh <- dialogResult{kind: "open", path: path, err: err}
		a.invalidate()
	}()
}

func (a *App) startSaveDialog() {
	if a.dialogBusy {
		return
	}
	a.dialogBusy = true
	a.setStatus("正在选择保存位置…", 0)
	a.invalidate()
	go func() {
		runtime.LockOSThread()
		path, err := saveFileDialog("保存 Markdown 文件", "未命名.md")
		a.dialogCh <- dialogResult{kind: "save", path: path, err: err}
		a.invalidate()
	}()
}

func (a *App) finishSavePath(path string) {
	if filepath.Ext(path) == "" {
		path += ".md"
	}
	a.path = path
	if a.writeCurrentDocument() {
		if a.modal != nil {
			a.performPending()
		}
	}
}

func (a *App) requestOpen() {
	a.commitEdit()
	if a.dirty {
		a.modal = &modalState{pending: "open"}
		a.invalidate()
		return
	}
	a.startOpenDialog()
}

func (a *App) performPending() {
	if a.dialogBusy {
		return
	}
	pending := ""
	if a.modal != nil {
		pending = a.modal.pending
	}
	a.modal = nil
	switch pending {
	case "open":
		a.startOpenDialog()
	case "close":
		a.closeDoc()
	}
	a.invalidate()
}

func (a *App) saveDocument() bool {
	a.commitEdit()
	if !a.docOpen || a.doc == nil {
		a.setStatus("没有可保存的文件", 1)
		a.invalidate()
		return false
	}
	if a.path == "" {
		a.startSaveDialog()
		return false
	}
	return a.writeCurrentDocument()
}

func (a *App) writeCurrentDocument() bool {
	if !a.docOpen || a.doc == nil || a.path == "" {
		return false
	}
	a.commitEdit()
	src := a.doc.Source()
	if err := os.WriteFile(a.path, []byte(src), 0644); err != nil {
		a.setStatus("保存失败: "+err.Error(), 1)
		a.invalidate()
		return false
	}
	a.dirty = false
	a.setStatus("已保存 "+filepath.Base(a.path), 2)
	a.updateTitle()
	a.invalidate()
	return true
}

func (a *App) requestClose() {
	a.commitEdit()
	if a.dirty {
		a.modal = &modalState{pending: "close"}
		a.invalidate()
		return
	}
	a.closeDoc()
}

func (a *App) closeDoc() {
	a.doc = nil
	debug.SetMemoryLimit(baseMemLimit)
	a.resetLayoutCache()
	a.invalidateWordCount()
	a.path = ""
	a.docOpen = false
	a.dirty = false
	a.editIdx = -1
	a.editRect = image.Rectangle{}
	a.editFocusPending = false
	a.scroll = 0
	a.contentH = 0
	a.findOpen = false
	a.findMatches = nil
	a.findQuery = ""
	a.findEd.SetText("")
	a.findCur = -1
	a.pendingScrollBlock = -1
	a.images = nil
	a.tocOpen = false
	a.tocEntries = nil
	a.tocClicks = a.tocClicks[:0]
	a.tocTops = a.tocTops[:0]
	a.tocCurrent = -1
	a.tocJump = -1
	a.setStatus("文件已关闭", 0)
	a.updateTitle()
	a.invalidate()
	go debug.FreeOSMemory()
}

func (a *App) closeModal() {
	a.modal = nil
	a.invalidate()
}

func (a *App) invalidate() {
	if a.w != nil {
		a.w.Invalidate()
	}
}

// invalidateHeights drops every cached block height after the content changed.
// The reading anchor survives, so the next frame can put the same block back
// under the reader instead of jumping when the estimates kick in again.
func (a *App) invalidateHeights() {
	a.blockHeights = a.blockHeights[:0]
	a.blockTops = a.blockTops[:0]
	a.heightsWidth = 0
	a.heightsDoc = nil
	a.anchorPending = true
}

// resetLayoutCache drops the height cache and the reading anchor. It is used
// when a different document is opened or the current one is closed.
func (a *App) resetLayoutCache() {
	a.blockHeights = a.blockHeights[:0]
	a.blockTops = a.blockTops[:0]
	a.heightsWidth = 0
	a.heightsDoc = nil
	a.anchorBlock = 0
	a.anchorOffset = 0
	a.anchorPending = false
}

// syncBlockHeights makes sure there is exactly one height slot per block for
// the current content width. It reports whether the cache was rebuilt.
func (a *App) syncBlockHeights(width int) bool {
	if a.doc == nil {
		a.blockHeights = a.blockHeights[:0]
		return false
	}
	if a.heightsDoc != a.doc || a.heightsWidth != width || len(a.blockHeights) != len(a.doc.Blocks) {
		a.heightsDoc = a.doc
		a.heightsWidth = width
		a.blockHeights = make([]int, len(a.doc.Blocks))
		return true
	}
	return false
}

func (a *App) invalidateWordCount() {
	a.wordCountValid = false
}

// wordCountValue computes the document statistics at most once per change;
// the status bar asks for them on every frame.
func (a *App) wordCountValue() int {
	if !a.wordCountValid {
		a.wordCount = 0
		if a.doc != nil {
			a.wordCount = a.doc.WordCount()
		}
		a.wordCountValid = true
	}
	return a.wordCount
}

func (a *App) beginEditAt(idx, relX, relY, width, height int) {
	if a.doc == nil || idx < 0 || idx >= len(a.doc.Blocks) {
		return
	}
	if a.editIdx == idx {
		return
	}
	b := a.doc.Blocks[idx]
	a.editIdx = idx
	a.editRect = image.Rectangle{}
	a.blockEditor.SetText(b.Raw)
	caret := caretForClick(b.Raw, relX, relY, width, height)
	a.blockEditor.SetCaret(caret, caret)
	a.editFocusPending = true
	a.setStatus("正在编辑，点击空白处完成", 0)
	a.invalidate()
}

func caretForClick(raw string, relX, relY, width, height int) int {
	lines := strings.Split(raw, "\n")
	if height <= 0 {
		height = 1
	}
	relY = clamp(relY, 0, height-1)
	line := int(float64(relY) / float64(height) * float64(len(lines)))
	line = clamp(line, 0, len(lines)-1)
	off := 0
	for i := 0; i < line; i++ {
		off += runeCount(lines[i]) + 1
	}
	runes := []rune(lines[line])
	col := len(runes)
	if width > 1 && relX > 0 {
		col = int(float64(relX) / float64(width) * float64(len(runes)))
		col = clamp(col, 0, len(runes))
	} else if relX <= 0 {
		col = 0
	}
	return off + col
}

func (a *App) commitEdit() int {
	if a.editIdx < 0 || a.doc == nil || a.editIdx >= len(a.doc.Blocks) {
		a.editIdx = -1
		return 0
	}
	idx := a.editIdx
	raw := a.blockEditor.Text()
	b := a.doc.Blocks[idx]
	delta := 0
	if raw != b.Raw {
		a.dirty = true
		if strings.TrimSpace(raw) == "" && len(a.doc.Blocks) > 1 {
			// Emptying a block deletes it, so no "click here to write"
			// placeholder is left behind in the rendered document.
			a.doc.removeBlock(idx)
			delta = -1
		} else {
			delta = a.doc.splitBlock(idx, raw)
		}
		// Keep the reading anchor pointing at the same content when blocks
		// were inserted or removed above it.
		if delta != 0 && idx <= a.anchorBlock {
			a.anchorBlock = clamp(a.anchorBlock+delta, 0, len(a.doc.Blocks)-1)
		}
		a.invalidateHeights()
		a.invalidateWordCount()
		a.refreshFind()
		a.rebuildTOC()
		a.updateTitle()
	}
	a.editIdx = -1
	a.editRect = image.Rectangle{}
	a.editFocusPending = false
	a.setStatus("段落已渲染", 0)
	a.invalidate()
	return delta
}

func (a *App) toggleFind() {
	if !a.docOpen || a.doc == nil {
		return
	}
	a.commitEdit()
	if a.findOpen {
		a.findFocusWait = true
		a.findEd.SetCaret(0, a.findEd.Len())
		a.invalidate()
		return
	}
	a.findOpen = true
	a.findFocusWait = true
	if a.findQuery == "" {
		a.findEd.SetText("")
	} else {
		a.findEd.SetText(a.findQuery)
	}
	a.findEd.SetCaret(0, a.findEd.Len())
	a.updateFind(a.findQuery)
	a.invalidate()
}

func (a *App) closeFind() {
	a.findOpen = false
	a.findFocusWait = false
	a.pendingScrollBlock = -1
	a.invalidate()
}

func (a *App) toggleTOC() {
	if !a.docOpen || a.doc == nil {
		return
	}
	a.commitEdit()
	a.rebuildTOC()
	a.tocOpen = !a.tocOpen
	a.tocShown = -2 // force the outline to reveal the current heading
	a.invalidate()
}

// rebuildTOC refreshes the outline entries after the document changes. Entry
// indices stay stable as long as headings keep their relative order, so a
// pending jump survives an edit commit.
func (a *App) rebuildTOC() {
	a.tocEntries = nil
	if a.doc != nil {
		a.tocEntries = a.doc.TOC()
	}
	for cap(a.tocClicks) < len(a.tocEntries) {
		a.tocClicks = append(a.tocClicks, widget.Clickable{})
	}
	a.tocClicks = a.tocClicks[:len(a.tocEntries)]
	// Heading positions are only valid for the block layout they were
	// measured against; the viewport refills them on the next pass.
	a.tocTops = a.tocTops[:0]
	a.tocCurrent = -1
	a.tocShown = -2
}

func (a *App) updateFind(q string) {
	a.findQuery = q
	a.findMatches = a.findMatches[:0]
	a.findCur = -1
	if q == "" || a.doc == nil {
		a.pendingScrollBlock = -1
		a.invalidate()
		return
	}
	qr := []rune(strings.ToLower(q))
	if len(qr) == 0 {
		a.pendingScrollBlock = -1
		a.invalidate()
		return
	}
	for bi, b := range a.doc.Blocks {
		if b.Display == nil {
			b.Display = buildDisplay(b)
		}
		plain := []rune(b.Display.Plain)
		lower := make([]rune, len(plain))
		for i, r := range plain {
			lower[i] = unicode.ToLower(r)
		}
		for i := 0; i+len(qr) <= len(lower); {
			match := true
			for j := range qr {
				if lower[i+j] != qr[j] {
					match = false
					break
				}
			}
			if match {
				a.findMatches = append(a.findMatches, findMatch{Block: bi, Start: i, End: i + len(qr)})
				i += len(qr)
			} else {
				i++
			}
		}
	}
	if len(a.findMatches) > 0 {
		a.findCur = 0
		a.pendingScrollBlock = a.findMatches[0].Block
	} else {
		a.pendingScrollBlock = -1
	}
	a.invalidate()
}

func (a *App) refreshFind() {
	if !a.findOpen || a.findQuery == "" {
		return
	}
	old := a.findCur
	a.updateFind(a.findQuery)
	if len(a.findMatches) == 0 {
		return
	}
	if old >= 0 && old < len(a.findMatches) {
		a.findCur = old
		a.pendingScrollBlock = a.findMatches[old].Block
	}
}

func (a *App) findNext(dir int) {
	if !a.findOpen || len(a.findMatches) == 0 {
		return
	}
	a.findCur = (a.findCur + dir + len(a.findMatches)) % len(a.findMatches)
	a.pendingScrollBlock = a.findMatches[a.findCur].Block
	a.setStatus(fmt.Sprintf("第 %d/%d 个结果", a.findCur+1, len(a.findMatches)), 0)
	a.invalidate()
}

func (a *App) blockMatches(idx int) []renderMatch {
	if !a.findOpen || len(a.findMatches) == 0 {
		return nil
	}
	var out []renderMatch
	for i, m := range a.findMatches {
		if m.Block == idx {
			out = append(out, renderMatch{start: m.Start, end: m.End, current: i == a.findCur})
		}
	}
	return out
}

func (a *App) handleViewportClick(gtx layout.Context) {
	if a.dialogBusy {
		for {
			if _, ok := a.viewportClick.Update(gtx.Source); !ok {
				break
			}
		}
		return
	}
	for {
		ev, ok := a.viewportClick.Update(gtx.Source)
		if !ok {
			break
		}
		if ev.Kind != gesture.KindPress || a.modal != nil {
			continue
		}
		p := ev.Position
		if a.editIdx >= 0 && p.In(a.editRect) {
			continue
		}
		idx := -1
		for i, r := range a.blockRects {
			if p.In(r) {
				idx = i
				break
			}
		}
		wasEditing := a.editIdx >= 0
		oldEdit := a.editIdx
		var targetRect image.Rectangle
		if idx >= 0 && idx < len(a.blockRects) {
			targetRect = a.blockRects[idx]
		}
		delta := 0
		if wasEditing {
			delta = a.commitEdit()
		}
		if idx >= 0 {
			if wasEditing && idx > oldEdit {
				idx += delta
			}
			if idx < len(a.doc.Blocks) {
				a.beginEditAt(idx, p.X-targetRect.Min.X, p.Y-targetRect.Min.Y, targetRect.Dx(), targetRect.Dy())
			}
		}
		if wasEditing || idx >= 0 {
			gtx.Execute(op.InvalidateCmd{})
		}
	}
}

func (a *App) processFindEditor(gtx layout.Context) {
	for {
		ev, ok := a.findEd.Update(gtx)
		if !ok {
			break
		}
		switch ev.(type) {
		case widget.ChangeEvent:
			a.updateFind(a.findEd.Text())
		case widget.SubmitEvent:
			a.findNext(1)
		}
	}
}

// Keep the scroll wheel from doing surprising things when the pointer is far
// outside the document area. The gesture range is intentionally generous.
func (a *App) scrollRange() pointer.ScrollRange {
	return pointer.ScrollRange{Min: -(1 << 30), Max: 1 << 30}
}
