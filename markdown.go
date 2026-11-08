package main

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

type blockKind int

const (
	kindParagraph blockKind = iota
	kindHeading
	kindCode
	kindQuote
	kindList
	kindTable
	kindRule
)

type Block struct {
	Raw     string
	Kind    blockKind
	Level   int
	Lang    string
	Display *blockDisplay
}

type Document struct {
	Blocks []*Block
	// Gaps holds the exact source text before each block and after the last
	// block. Index i is the gap before Blocks[i]; the final element is the
	// trailing gap. Keeping gaps separate makes saving lossless even when the
	// user has not touched a region.
	Gaps []string
	EOL  string
}

type inlineStyle struct {
	Bold   bool
	Italic bool
	Code   bool
	Link   bool
	Image  bool
	Strike bool
}

type inlineSeg struct {
	Text  string
	Start int // rune offset in the block's Plain string
	Style inlineStyle
}

type listDisplay struct {
	Indent  int
	Ordered bool
	Number  string
	Task    int // -1: not a task, 0: unchecked, 1: checked
	Segs    []inlineSeg
}

type tableCell struct {
	Plain string
	Segs  []inlineSeg
}

type tableDisplay struct {
	Header []tableCell
	Rows   [][]tableCell
	Aligns []string
}

type blockDisplay struct {
	Plain    string
	Segs     []inlineSeg
	Code     string
	CodeLang string
	List     *listDisplay
	Table    *tableDisplay
	Quote    bool
	Images   []docImage
	Center   bool
}

var (
	reFence = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})(.*)$")
	reATX   = regexp.MustCompile(`^ {0,3}(#{1,6})(?:\s+(.*?))?\s*#*\s*$`)
	reSetex = regexp.MustCompile(`^ {0,3}(=+|-+)\s*$`)
	reHR    = regexp.MustCompile(`^ {0,3}((\*\s*){3,}|(-\s*){3,}|(_\s*){3,})$`)
	reList  = regexp.MustCompile(`^(\s*)([-+*]|\d{1,9}[.)])(\s+)(.*)$`)
	reTable = regexp.MustCompile(`^ {0,3}\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)+\|?\s*$`)
	reTask  = regexp.MustCompile(`^\[([ xX])\]\s+(.*)$`)
	reQuote = regexp.MustCompile(`^ {0,3}>\s?(.*)$`)

	inlineParser = goldmark.New(goldmark.WithExtensions(extension.GFM))
)

func runeCount(s string) int {
	return utf8.RuneCountInString(s)
}

func splitSourceLines(src string) ([]string, []int) {
	lines := make([]string, 0, 64)
	offsets := make([]int, 0, 64)
	start := 0
	offsets = append(offsets, 0)
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			lines = append(lines, src[start:i])
			start = i + 1
			offsets = append(offsets, start)
		}
	}
	lines = append(lines, src[start:])
	return lines, offsets
}

func parseDocument(src string) *Document {
	eol := "\n"
	if strings.Contains(src, "\r\n") {
		eol = "\r\n"
		src = strings.ReplaceAll(src, "\r\n", "\n")
	} else {
		src = strings.ReplaceAll(src, "\r", "\n")
	}
	lines, offsets := splitSourceLines(src)
	doc := &Document{EOL: eol}
	gapStart := 0
	i := 0
	for i < len(lines) {
		if isBlankLine(lines[i]) {
			i++
			continue
		}
		start := i
		end := blockEnd(lines, i)
		if end <= start {
			end = start + 1
		}
		for end > start && isBlankLine(lines[end-1]) {
			end--
		}
		rawStart := offsets[start]
		rawEnd := len(src)
		if end < len(offsets) {
			rawEnd = offsets[end]
		}
		if rawEnd > rawStart && src[rawEnd-1] == '\n' {
			rawEnd--
		}
		doc.Gaps = append(doc.Gaps, src[gapStart:rawStart])
		raw := src[rawStart:rawEnd]
		doc.Blocks = append(doc.Blocks, newBlock(raw))
		gapStart = rawEnd
		i = end
	}
	if len(doc.Blocks) == 0 {
		// An empty document still needs an editable block. Everything the file
		// contained (possibly blank lines) becomes leading whitespace.
		doc.Gaps = []string{src, ""}
		doc.Blocks = []*Block{newBlock("")}
	} else {
		doc.Gaps = append(doc.Gaps, src[gapStart:])
	}
	return doc
}

