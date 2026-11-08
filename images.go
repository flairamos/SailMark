package main

import (
	"fmt"
	"html"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/yuin/goldmark/ast"
	mdtext "github.com/yuin/goldmark/text"
)

type docImage struct {
	Src      string
	Alt      string
	WidthPx  int
	WidthPct int
}

type imageEntry struct {
	img image.Image
	op  paint.ImageOp
	err error
}

var (
	htmlImgTagRe   = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	htmlAttrRe     = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*("([^"]*)"|'([^']*)'|([^\s"'>]+))`)
	htmlCommentRe  = regexp.MustCompile(`(?is)<!--.*?-->`)
	htmlTagStripRe = regexp.MustCompile(`(?is)<[^>]*>`)
	mdImageStripRe = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	centerAlignRe  = regexp.MustCompile(`(?is)(align\s*=\s*["']?\s*center|text-align\s*:\s*center)`)
	htmlStrongRe   = regexp.MustCompile(`(?is)<strong>(.*?)</strong>`)
	htmlBoldRe     = regexp.MustCompile(`(?is)<b>(.*?)</b>`)
	htmlEmRe       = regexp.MustCompile(`(?is)<em>(.*?)</em>`)
	htmlItalicRe   = regexp.MustCompile(`(?is)<i>(.*?)</i>`)
	htmlCodeRe     = regexp.MustCompile(`(?is)<code>(.*?)</code>`)
	htmlBRRe       = regexp.MustCompile(`(?is)<br\s*/?>`)
)

// extractImages finds both Markdown images and raw HTML <img> tags in a
// paragraph. It also returns the remaining textual content, so the rendered
// paragraph does not show image markup as code.
func extractImages(raw string) ([]docImage, string, bool) {
	source := []byte(raw)
	doc := inlineParser.Parser().Parse(mdtext.NewReader(source))
	var images []docImage
	var htmlText strings.Builder

	addHTML := func(s string) {
		for _, tag := range htmlImgTagRe.FindAllString(s, -1) {
			if img, ok := parseHTMLImage(tag); ok {
				images = append(images, img)
			}
		}
	}

	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		switch n := n.(type) {
		case *ast.Image:
			images = append(images, docImage{
				Src: string(n.Destination),
				Alt: nodeAltText(n, source),
			})
		case *ast.RawHTML:
			if n.Segments != nil {
				for i := 0; i < n.Segments.Len(); i++ {
					seg := n.Segments.At(i)
					htmlText.Write(seg.Value(source))
				}
			}
		case *ast.HTMLBlock:
			if lines := n.Lines(); lines != nil {
				for i := 0; i < lines.Len(); i++ {
					seg := lines.At(i)
					htmlText.Write(seg.Value(source))
				}
			}
			if n.HasClosure() {
				htmlText.Write(n.ClosureLine.Value(source))
			}
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			walk(c)
		}
	}
	walk(doc)
	addHTML(htmlCommentRe.ReplaceAllString(htmlText.String(), ""))

	if len(images) == 0 {
		return nil, raw, false
	}

	textOnly := htmlCommentRe.ReplaceAllString(raw, "")
	textOnly = mdImageStripRe.ReplaceAllString(textOnly, "")
	textOnly = htmlTagStripRe.ReplaceAllString(textOnly, "")
	textOnly = html.UnescapeString(textOnly)
	textOnly = strings.TrimSpace(textOnly)
	return images, textOnly, centerAlignRe.MatchString(raw)
}

// stripHTMLToText handles simple raw-HTML blocks such as
// <p align="center"><strong>text</strong></p> so their tags are not shown as
// literal source in the rendered document.
func stripHTMLToText(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "<") || !strings.Contains(trimmed, ">") {
		return "", false
	}
	s := htmlCommentRe.ReplaceAllString(raw, "")
	s = htmlBRRe.ReplaceAllString(s, "\n")
	s = htmlStrongRe.ReplaceAllString(s, "**$1**")
	s = htmlBoldRe.ReplaceAllString(s, "**$1**")
	s = htmlEmRe.ReplaceAllString(s, "*$1*")
	s = htmlItalicRe.ReplaceAllString(s, "*$1*")
	s = htmlCodeRe.ReplaceAllString(s, "`$1`")
	s = htmlTagStripRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.TrimSpace(s)
	return s, s != ""
}

func nodeAltText(n ast.Node, source []byte) string {
	var b strings.Builder
	ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := c.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(source))
		case *ast.String:
			b.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(html.UnescapeString(b.String()))
}

