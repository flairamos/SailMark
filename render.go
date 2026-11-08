package main

import (
	"image"
	"image/color"
	"strings"
	"unicode"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

type textStyle struct {
	size   unit.Sp
	bold   bool
	italic bool
	code   bool
	link   bool
	strike bool
	color  color.NRGBA
}

type renderMatch struct {
	start   int
	end     int
	current bool
}

type textToken struct {
	text        string
	style       inlineStyle
	hl          int
	spaceBefore bool
	hardBreak   bool
	start       int
}

func (a *App) fontFor(style textStyle) font.Font {
	f := a.fonts.body
	if style.code {
		f = a.fonts.mono
	}
	if style.bold {
		f.Weight = font.Bold
	}
	if style.italic {
		f.Style = font.Italic
	}
	return f
}

func (a *App) measureText(gtx layout.Context, s string, style textStyle) (layout.Dimensions, op.CallOp) {
	c := gtx
	c.Constraints.Min = image.Point{}
	c.Constraints.Max = image.Pt(1<<20, 1<<20)
	m := op.Record(gtx.Ops)
	dims := widget.Label{WrapPolicy: text.WrapGraphemes}.Layout(
		c, a.fonts.shaper, a.fontFor(style), style.size, s,
		colorMaterial(gtx.Ops, style.color),
	)
	call := m.Stop()
	return dims, call
}

func (a *App) drawLabel(gtx layout.Context, s string, style textStyle, x, y int) layout.Dimensions {
	dims, call := a.measureText(gtx, s, style)
	st := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	st.Pop()
	return dims
}

func matchLevel(matches []renderMatch, off int) int {
	for _, m := range matches {
		if off >= m.start && off < m.end {
			if m.current {
				return 2
			}
			return 1
		}
	}
	return 0
}

func buildTokens(segs []inlineSeg, matches []renderMatch) []textToken {
	var tokens []textToken
	pendingSpace := false
	cur := textToken{}
	flush := func() {
		if cur.text != "" {
			tokens = append(tokens, cur)
			cur = textToken{}
		}
	}
	for _, seg := range segs {
		runes := []rune(seg.Text)
		for i, r := range runes {
			off := seg.Start + i
			hl := matchLevel(matches, off)
			if r == '\n' {
				flush()
				tokens = append(tokens, textToken{hardBreak: true})
				pendingSpace = false
				continue
			}
			if unicode.IsSpace(r) {
				flush()
				pendingSpace = true
				continue
			}
			if isCJK(r) {
				flush()
				tokens = append(tokens, textToken{
					text:        string(r),
					style:       seg.Style,
					hl:          hl,
					spaceBefore: pendingSpace,
					start:       off,
				})
				pendingSpace = false
				continue
			}
			if cur.text != "" && (cur.style != seg.Style || cur.hl != hl) {
				flush()
			}
			if cur.text == "" {
				cur.style = seg.Style
				cur.hl = hl
				cur.start = off
				cur.spaceBefore = pendingSpace
				pendingSpace = false
			}
			cur.text += string(r)
		}
	}
	flush()
	return tokens
}

func (a *App) layoutRich(gtx layout.Context, segs []inlineSeg, base textStyle, matches []renderMatch) layout.Dimensions {
	maxW := gtx.Constraints.Max.X
	if maxW <= 0 {
		return layout.Dimensions{}
	}
	tokens := buildTokens(segs, matches)
	if len(tokens) == 0 {
		lineH := int(float32(gtx.Sp(base.size)) * 1.55)
		return layout.Dimensions{Size: image.Pt(maxW, lineH)}
	}

	lineH := int(float32(gtx.Sp(base.size)) * 1.70)
	if lineH < gtx.Sp(base.size)+6 {
		lineH = gtx.Sp(base.size) + 6
	}
	spaceW := gtx.Sp(unit.Sp(float32(base.size) * 0.30))
	x := 0
	lineTop := 0
	maxX := 0
	lineHasContent := false

	newLine := func() {
		if x > maxX {
			maxX = x
		}
		lineTop += lineH
		x = 0
		lineHasContent = false
	}

	for _, tok := range tokens {
		if tok.hardBreak {
			if lineHasContent {
				newLine()
			} else {
				lineTop += lineH
			}
			continue
		}
		st := base
		st.bold = base.bold || tok.style.Bold
		st.italic = base.italic || tok.style.Italic
		st.code = tok.style.Code
		st.link = tok.style.Link
		st.strike = tok.style.Strike
		if tok.style.Code {
			st.color = colCode
		}
		if tok.style.Link {
			st.color = colLink
		}
		dims, call := a.measureText(gtx, tok.text, st)
		if dims.Size.X <= 0 {
			continue
		}
		space := 0
		if lineHasContent && tok.spaceBefore {
			space = spaceW
		}
		if lineHasContent && x+space+dims.Size.X > maxW {
			newLine()
			space = 0
		}
		x += space
		yTop := lineTop + (lineH-dims.Size.Y)/2
		if yTop < lineTop {
			yTop = lineTop
		}
		// Highlight and inline-code backgrounds sit behind the glyphs.
		if tok.hl > 0 {
			hc := colHighlight
			if tok.hl == 2 {
				hc = colHighlightCur
			}
			fillRRect(gtx.Ops, image.Rect(x-1, lineTop+1, x+dims.Size.X+1, lineTop+lineH-2), 3, hc)
		} else if tok.style.Code {
			fillRRect(gtx.Ops, image.Rect(x-1, lineTop+2, x+dims.Size.X+2, lineTop+lineH-2), 3, colCodeBg)
		}
		off := op.Offset(image.Pt(x, yTop)).Push(gtx.Ops)
		if st.italic && !st.code {
			// Microsoft YaHei has no italic face. Gio falls back to the
			// regular face, so synthesize a gentle slant for Markdown
			// emphasis that also works for CJK glyphs.
			shear := op.Affine(f32.Affine2D{}.Shear(f32.Point{Y: float32(dims.Size.Y)}, -0.18, 0)).Push(gtx.Ops)
			call.Add(gtx.Ops)
			shear.Pop()
		} else {
			call.Add(gtx.Ops)
		}
		off.Pop()
		if tok.style.Link {
			fillRect(gtx.Ops, image.Rect(x, yTop+dims.Size.Y-2, x+dims.Size.X, yTop+dims.Size.Y-1), withAlpha(colLink, 0x80))
		}
		if tok.style.Strike {
			yy := yTop + dims.Size.Y/2
			fillRect(gtx.Ops, image.Rect(x, yy, x+dims.Size.X, yy+1), colText)
		}
		x += dims.Size.X
		lineHasContent = true
	}
	if lineHasContent {
		if x > maxX {
			maxX = x
		}
		lineTop += lineH
	} else {
		lineTop += lineH
	}
	if maxX < maxW {
		maxX = maxW
	}
	return layout.Dimensions{Size: image.Pt(maxX, lineTop)}
}

func (a *App) layoutBlock(gtx layout.Context, idx int, width int) layout.Dimensions {
	b := a.doc.Blocks[idx]
	if b.Display == nil {
		b.Display = buildDisplay(b)
	}
	matches := a.blockMatches(idx)
	switch b.Kind {
	case kindHeading:
		return a.layoutHeading(gtx, b, width, matches)
	case kindCode:
		return a.layoutCode(gtx, b, width, matches)
	case kindQuote:
		return a.layoutQuote(gtx, b, width, matches)
	case kindList:
		return a.layoutList(gtx, b, width, matches)
	case kindTable:
		return a.layoutTable(gtx, b, width, matches)
	case kindRule:
		return a.layoutRule(gtx, width)
	default:
		return a.layoutParagraph(gtx, b, width, matches)
	}
}

func (a *App) layoutParagraph(gtx layout.Context, b *Block, width int, matches []renderMatch) layout.Dimensions {
	top := gtx.Dp(5)
	bottom := gtx.Dp(5)
	gap := gtx.Dp(8)
	st := textStyle{size: unit.Sp(16), color: colText}
	hasText := strings.TrimSpace(b.Display.Plain) != ""
	hasImages := len(b.Display.Images) > 0

	if !hasText && !hasImages {
		if b.Raw == "" {
			st.color = colTextFaint
			h := top + bottom + gtx.Sp(unit.Sp(16))*2
			y := top + gtx.Sp(unit.Sp(16))/2
			a.drawLabel(gtx, "点击此处开始编写…", st, 2, y)
			return layout.Dimensions{Size: image.Pt(width, h)}
		}
		// Whitespace-only block: keep a single empty line clickable.
		c := gtx
		c.Constraints.Min = image.Pt(width, 0)
		c.Constraints.Max = image.Pt(width, 1<<20)
		off := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
		dims := a.layoutRich(c, b.Display.Segs, textStyle{size: unit.Sp(16), color: colTextFaint}, nil)
		off.Pop()
		return layout.Dimensions{Size: image.Pt(width, top+bottom+dims.Size.Y)}
	}

	c := gtx
	c.Constraints.Min = image.Pt(width, 0)
	c.Constraints.Max = image.Pt(width, 1<<20)
	y := top
	if hasText {
		off := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		dims := a.layoutRich(c, b.Display.Segs, st, matches)
		off.Pop()
		y += dims.Size.Y
	}
	if hasImages {
		if hasText {
			y += gap
		}
		off := op.Offset(image.Pt(0, y)).Push(gtx.Ops)
		dims := a.layoutImages(c, b.Display.Images, width, b.Display.Center || !hasText)
		off.Pop()
		y += dims.Size.Y
	}
	return layout.Dimensions{Size: image.Pt(width, y+bottom)}
}

func headingSize(level int) unit.Sp {
	switch level {
	case 1:
		return 30
	case 2:
		return 25
	case 3:
		return 21
	case 4:
		return 18.5
	case 5:
		return 17
	default:
		return 16
	}
}

func (a *App) layoutHeading(gtx layout.Context, b *Block, width int, matches []renderMatch) layout.Dimensions {
	level := b.Level
	if level < 1 || level > 6 {
		level = 1
	}
	top := gtx.Dp(unit.Dp(float32(10 + level*2)))
	if level == 1 {
		top = gtx.Dp(unit.Dp(20))
	}
	bottom := gtx.Dp(unit.Dp(5))
	st := textStyle{size: headingSize(level), bold: true, color: colText}
	c := gtx
	c.Constraints.Min = image.Pt(width, 0)
	c.Constraints.Max = image.Pt(width, 1<<20)
	off := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
	dims := a.layoutRich(c, b.Display.Segs, st, matches)
	off.Pop()
	h := top + bottom + dims.Size.Y
	if level <= 2 {
		y := h - gtx.Dp(unit.Dp(2))
		fillRect(gtx.Ops, image.Rect(0, y, width, y+1), colBorderSoft)
		h += gtx.Dp(unit.Dp(2))
	}
	return layout.Dimensions{Size: image.Pt(width, h)}
}

func (a *App) layoutCode(gtx layout.Context, b *Block, width int, matches []renderMatch) layout.Dimensions {
	top := gtx.Dp(unit.Dp(8))
	bottom := gtx.Dp(unit.Dp(8))
	pad := gtx.Dp(unit.Dp(14))
	radius := gtx.Dp(unit.Dp(8))
	code := b.Display.Code
	lines := strings.Split(code, "\n")
	if code == "" {
		lines = []string{""}
	}
	codeStyle := textStyle{size: unit.Sp(13.5), code: true, color: colText}
	langStyle := textStyle{size: unit.Sp(11.5), code: true, color: colTextFaint}
	lineH := gtx.Sp(unit.Sp(21))

	m := op.Record(gtx.Ops)
	y := pad
	lang := strings.TrimSpace(b.Display.CodeLang)
	if lang != "" {
		a.drawLabel(gtx, strings.ToUpper(lang), langStyle, pad, y)
		y += gtx.Sp(unit.Sp(19))
	}
	offset := 0
	for _, line := range lines {
		lineStart := offset
		lineEnd := lineStart + runeCount(line)
		hl := false
		cur := false
		for _, mm := range matches {
			if mm.start < lineEnd && mm.end > lineStart {
				hl = true
				if mm.current {
					cur = true
				}
			}
		}
		if hl && line != "" {
			hc := colHighlight
			if cur {
				hc = colHighlightCur
			}
			fillRect(gtx.Ops, image.Rect(pad-2, y, width-pad+2, y+lineH), withAlpha(hc, 0xB0))
		}
		c := gtx
		c.Constraints.Min = image.Point{}
		c.Constraints.Max = image.Pt(width-pad*2, 1<<20)
		off := op.Offset(image.Pt(pad, y)).Push(gtx.Ops)
		dims := widget.Label{WrapPolicy: text.WrapGraphemes, LineHeight: unit.Sp(21)}.Layout(
			c, a.fonts.shaper, a.fonts.mono, codeStyle.size, line,
			colorMaterial(gtx.Ops, codeStyle.color),
		)
		off.Pop()
		if dims.Size.Y > lineH {
			y += dims.Size.Y
		} else {
			y += lineH
		}
		offset = lineEnd + 1
	}
	call := m.Stop()
	blockH := y + pad
	if blockH < gtx.Sp(unit.Sp(44)) {
		blockH = gtx.Sp(unit.Sp(44))
	}
	outer := image.Rect(0, top, width, top+blockH)
	fillRRect(gtx.Ops, outer, radius, colBorder)
	inner := image.Rect(outer.Min.X+1, outer.Min.Y+1, outer.Max.X-1, outer.Max.Y-1)
	fillRRect(gtx.Ops, inner, radius-1, colCodeBlockBg)
	off := op.Offset(image.Pt(0, top)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	off.Pop()
	return layout.Dimensions{Size: image.Pt(width, top+bottom+blockH)}
}

func (a *App) layoutQuote(gtx layout.Context, b *Block, width int, matches []renderMatch) layout.Dimensions {
	top := gtx.Dp(unit.Dp(5))
	bottom := gtx.Dp(unit.Dp(5))
	barW := gtx.Dp(unit.Dp(4))
	gap := gtx.Dp(unit.Dp(14))
	st := textStyle{size: unit.Sp(15.5), color: colTextMuted}
	m := op.Record(gtx.Ops)
	c := gtx
	c.Constraints.Min = image.Pt(width-barW-gap, 0)
	c.Constraints.Max = image.Pt(width-barW-gap, 1<<20)
	off := op.Offset(image.Pt(barW+gap, top)).Push(gtx.Ops)
	dims := a.layoutRich(c, b.Display.Segs, st, matches)
	off.Pop()
	call := m.Stop()
	h := dims.Size.Y + top + bottom
	fillRRect(gtx.Ops, image.Rect(0, top, barW, h-bottom), barW/2, colQuoteBar)
	// The recorded call already contains the top offset.
	call.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(width, h)}
}

func (a *App) layoutList(gtx layout.Context, b *Block, width int, matches []renderMatch) layout.Dimensions {
	top := gtx.Dp(unit.Dp(4))
	bottom := gtx.Dp(unit.Dp(4))
	ld := b.Display.List
	indent := 0
	if ld != nil {
		indent = ld.Indent
	}
	if indent > 12 {
		indent = 12
	}
	markerX := gtx.Dp(unit.Dp(6)) + indent*gtx.Dp(unit.Dp(7))
	contentX := markerX + gtx.Dp(unit.Dp(26))
	if contentX > width-20 {
		contentX = width / 4
	}
	st := textStyle{size: unit.Sp(16), color: colText}
	if ld != nil && ld.Task == 1 {
		st.color = colTextMuted
	}

	m := op.Record(gtx.Ops)
	c := gtx
	c.Constraints.Min = image.Pt(width-contentX, 0)
	c.Constraints.Max = image.Pt(width-contentX, 1<<20)
	off := op.Offset(image.Pt(contentX, top)).Push(gtx.Ops)
	dims := a.layoutRich(c, b.Display.Segs, st, matches)
	off.Pop()
	contentCall := m.Stop()
	contentH := dims.Size.Y
	if contentH < gtx.Sp(unit.Sp(24)) {
		contentH = gtx.Sp(unit.Sp(24))
	}
	h := top + bottom + contentH

	if ld != nil && ld.Task >= 0 {
		box := gtx.Dp(unit.Dp(14))
		by := top + (contentH-box)/2
		bx := markerX
		if ld.Task == 1 {
			fillRRect(gtx.Ops, image.Rect(bx, by, bx+box, by+box), 3, colAccent)
			// A simple white check mark.
			paint.FillShape(gtx.Ops, color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
				clip.Stroke{
					Path: func() clip.PathSpec {
						var p clip.Path
						p.Begin(gtx.Ops)
						p.MoveTo(f32.Point{X: float32(bx) + 3, Y: float32(by) + float32(box)/2})
						p.LineTo(f32.Point{X: float32(bx) + 6, Y: float32(by+box) - 4})
						p.LineTo(f32.Point{X: float32(bx+box) - 3, Y: float32(by) + 4})
						return p.End()
					}(),
					Width: float32(gtx.Dp(unit.Dp(2))),
				}.Op())
		} else {
			fillRRect(gtx.Ops, image.Rect(bx, by, bx+box, by+box), 3, colSurface)
			fillRect(gtx.Ops, image.Rect(bx, by, bx+box, by+1), colBorder)
			fillRect(gtx.Ops, image.Rect(bx, by+box-1, bx+box, by+box), colBorder)
			fillRect(gtx.Ops, image.Rect(bx, by, bx+1, by+box), colBorder)
			fillRect(gtx.Ops, image.Rect(bx+box-1, by, bx+box, by+box), colBorder)
		}
	} else if ld != nil && ld.Ordered {
		ms := textStyle{size: unit.Sp(15), color: colTextMuted}
		d, call := a.measureText(gtx, ld.Number, ms)
		x := markerX + gtx.Dp(unit.Dp(22)) - d.Size.X
		if x < markerX {
			x = markerX
		}
		yy := top + (contentH-d.Size.Y)/2
		o := op.Offset(image.Pt(x, yy)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		o.Pop()
	} else {
		by := top + (contentH-gtx.Dp(unit.Dp(6)))/2
		fillRRect(gtx.Ops, image.Rect(markerX, by, markerX+gtx.Dp(unit.Dp(6)), by+gtx.Dp(unit.Dp(6))), 3, colTextMuted)
	}
	contentCall.Add(gtx.Ops)
	return layout.Dimensions{Size: image.Pt(width, h)}
}

func (a *App) layoutTable(gtx layout.Context, b *Block, width int, matches []renderMatch) layout.Dimensions {
	top := gtx.Dp(unit.Dp(8))
	bottom := gtx.Dp(unit.Dp(8))
	td := b.Display.Table
	if td == nil {
		return layout.Dimensions{Size: image.Pt(width, top+bottom)}
	}
	rows := make([][]tableCell, 0, len(td.Rows)+1)
	rows = append(rows, td.Header)
	rows = append(rows, td.Rows...)
	cols := 1
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	colW := (width - 1) / cols
	pad := gtx.Dp(unit.Dp(9))
	cellStyle := textStyle{size: unit.Sp(14.5), color: colText}
	headStyle := cellStyle
	headStyle.bold = true

	y := top
	rowHeights := make([]int, len(rows))
	rowCalls := make([][]op.CallOp, len(rows))
	for ri, row := range rows {
		calls := make([]op.CallOp, cols)
		rowH := gtx.Sp(unit.Sp(32))
		for ci := 0; ci < cols; ci++ {
			var cell tableCell
			if ci < len(row) {
				cell = row[ci]
			}
			st := cellStyle
			if ri == 0 {
				st = headStyle
			}
			m := op.Record(gtx.Ops)
			c := gtx
			c.Constraints.Min = image.Pt(colW-pad*2, 0)
			c.Constraints.Max = image.Pt(colW-pad*2, 1<<20)
			o := op.Offset(image.Pt(pad, pad)).Push(gtx.Ops)
			dims := a.layoutRich(c, cell.Segs, st, matches)
			o.Pop()
			call := m.Stop()
			calls[ci] = call
			if dims.Size.Y+pad*2 > rowH {
				rowH = dims.Size.Y + pad*2
			}
		}
		rowHeights[ri] = rowH
		rowCalls[ri] = calls
		y += rowH
	}
	totalH := y + bottom
	// Header background and outer border.
	fillRect(gtx.Ops, image.Rect(0, top, width, top+rowHeights[0]), colSurfaceAlt)
	fillRect(gtx.Ops, image.Rect(0, top, width, top+1), colBorder)
	fillRect(gtx.Ops, image.Rect(0, totalH-bottom-1, width, totalH-bottom), colBorder)
	fillRect(gtx.Ops, image.Rect(0, top, 1, totalH-bottom), colBorder)
	fillRect(gtx.Ops, image.Rect(width-1, top, width, totalH-bottom), colBorder)
	yy := top
	for ri, rowH := range rowHeights {
		if ri > 0 {
			fillRect(gtx.Ops, image.Rect(0, yy, width, yy+1), colBorderSoft)
		}
		for ci := 0; ci < cols; ci++ {
			cx := ci * colW
			if ci > 0 {
				fillRect(gtx.Ops, image.Rect(cx, yy, cx+1, yy+rowH), colBorderSoft)
			}
			o := op.Offset(image.Pt(cx, yy)).Push(gtx.Ops)
			rowCalls[ri][ci].Add(gtx.Ops)
			o.Pop()
		}
		yy += rowH
	}
	return layout.Dimensions{Size: image.Pt(width, totalH)}
}

func (a *App) layoutRule(gtx layout.Context, width int) layout.Dimensions {
	h := gtx.Dp(unit.Dp(28))
	y := h / 2
	fillRect(gtx.Ops, image.Rect(0, y, width, y+1), colBorder)
	return layout.Dimensions{Size: image.Pt(width, h)}
}

func (a *App) layoutEditCard(gtx layout.Context, idx int, width int) layout.Dimensions {
	top := gtx.Dp(unit.Dp(8))
	bottom := gtx.Dp(unit.Dp(12))
	pad := gtx.Dp(unit.Dp(14))
	radius := gtx.Dp(unit.Dp(9))
	headerH := gtx.Sp(unit.Sp(20))

	m := op.Record(gtx.Ops)
	// Header hint.
	hintStyle := textStyle{size: unit.Sp(11.5), color: colAccent}
	o := op.Offset(image.Pt(pad, pad)).Push(gtx.Ops)
	a.drawLabel(gtx, "编辑中 · 点击空白处或按 Esc 完成", hintStyle, 0, 0)
	o.Pop()
	o = op.Offset(image.Pt(pad, pad+headerH)).Push(gtx.Ops)
	ed := gtx
	ed.Constraints.Min = image.Pt(width-pad*2, 0)
	ed.Constraints.Max = image.Pt(width-pad*2, 1<<20)
	editorDims := a.blockEditor.Layout(
		ed, a.fonts.shaper, a.fonts.mono, unit.Sp(14),
		colorMaterial(gtx.Ops, colText),
		colorMaterial(gtx.Ops, withAlpha(colAccent, 0x55)),
	)
	o.Pop()
	call := m.Stop()

	innerH := pad*2 + headerH + editorDims.Size.Y
	if innerH < gtx.Sp(unit.Sp(56)) {
		innerH = gtx.Sp(unit.Sp(56))
	}
	outer := image.Rect(0, top, width, top+innerH)
	fillRRect(gtx.Ops, outer, radius, colAccent)
	inner := image.Rect(outer.Min.X+1, outer.Min.Y+1, outer.Max.X-1, outer.Max.Y-1)
	fillRRect(gtx.Ops, inner, radius-1, colSurface)
	o = op.Offset(image.Pt(0, top)).Push(gtx.Ops)
	call.Add(gtx.Ops)
	o.Pop()

	// Store the screen-space rectangle. The viewport adjusts it by its own
	// position, which is at the top of the content area.
	a.editRect = image.Rect(0, top, width, top+innerH)
	return layout.Dimensions{Size: image.Pt(width, top+bottom+innerH)}
}