func isBlankLine(s string) bool {
	return strings.TrimSpace(s) == ""
}

func fenceInfo(line string) (string, string, bool) {
	m := reFence.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	return m[1], strings.TrimSpace(m[2]), true
}

func listInfo(line string) (indent int, marker, content string, ok bool) {
	m := reList.FindStringSubmatch(line)
	if m == nil {
		return 0, "", "", false
	}
	for _, r := range m[1] {
		if r == '\t' {
			indent += 4
		} else {
			indent++
		}
	}
	return indent, m[2], m[4], true
}

func isTableStart(lines []string, i int) bool {
	if i+1 >= len(lines) {
		return false
	}
	if !strings.Contains(lines[i], "|") {
		return false
	}
	return reTable.MatchString(lines[i+1])
}

func isQuoteLine(line string) bool {
	return reQuote.MatchString(line)
}

func isIndentedCodeLine(line string) bool {
	return strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")
}

func isParagraphBreaker(lines []string, i int) bool {
	line := lines[i]
	if isBlankLine(line) {
		return true
	}
	if _, _, ok := fenceInfo(line); ok {
		return true
	}
	if reATX.MatchString(line) || reHR.MatchString(line) || isQuoteLine(line) {
		return true
	}
	if _, _, _, ok := listInfo(line); ok {
		return true
	}
	if isTableStart(lines, i) {
		return true
	}
	return false
}

func blockEnd(lines []string, start int) int {
	line := lines[start]

	// Fenced code may contain blank lines and any other block syntax.
	if fence, _, ok := fenceInfo(line); ok {
		fenceChar := fence[0]
		fenceLen := len(fence)
		for i := start + 1; i < len(lines); i++ {
			trim := strings.TrimSpace(lines[i])
			if len(trim) >= fenceLen && trim[0] == fenceChar && strings.Count(trim, string(fenceChar)) >= fenceLen && strings.HasPrefix(trim, strings.Repeat(string(fenceChar), fenceLen)) {
				return i + 1
			}
		}
		return len(lines)
	}

	if reATX.MatchString(line) || reHR.MatchString(line) {
		return start + 1
	}

	// Setext headings are a paragraph line followed by an underline.
	if start+1 < len(lines) && reSetex.MatchString(lines[start+1]) && !isBlankLine(line) && !reList.MatchString(line) {
		return start + 2
	}

	if isTableStart(lines, start) {
		i := start + 2
		for i < len(lines) && !isBlankLine(lines[i]) && strings.Contains(lines[i], "|") {
			i++
		}
		return i
	}

	if isQuoteLine(line) {
		i := start + 1
		for i < len(lines) && isQuoteLine(lines[i]) {
			i++
		}
		return i
	}

	if _, _, _, ok := listInfo(line); ok {
		i := start + 1
		for i < len(lines) {
			if isBlankLine(lines[i]) {
				break
			}
			// Nested and following list items are separate editable blocks.
			// This keeps each bullet independently clickable and preserves
			// the visual indentation of nested lists.
			if _, _, _, ok := listInfo(lines[i]); ok {
				break
			}
			if reATX.MatchString(lines[i]) || reHR.MatchString(lines[i]) {
				break
			}
			if _, _, ok := fenceInfo(lines[i]); ok {
				break
			}
			i++
		}
		return i
	}

	if isIndentedCodeLine(line) {
		i := start + 1
		for i < len(lines) && isIndentedCodeLine(lines[i]) {
			i++
		}
		return i
	}

	// Ordinary paragraph: stop at a blank line or a block that cannot be a
	// lazy continuation.
	i := start + 1
	for i < len(lines) {
		if isParagraphBreaker(lines, i) {
			break
		}
		i++
	}
	return i
}

func newBlock(raw string) *Block {
	b := &Block{Raw: raw}
	b.Kind, b.Level, b.Lang = classifyBlock(raw)
	b.Display = buildDisplay(b)
	return b
}

