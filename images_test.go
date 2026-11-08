package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractMarkdownImage(t *testing.T) {
	images, text, center := extractImages("before ![a picture](pics/a.png)\nafter")
	if len(images) != 1 {
		t.Fatalf("images = %d", len(images))
	}
	if images[0].Src != "pics/a.png" || images[0].Alt != "a picture" {
		t.Fatalf("image = %+v", images[0])
	}
	if center {
		t.Fatal("unexpected center")
	}
	if strings.TrimSpace(text) != "before\n\nafter" && strings.TrimSpace(text) != "before\nafter" {
		// Whitespace normalization is not important; markup must be gone.
		if strings.Contains(text, "![") || strings.Contains(text, "pics/a.png") {
			t.Fatalf("markup leaked into text: %q", text)
		}
	}
}

func TestExtractHTMLImages(t *testing.T) {
	raw := "<p align=\"center\">\n  <img src=\"./startpage.png\" alt=\"start\" width=\"48%\">\n  <img src='./homepage.png' alt='home' width='48%'>\n</p>"
	images, text, center := extractImages(raw)
	if len(images) != 2 {
		t.Fatalf("images = %d: %+v", len(images), images)
	}
	if images[0].Src != "./startpage.png" || images[0].WidthPct != 48 {
		t.Fatalf("first = %+v", images[0])
	}
	if images[1].Src != "./homepage.png" || images[1].WidthPct != 48 {
		t.Fatalf("second = %+v", images[1])
	}
	if !center {
		t.Fatal("center should be true")
	}
	if strings.TrimSpace(text) != "" {
		t.Fatalf("text = %q", text)
	}
}

func TestResolveImageSrc(t *testing.T) {
	a := &App{path: filepath.Join("/tmp", "docs", "note.md")}
	got, remote, err := a.resolveImageSrc("./pics/a.png")
	if err != nil || remote {
		t.Fatalf("resolve: %q remote=%v err=%v", got, remote, err)
	}
	want := filepath.Join("/tmp", "docs", "pics", "a.png")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if _, remote, err := a.resolveImageSrc("https://example.com/a.png"); err != nil || !remote {
		t.Fatalf("remote resolve: remote=%v err=%v", remote, err)
	}
}

func TestStripHTMLText(t *testing.T) {
	got, ok := stripHTMLToText(`<p align="center"><strong>自愿赠与，感谢支持！</strong></p>`)
	if !ok {
		t.Fatal("expected html text")
	}
	if got != "**自愿赠与，感谢支持！**" {
		t.Fatalf("got %q", got)
	}
	if _, ok := stripHTMLToText("plain markdown"); ok {
		t.Fatal("plain text should not be treated as html")
	}
}

func TestExtractImagesHTMLOnly(t *testing.T) {
	doc := parseDocument("<p align=\"center\"><strong>caption</strong></p>\n")
	if len(doc.Blocks) != 1 {
		t.Fatalf("blocks = %d", len(doc.Blocks))
	}
	plain := doc.Blocks[0].Display.Plain
	if plain != "caption" {
		t.Fatalf("plain = %q", plain)
	}
}

func TestImageLoadAndCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dot.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()
	a := &App{path: filepath.Join(dir, "note.md")}
	entry, resolved := a.imageFor("dot.png")
	if entry.err != nil {
		t.Fatalf("load error: %v (%s)", entry.err, resolved)
	}
	if entry.img.Bounds().Dx() != 2 || entry.img.Bounds().Dy() != 2 {
		t.Fatalf("bounds = %v", entry.img.Bounds())
	}
	if again, _ := a.imageFor("dot.png"); again != entry {
		t.Fatal("image should be cached")
	}
}