func parseHTMLImage(tag string) (docImage, bool) {
	attrs := make(map[string]string)
	for _, m := range htmlAttrRe.FindAllStringSubmatch(tag, -1) {
		name := strings.ToLower(m[1])
		value := m[3]
		if value == "" {
			value = m[4]
		}
		if value == "" {
			value = m[5]
		}
		attrs[name] = html.UnescapeString(value)
	}
	src := strings.TrimSpace(attrs["src"])
	if src == "" {
		return docImage{}, false
	}
	img := docImage{Src: src, Alt: strings.TrimSpace(attrs["alt"])}
	if w := strings.TrimSpace(attrs["width"]); w != "" {
		if strings.HasSuffix(w, "%") {
			if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(w, "%"))); err == nil {
				img.WidthPct = n
			}
		} else {
			w = strings.TrimSpace(strings.TrimSuffix(strings.ToLower(w), "px"))
			if n, err := strconv.Atoi(w); err == nil {
				img.WidthPx = n
			}
		}
	}
	return img, true
}

func (a *App) resolveImageSrc(src string) (string, bool, error) {
	src = strings.TrimSpace(html.UnescapeString(src))
	if src == "" {
		return "", false, fmt.Errorf("图片路径为空")
	}
	lower := strings.ToLower(src)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:") {
		return src, true, nil
	}
	if strings.HasPrefix(lower, "file://") {
		u, err := url.Parse(src)
		if err != nil {
			return "", false, err
		}
		p := u.Path
		if u.Host != "" {
			p = "//" + u.Host + p
		}
		if runtime.GOOS == "windows" && len(p) >= 3 && p[0] == '/' && p[2] == ':' {
			p = p[1:]
		}
		src = filepath.FromSlash(p)
	} else if len(src) >= 2 && src[1] == ':' && ((src[0] >= 'a' && src[0] <= 'z') || (src[0] >= 'A' && src[0] <= 'Z')) {
		// Windows drive path.
		src = filepath.FromSlash(src)
	} else {
		if unescaped, err := url.PathUnescape(src); err == nil {
			src = unescaped
		}
		src = filepath.FromSlash(src)
	}
	if !filepath.IsAbs(src) {
		base := ""
		if a.path != "" {
			base = filepath.Dir(a.path)
		} else {
			base, _ = os.Getwd()
		}
		src = filepath.Join(base, src)
	}
	return filepath.Clean(src), false, nil
}

func (a *App) imageFor(src string) (*imageEntry, string) {
	resolved, remote, err := a.resolveImageSrc(src)
	if err != nil {
		return &imageEntry{err: err}, src
	}
	if remote {
		return &imageEntry{err: fmt.Errorf("暂不支持网络图片")}, resolved
	}
	if a.images == nil {
		a.images = make(map[string]*imageEntry)
	}
	if e, ok := a.images[resolved]; ok {
		return e, resolved
	}
	f, err := os.Open(resolved)
	if err != nil {
		e := &imageEntry{err: err}
		a.images[resolved] = e
		return e, resolved
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		e := &imageEntry{err: fmt.Errorf("无法解码图片: %w", err)}
		a.images[resolved] = e
		return e, resolved
	}
	e := &imageEntry{img: img, op: paint.NewImageOp(img)}
	a.images[resolved] = e
	return e, resolved
}

type imageVisual struct {
	item  docImage
	entry *imageEntry
	path  string
	w, h  int
}