func classifyBlock(raw string) (blockKind, int, string) {
	if strings.TrimSpace(raw) == "" {
		return kindParagraph, 0, ""
	}
	lines := strings.Split(raw, "\n")
	first := lines[0]
	if _, lang, ok := fenceInfo(first); ok {
		return kindCode, 0, lang
	}
	if m := reATX.FindStringSubmatch(first); m != nil {
		return kindHeading, len(m[1]), ""
	}
	if reHR.MatchString(first) {
		return kindRule, 0, ""
	}
	if len(lines) > 1 && reSetex.MatchString(lines[1]) {
		if strings.HasPrefix(strings.TrimSpace(lines[1]), "=") {
			return kindHeading, 1, ""
		}
		return kindHeading, 2, ""
	}
	if len(lines) > 1 && strings.Contains(first, "|") && reTable.MatchString(lines[1]) {
		return kindTable, 0, ""
	}
	if isQuoteLine(first) {
		return kindQuote, 0, ""
	}
	if _, _, _, ok := listInfo(first); ok {
		return kindList, 0, ""
	}
	if isIndentedCodeLine(first) {
		return kindCode, 0, ""
	}
	return kindParagraph, 0, ""
}

func headingText(raw string) (int, string) {
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return 0, ""
	}
	if m := reATX.FindStringSubmatch(lines[0]); m != nil {
		text := ""
		if len(m) > 2 {
			text = strings.TrimSpace(m[2])
		}
		text = strings.TrimSpace(strings.TrimRight(text, "#"))
		return len(m[1]), text
	}
	if len(lines) > 1 && reSetex.MatchString(lines[1]) {
		level := 2
		if strings.HasPrefix(strings.TrimSpace(lines[1]), "=") {
			level = 1
		}
		return level, strings.TrimSpace(lines[0])
	}
	return 0, strings.TrimSpace(raw)
}

func codeText(raw string) (string, string) {
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return "", ""
	}
	first := lines[0]
	if fence, lang, ok := fenceInfo(first); ok {
		lang = strings.TrimSpace(strings.Trim(lang, "{}"))
		body := lines[1:]
		if len(body) > 0 {
			trim := strings.TrimSpace(body[len(body)-1])
			if len(trim) >= len(fence) && strings.HasPrefix(trim, strings.Repeat(string(fence[0]), len(fence))) {
				body = body[:len(body)-1]
			}
		}
		return strings.Join(body, "\n"), lang
	}
	body := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "\t") {
			body = append(body, line[1:])
		} else if strings.HasPrefix(line, "    ") {
			body = append(body, line[4:])
		} else {
			body = append(body, line)
		}
	}
	return strings.Join(body, "\n"), ""
}

func quoteText(raw string) string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if m := reQuote.FindStringSubmatch(line); m != nil {
			out = append(out, m[1])
		} else {
			out = append(out, strings.TrimSpace(line))
		}
	}
	return strings.Join(out, "\n")
}

func splitTableRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	if line == "" {
		return []string{""}
	}
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func parseTableDisplay(raw string) *tableDisplay {
	lines := strings.Split(raw, "\n")
	if len(lines) < 2 {
		return nil
	}
	headerText := splitTableRow(lines[0])
	sepText := splitTableRow(lines[1])
	aligns := make([]string, len(sepText))
	for i, s := range sepText {
		s = strings.TrimSpace(s)
		switch {
		case strings.HasPrefix(s, ":") && strings.HasSuffix(s, ":"):
			aligns[i] = "center"
		case strings.HasSuffix(s, ":"):
			aligns[i] = "right"
		default:
			aligns[i] = "left"
		}
	}
	makeCell := func(s string) tableCell {
		segs := parseInlineSegs(s)
		plain, segs := assignPlain(segs)
		return tableCell{Plain: plain, Segs: segs}
	}
	td := &tableDisplay{Aligns: aligns}
	for _, h := range headerText {
		td.Header = append(td.Header, makeCell(h))
	}
	for _, line := range lines[2:] {
		if isBlankLine(line) {
			continue
		}
		cells := splitTableRow(line)
		row := make([]tableCell, 0, len(cells))
		for _, c := range cells {
			row = append(row, makeCell(c))
		}
		td.Rows = append(td.Rows, row)
	}
	return td
}

