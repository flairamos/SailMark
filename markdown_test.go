package main

import (
	"os"
	"strings"
	"testing"
)

func TestCodeBlockText(t *testing.T) {
	raw := "```go\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```"
	b := newBlock(raw)
	if b.Kind != kindCode {
		t.Fatalf("kind = %v", b.Kind)
	}
	if b.Display.Code != "func main() {\n    fmt.Println(\"Hello\")\n}" {
		t.Fatalf("code = %q", b.Display.Code)
	}
	if b.Display.CodeLang != "go" {
		t.Fatalf("lang = %q", b.Display.CodeLang)
	}
}

func TestDocumentRoundTrip(t *testing.T) {
	cases := []string{
		"",
		"# Title\n\nHello **world**.\n",
		"a\n\n- one\n- two\n\n> quote\n\n```go\nfmt.Println(1)\n```\n",
		"| a | b |\n| --- | --- |\n| 1 | 2 |\n",
	}
	for _, src := range cases {
		doc := parseDocument(src)
		if got := doc.Source(); got != src {
			t.Fatalf("round trip:\nwant %q\ngot  %q", src, got)
		}
	}
}

func TestBlockKinds(t *testing.T) {
	src := "# H1\n\npara\nline2\n\n- item\n- [x] task\n\n> quote\n\n```go\ncode\n```\n\n---\n\n| a | b |\n| --- | --- |\n| 1 | 2 |"
	doc := parseDocument(src)
	var kinds []blockKind
	for _, b := range doc.Blocks {
		kinds = append(kinds, b.Kind)
	}
	want := []blockKind{kindHeading, kindParagraph, kindList, kindList, kindQuote, kindCode, kindRule, kindTable}
	if len(kinds) != len(want) {
		t.Fatalf("got %d blocks: %v", len(kinds), kinds)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("block %d kind = %v want %v", i, kinds[i], want[i])
		}
	}
}

func TestInlinePlain(t *testing.T) {
	doc := parseDocument("a **bold** and `code` and [link](x)\n")
	got := doc.Blocks[0].Display.Plain
	if !strings.Contains(got, "bold") || !strings.Contains(got, "code") || !strings.Contains(got, "link") {
		t.Fatalf("plain = %q", got)
	}
	if strings.Contains(got, "**") || strings.Contains(got, "](") {
		t.Fatalf("markdown syntax leaked into plain: %q", got)
	}
}

func TestCaretForClick(t *testing.T) {
	if got := caretForClick("0123456789", 500, 10, 1000, 30); got != 5 {
		t.Fatalf("single line caret = %d, want 5", got)
	}
	got := caretForClick("abc\ndefgh", 0, 45, 100, 60)
	if got != 4 {
		t.Fatalf("second line caret = %d, want 4", got)
	}
}

func TestTablePlain(t *testing.T) {
	doc := parseDocument("| name | value |\n| --- | --- |\n| alpha | 42 |\n")
	if len(doc.Blocks) != 1 || doc.Blocks[0].Kind != kindTable {
		t.Fatalf("unexpected blocks: %#v", doc.Blocks)
	}
	plain := doc.Blocks[0].Display.Plain
	if !strings.Contains(plain, "alpha") || !strings.Contains(plain, "42") {
		t.Fatalf("table plain = %q", plain)
	}
}

func TestInlineStyles(t *testing.T) {
	segs := parseInlineSegs("~~strike~~ **bold** *italic* `code` [link](x)")
	var strike, bold, italic, code, link bool
	for _, s := range segs {
		strike = strike || s.Style.Strike
		bold = bold || s.Style.Bold
		italic = italic || s.Style.Italic
		code = code || s.Style.Code
		link = link || s.Style.Link
	}
	if !(strike && bold && italic && code && link) {
		t.Fatalf("styles strike=%v bold=%v italic=%v code=%v link=%v segs=%+v", strike, bold, italic, code, link, segs)
	}
}

func TestCRLFRoundTrip(t *testing.T) {
	src := "# Title\r\n\r\nHello\r\n"
	doc := parseDocument(src)
	if got := doc.Source(); got != src {
		t.Fatalf("CRLF round trip: want %q got %q", src, got)
	}
}

func TestNestedListSplit(t *testing.T) {
	doc := parseDocument("- parent\n  - child\n- next\n")
	if len(doc.Blocks) != 3 {
		t.Fatalf("got %d blocks, want 3", len(doc.Blocks))
	}
	if doc.Blocks[1].Display.List == nil || doc.Blocks[1].Display.List.Indent != 2 {
		t.Fatalf("nested block indent = %#v", doc.Blocks[1].Display.List)
	}
	if got := doc.Source(); got != "- parent\n  - child\n- next\n" {
		t.Fatalf("source = %q", got)
	}
}

func TestSplitBlock(t *testing.T) {
	doc := parseDocument("para\n\nnext\n")
	before := doc.Source()
	delta := doc.splitBlock(0, "line one\n\nline two")
	if delta != 1 {
		t.Fatalf("delta = %d, want 1", delta)
	}
	if len(doc.Blocks) != 3 {
		t.Fatalf("blocks = %d, want 3", len(doc.Blocks))
	}
	want := "line one\n\nline two\n\nnext\n"
	if got := doc.Source(); got != want {
		t.Fatalf("source = %q, want %q (before %q)", got, want, before)
	}
}

func TestSplitBlockPreservesGaps(t *testing.T) {
	doc := parseDocument("\n\na\n\n\nb\n\n")
	doc.splitBlock(0, "a\n\nnew")
	want := "\n\na\n\nnew\n\n\nb\n\n"
	if got := doc.Source(); got != want {
		t.Fatalf("source = %q, want %q", got, want)
	}
}

func TestDialogOpenResult(t *testing.T) {
	dir := t.TempDir()
	path := dir + string(os.PathSeparator) + "note.md"
	if err := os.WriteFile(path, []byte("# hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	a := &App{
		editIdx:            -1,
		findCur:            -1,
		pendingScrollBlock: -1,
		dialogCh:           make(chan dialogResult, 1),
		dialogBusy:         true,
	}
	a.dialogCh <- dialogResult{kind: "open", path: path}
	a.pollDialog()
	if !a.docOpen || a.path != path || a.doc == nil {
		t.Fatalf("dialog open result not applied: open=%v path=%q doc=%v", a.docOpen, a.path, a.doc)
	}
	if a.dialogBusy {
		t.Fatal("dialogBusy should be cleared")
	}
}

func TestSaveDialogResult(t *testing.T) {
	dir := t.TempDir()
	a := &App{
		editIdx:            -1,
		findCur:            -1,
		pendingScrollBlock: -1,
		doc:                parseDocument("# note\n"),
		docOpen:            true,
		dirty:              true,
	}
	a.finishSavePath(dir + string(os.PathSeparator) + "out")
	if a.path == "" || !strings.HasSuffix(a.path, ".md") {
		t.Fatalf("path = %q", a.path)
	}
	data, err := os.ReadFile(a.path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# note\n" {
		t.Fatalf("saved %q", data)
	}
	if a.dirty {
		t.Fatal("document should be clean after save")
	}
}