func (a *App) layoutImages(gtx layout.Context, images []docImage, width int, center bool) layout.Dimensions {
	if len(images) == 0 || width <= 0 {
		return layout.Dimensions{}
	}
	top := gtx.Dp(unit.Dp(6))
	bottom := gtx.Dp(unit.Dp(6))
	gap := gtx.Dp(unit.Dp(10))
	lineH := gtx.Sp(unit.Sp(18))

	visuals := make([]imageVisual, 0, len(images))
	rowMode := len(images) > 1
	sumPct := 0
	for _, img := range images {
		entry, path := a.imageFor(img.Src)
		v := imageVisual{item: img, entry: entry, path: path}
		if entry.err != nil {
			v.w = width
			if img.WidthPct > 0 {
				v.w = width * img.WidthPct / 100
			} else if img.WidthPx > 0 {
				v.w = gtx.Dp(unit.Dp(float32(img.WidthPx)))
			}
			if v.w > width {
				v.w = width
			}
			if v.w < 1 {
				v.w = width
			}
			v.h = lineH*2 + gtx.Dp(unit.Dp(12))
			rowMode = false
		} else {
			b := entry.img.Bounds()
			nw, nh := b.Dx(), b.Dy()
			if nw <= 0 || nh <= 0 {
				v.entry = &imageEntry{err: fmt.Errorf("图片尺寸无效")}
				v.w, v.h = width, lineH*2+gtx.Dp(unit.Dp(12))
				rowMode = false
			} else {
				if img.WidthPct > 0 {
					sumPct += img.WidthPct
				} else {
					rowMode = false
				}
			}
		}
		visuals = append(visuals, v)
	}
	if rowMode && sumPct > 100 {
		rowMode = false
	}
	if rowMode {
		avail := width - gap*(len(visuals)-1)
		if avail < 1 {
			rowMode = false
		} else {
			for i := range visuals {
				b := visuals[i].entry.img.Bounds()
				visuals[i].w = avail * visuals[i].item.WidthPct / 100
				if visuals[i].w < 1 {
					visuals[i].w = 1
				}
				visuals[i].h = visuals[i].w * b.Dy() / b.Dx()
				if visuals[i].h < 1 {
					visuals[i].h = 1
				}
			}
		}
	}
	if !rowMode {
		for i := range visuals {
			if visuals[i].entry.err != nil {
				continue
			}
			b := visuals[i].entry.img.Bounds()
			nw, nh := b.Dx(), b.Dy()
			w := 0
			switch {
			case visuals[i].item.WidthPct > 0:
				w = width * visuals[i].item.WidthPct / 100
			case visuals[i].item.WidthPx > 0:
				w = gtx.Dp(unit.Dp(float32(visuals[i].item.WidthPx)))
			default:
				w = nw
			}
			if w > width {
				w = width
			}
			if w < 1 {
				w = 1
			}
			visuals[i].w = w
			visuals[i].h = w * nh / nw
			if visuals[i].h < 1 {
				visuals[i].h = 1
			}
		}
	}

	y := top
	if rowMode {
		rowH := 0
		totalW := 0
		for i, v := range visuals {
			if v.h > rowH {
				rowH = v.h
			}
			totalW += v.w
			if i > 0 {
				totalW += gap
			}
		}
		x := 0
		if center && totalW < width {
			x = (width - totalW) / 2
		}
		for _, v := range visuals {
			yy := y + (rowH-v.h)/2
			a.drawOneImage(gtx, v, image.Rect(x, yy, x+v.w, yy+v.h))
			x += v.w + gap
		}
		y += rowH
	} else {
		for _, v := range visuals {
			x := 0
			if center && v.w < width {
				x = (width - v.w) / 2
			}
			a.drawOneImage(gtx, v, image.Rect(x, y, x+v.w, y+v.h))
			y += v.h + gap
		}
		if len(visuals) > 0 {
			y -= gap
		}
	}
	return layout.Dimensions{Size: image.Pt(width, y+bottom)}
}

func (a *App) drawOneImage(gtx layout.Context, v imageVisual, rect image.Rectangle) {
	if v.entry != nil && v.entry.err != nil {
		r := image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Max.Y)
		fillRRect(gtx.Ops, r, gtx.Dp(unit.Dp(7)), colSurfaceAlt)
		msg := "图片加载失败"
		if strings.Contains(v.entry.err.Error(), "网络图片") {
			msg = "暂不支持网络图片"
		}
		if v.item.Alt != "" {
			msg += "：" + v.item.Alt
		}
		msg += "\n" + v.path
		c := gtx
		c.Constraints.Min = image.Point{}
		c.Constraints.Max = image.Pt(maxInt(1, r.Dx()-16), maxInt(1, r.Dy()-12))
		o := op.Offset(image.Pt(r.Min.X+8, r.Min.Y+6)).Push(gtx.Ops)
		widget.Label{
			MaxLines:   2,
			Truncator:  "…",
			WrapPolicy: text.WrapGraphemes,
		}.Layout(c, a.fonts.shaper, a.fonts.body, unit.Sp(12), msg,
			colorMaterial(gtx.Ops, colTextMuted))
		o.Pop()
		return
	}
	if v.entry == nil || v.entry.img == nil || rect.Dx() <= 0 || rect.Dy() <= 0 {
		return
	}
	bounds := v.entry.img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	// A soft border separates screenshots and transparent images from the page.
	fillRect(gtx.Ops, image.Rect(rect.Min.X-1, rect.Min.Y-1, rect.Max.X+1, rect.Max.Y+1), colBorderSoft)
	clipStack := clip.Rect(rect).Push(gtx.Ops)
	off := op.Offset(rect.Min).Push(gtx.Ops)
	sx := float32(rect.Dx()) / float32(bounds.Dx())
	sy := float32(rect.Dy()) / float32(bounds.Dy())
	scale := op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Point{X: sx, Y: sy})).Push(gtx.Ops)
	v.entry.op.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	scale.Pop()
	off.Pop()
	clipStack.Pop()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