func tablePlain(td *tableDisplay) string {
	var sb strings.Builder
	addCell := func(cell *tableCell) {
		base := runeCount(sb.String())
		for i := range cell.Segs {
			cell.Segs[i].Start += base
		}
		sb.WriteString(cell.Plain)
		sb.WriteByte('\t')
	}
	for i := range td.Header {
		addCell(&td.Header[i])
	}
	sb.WriteByte('\n')
	for ri := range td.Rows {
		for ci := range td.Rows[ri] {
			addCell(&td.Rows[ri][ci])
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func buildDisplay(b *Block) *blockDisplay {
	switch b.Kind {
	case kindCode:
		code, lang := codeText(b.Raw)
		return &blockDisplay{Plain: code, Code: code, CodeLang: lang}
	case kindHeading:
		_, text := headingText(b.Raw)
		segs := parseInlineSegs(text)
		plain, segs := assignPlain(segs)
		return &blockDisplay{Plain: plain, Segs: segs}
	case kindQuote:
		segs := parseInlineSegs(quoteText(b.Raw))
		plain, segs := assignPlain(segs)
		return &blockDisplay{Plain: plain, Segs: segs, Quote: true}
	case kindList:
		return buildListDisplay(b.Raw)
	case kindTable:
		td := parseTableDisplay(b.Raw)
		if td == nil {
			return &blockDisplay{}
		}
		return &blockDisplay{Plain: tablePlain(td), Table: td}
	case kindRule:
		return &blockDisplay{}
	default:
		if images, textOnly, center := extractImages(b.Raw); len(images) > 0 {
			segs := parseInlineSegs(textOnly)
			plain, segs := assignPlain(segs)
			return &blockDisplay{Plain: plain, Segs: segs, Images: images, Center: center}
		}
		if text, ok := stripHTMLToText(b.Raw); ok {
			segs := parseInlineSegs(text)
			plain, segs := assignPlain(segs)
			return &blockDisplay{Plain: plain, Segs: segs}
		}
		segs := parseInlineSegs(b.Raw)
		plain, segs := assignPlain(segs)
		return &blockDisplay{Plain: plain, Segs: segs}
	}
}

func buildListDisplay(raw string) *blockDisplay {
	lines := strings.Split(raw, "\n")
	indent, marker, content, _ := listInfo(lines[0])
	ld := &listDisplay{Indent: indent, Task: -1}
	ld.Ordered = marker != "" && marker[0] >= '0' && marker[0] <= '9'
	if ld.Ordered {
		ld.Number = marker
	}
	// Continuation lines are rendered as part of the item. The common
	// indentation is removed so wrapping looks natural.
	parts := []string{content}
	for _, line := range lines[1:] {
		trimmed := line
		cut := 0
		for cut < len(trimmed) && cut < indent+2 && (trimmed[cut] == ' ' || trimmed[cut] == '\t') {
			cut++
		}
		trimmed = trimmed[cut:]
		parts = append(parts, trimmed)
	}
	joined := strings.Join(parts, "\n")
	if m := reTask.FindStringSubmatch(joined); m != nil {
		if m[1] == " " {
			ld.Task = 0
		} else {
			ld.Task = 1
		}
		joined = m[2]
	}
	segs := parseInlineSegs(joined)
	plain, segs := assignPlain(segs)
	ld.Segs = segs
	return &blockDisplay{Plain: plain, Segs: segs, List: ld}
}

func assignPlain(segs []inlineSeg) (string, []inlineSeg) {
	var sb strings.Builder
	off := 0
	for i := range segs {
		segs[i].Start = off
		sb.WriteString(segs[i].Text)
		off += runeCount(segs[i].Text)
	}
	list := segs[:0]
	for _, seg := range segs {
		if seg.Text == "" {
			continue
		}
		if len(list) > 0 && list[len(list)-1].Style == seg.Style && list[len(list)-1].Start+runeCount(list[len(list)-1].Text) == seg.Start {
			list[len(list)-1].Text += seg.Text
		} else {
			list = append(list, seg)
		}
	}
	return sb.String(), list
}

func parseInlineSegs(src string) []inlineSeg {
	source := []byte(src)
	doc := inlineParser.Parser().Parse(text.NewReader(source))
	var out []inlineSeg
	add := func(s string, st inlineStyle) {
		if s != "" {
			out = append(out, inlineSeg{Text: s, Style: st})
		}
	}
	var walk func(n ast.Node, st inlineStyle)
	walk = func(n ast.Node, st inlineStyle) {
		switch n := n.(type) {
		case *ast.Text:
			add(string(n.Segment.Value(source)), st)
			if n.SoftLineBreak() {
				add(" ", st)
			}
			if n.HardLineBreak() {
				add("\n", st)
			}
		case *ast.String:
			add(string(n.Value), st)
		case *ast.CodeSpan:
			st.Code = true
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				walk(c, st)
			}
		case *ast.Emphasis:
			if n.Level >= 2 {
				st.Bold = true
			} else {
				st.Italic = true
			}
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				walk(c, st)
			}
		case *ast.Link:
			st.Link = true
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				walk(c, st)
			}
		case *ast.AutoLink:
			st.Link = true
			add(string(n.Label(source)), st)
		case *ast.Image:
			st.Image = true
			st.Link = true
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				walk(c, st)
			}
		case *extast.Strikethrough:
			st.Strike = true
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				walk(c, st)
			}
		case *ast.RawHTML:
			if n.Segments != nil {
				var b strings.Builder
				for i := 0; i < n.Segments.Len(); i++ {
					seg := n.Segments.At(i)
					b.Write(seg.Value(source))
				}
				tag := strings.ToLower(b.String())
				if strings.HasPrefix(tag, "<br") {
					add("\n", st)
				}
				// Other inline HTML tags are not rendered as literal source.
			}
		case *ast.ThematicBreak:
			// handled at block level
		default:
			if n.Type() == ast.TypeBlock && n.Kind() == ast.KindParagraph {
				if len(out) > 0 {
					add("\n", inlineStyle{})
				}
			}
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				walk(c, st)
			}
		}
	}
	for c := doc.FirstChild(); c != nil; c = c.NextSibling() {
		walk(c, inlineStyle{})
	}
	if len(out) == 0 && src != "" {
		add(src, inlineStyle{})
	}
	return out
}

// splitBlock replaces one edited block by the blocks parsed from its new
// source text. This is what makes adding a blank line inside an edited block
// behave like adding a new paragraph, instead of keeping everything glued
// together until the file is reopened.
func (d *Document) splitBlock(idx int, raw string) int {
	if idx < 0 || idx >= len(d.Blocks) {
		return 0
	}
	sub := parseDocument(raw)
	n := len(sub.Blocks)
	if n == 0 {
		return 0
	}
	oldBefore := ""
	if idx < len(d.Gaps) {
		oldBefore = d.Gaps[idx]
	}
	oldAfter := ""
	if idx+1 < len(d.Gaps) {
		oldAfter = d.Gaps[idx+1]
	}
	newGaps := make([]string, 0, len(d.Gaps)-1+n)
	newGaps = append(newGaps, d.Gaps[:idx]...)
	newGaps = append(newGaps, oldBefore+sub.Gaps[0])
	if n > 1 {
		newGaps = append(newGaps, sub.Gaps[1:n]...)
	}
	newGaps = append(newGaps, sub.Gaps[n]+oldAfter)
	if idx+2 < len(d.Gaps) {
		newGaps = append(newGaps, d.Gaps[idx+2:]...)
	}
	newBlocks := make([]*Block, 0, len(d.Blocks)-1+n)
	newBlocks = append(newBlocks, d.Blocks[:idx]...)
	newBlocks = append(newBlocks, sub.Blocks...)
	newBlocks = append(newBlocks, d.Blocks[idx+1:]...)
	d.Blocks = newBlocks
	d.Gaps = newGaps
	return n - 1
}

func (d *Document) Source() string {
	var sb strings.Builder
	for i, b := range d.Blocks {
		if i < len(d.Gaps) {
			sb.WriteString(d.Gaps[i])
		}
		sb.WriteString(b.Raw)
	}
	if len(d.Gaps) > len(d.Blocks) {
		sb.WriteString(d.Gaps[len(d.Blocks)])
	}
	src := sb.String()
	if d.EOL == "\r\n" {
		src = strings.ReplaceAll(src, "\n", "\r\n")
	}
	return src
}

func (d *Document) PlainText() string {
	var sb strings.Builder
	for i, b := range d.Blocks {
		if b.Display != nil {
			sb.WriteString(b.Display.Plain)
		}
		if i != len(d.Blocks)-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func (d *Document) WordCount() int {
	plain := []rune(d.PlainText())
	count := 0
	inWord := false
	for _, r := range plain {
		if isCJK(r) {
			count++
			inWord = false
			continue
		}
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			inWord = false
			continue
		}
		if !inWord {
			count++
			inWord = true
		}
	}
	return count
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x3040 && r <= 0x30FF) ||
		(r >= 0xAC00 && r <= 0xD7AF) ||
		(r >= 0xF900 && r <= 0xFAFF)
}
