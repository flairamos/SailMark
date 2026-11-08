package main

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/gesture"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

func (a *App) labelNow(gtx layout.Context, s string, st textStyle) layout.Dimensions {
	dims, call := a.measureText(gtx, s, st)
	call.Add(gtx.Ops)
	return dims
}

func (a *App) labelTruncated(gtx layout.Context, s string, st textStyle, maxW int, align text.Alignment) layout.Dimensions {
	c := gtx
	c.Constraints.Min = image.Point{}
	c.Constraints.Max = image.Pt(maxW, 1<<20)
	return widget.Label{
		MaxLines:  1,
		Truncator: "…",
		Alignment: align,
	}.Layout(c, a.fonts.shaper, a.fontFor(st), st.size, s, colorMaterial(gtx.Ops, st.color))
}

func (a *App) labelRight(gtx layout.Context, s string, st textStyle, maxW int) layout.Dimensions {
	c := gtx
	c.Constraints.Min = image.Pt(maxW, 0)
	c.Constraints.Max = image.Pt(maxW, 1<<20)
	return widget.Label{
		MaxLines:  1,
		Truncator: "…",
		Alignment: text.End,
	}.Layout(c, a.fonts.shaper, a.fontFor(st), st.size, s, colorMaterial(gtx.Ops, st.color))
}

func (a *App) spacer(gtx layout.Context, dp unit.Dp) layout.Dimensions {
	return layout.Dimensions{Size: image.Pt(gtx.Dp(dp), 0)}
}

func (a *App) textButton(gtx layout.Context, text string, btn *widget.Clickable, enabled, primary bool, action func()) layout.Dimensions {
	clicked := btn.Clicked(gtx)
	if clicked && enabled && action != nil {
		action()
	}
	st := textStyle{size: unit.Sp(13.5), color: colText}
	if primary {
		st.color = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	} else if !enabled {
		st.color = colTextFaint
	} else {
		st.color = colTextMuted
	}
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		dims := layout.Inset{Top: unit.Dp(7), Bottom: unit.Dp(7), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.labelNow(gtx, text, st)
		})
		call := m.Stop()
		bg := color.NRGBA{}
		if primary {
			bg = colAccent
			if enabled && btn.Pressed() {
				bg = color.NRGBA{R: 0x1E, G: 0x5A, B: 0xD0, A: 0xFF}
			} else if enabled && btn.Hovered() {
				bg = color.NRGBA{R: 0x25, G: 0x63, B: 0xE0, A: 0xFF}
			}
		} else if enabled && btn.Pressed() {
			bg = colMenuPressed
		} else if enabled && btn.Hovered() {
			bg = colMenuHover
		}
		if bg.A != 0 {
			fillRRect(gtx.Ops, image.Rect(0, 0, dims.Size.X, dims.Size.Y), gtx.Dp(unit.Dp(6)), bg)
		}
		call.Add(gtx.Ops)
		return dims
	})
}

func (a *App) menuButton(gtx layout.Context, text string, btn *widget.Clickable, enabled bool, action func()) layout.Dimensions {
	st := textStyle{size: unit.Sp(14.5), color: colTextMuted}
	if !enabled {
		st.color = colTextFaint
	}
	clicked := btn.Clicked(gtx)
	if clicked && enabled && action != nil {
		action()
	}
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		dims := layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.labelNow(gtx, text, st)
		})
		call := m.Stop()
		bg := color.NRGBA{}
		if enabled && btn.Pressed() {
			bg = colMenuPressed
		} else if enabled && btn.Hovered() {
			bg = colMenuHover
		}
		if bg.A != 0 {
			fillRRect(gtx.Ops, image.Rect(0, 0, dims.Size.X, dims.Size.Y), gtx.Dp(unit.Dp(6)), bg)
		}
		call.Add(gtx.Ops)
		return dims
	})
}

func (a *App) layoutMenu(gtx layout.Context) layout.Dimensions {
	h := gtx.Dp(unit.Dp(52))
	w := gtx.Constraints.Max.X
	gtx.Constraints.Min.Y = h
	gtx.Constraints.Max.Y = h
	fillRect(gtx.Ops, image.Rect(0, 0, w, h), colSurface)
	fillRect(gtx.Ops, image.Rect(0, h-1, w, h), colBorderSoft)

	enabled := a.docOpen && !a.dialogBusy
	dims := layout.Inset{Left: unit.Dp(18), Right: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.labelNow(gtx, "SailMark", textStyle{size: unit.Sp(19), bold: true, color: colAccent})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.spacer(gtx, unit.Dp(18)) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.menuButton(gtx, "打开", &a.openBtn, !a.dialogBusy, a.requestOpen)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.menuButton(gtx, "保存", &a.saveBtn, enabled, func() { a.saveDocument() })
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.menuButton(gtx, "关闭", &a.closeBtn, enabled, a.requestClose)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.menuButton(gtx, "查找", &a.findBtn, enabled, a.toggleFind)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				title := ""
				if a.docOpen {
					title = a.currentFileName()
					if a.dirty {
						title = "● " + title
					}
				}
				st := textStyle{size: unit.Sp(12.5), color: colTextFaint}
				return layout.Inset{Left: unit.Dp(18)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						maxW := gtx.Constraints.Max.X
						if maxW < 0 {
							maxW = 0
						}
						return a.labelRight(gtx, title, st, maxW)
					})
				})
			}),
		)
	})
	return dims
}

func (a *App) layoutFindBar(gtx layout.Context) layout.Dimensions {
	h := gtx.Dp(unit.Dp(52))
	w := gtx.Constraints.Max.X
	gtx.Constraints.Min.Y = h
	gtx.Constraints.Max.Y = h
	fillRect(gtx.Ops, image.Rect(0, 0, w, h), colSurfaceAlt)
	fillRect(gtx.Ops, image.Rect(0, h-1, w, h), colBorderSoft)

	count := ""
	if a.findQuery != "" {
		if len(a.findMatches) == 0 {
			count = "无结果"
		} else {
			count = fmt.Sprintf("%d/%d", a.findCur+1, len(a.findMatches))
		}
	}
	countColor := colTextMuted
	if a.findQuery != "" && len(a.findMatches) == 0 {
		countColor = colDanger
	}

	return layout.Inset{Left: unit.Dp(18), Right: unit.Dp(18), Top: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, a.layoutFindInput),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.spacer(gtx, unit.Dp(12)) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.labelNow(gtx, count, textStyle{size: unit.Sp(12.5), color: countColor})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.spacer(gtx, unit.Dp(10)) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.textButton(gtx, "上一个", &a.findPrev, len(a.findMatches) > 0, false, func() { a.findNext(-1) })
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.spacer(gtx, unit.Dp(4)) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.textButton(gtx, "下一个", &a.findNextBtn, len(a.findMatches) > 0, false, func() { a.findNext(1) })
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.spacer(gtx, unit.Dp(6)) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.textButton(gtx, "✕", &a.findClose, true, false, a.closeFind)
			}),
		)
	})
}

func (a *App) layoutFindInput(gtx layout.Context) layout.Dimensions {
	a.processFindEditor(gtx)
	h := gtx.Dp(unit.Dp(34))
	w := gtx.Constraints.Max.X
	gtx.Constraints.Min.Y = h
	gtx.Constraints.Max.Y = h
	fillRRect(gtx.Ops, image.Rect(0, 0, w, h), gtx.Dp(unit.Dp(7)), colSurface)
	fillRect(gtx.Ops, image.Rect(0, 0, w, 1), colBorderSoft)
	fillRect(gtx.Ops, image.Rect(0, h-1, w, h), colBorderSoft)

	inner := layout.Inset{Left: unit.Dp(11), Right: unit.Dp(11)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		st := textStyle{size: unit.Sp(14), color: colText}
		ed := gtx
		ed.Constraints.Min = image.Pt(gtx.Constraints.Max.X, 0)
		ed.Constraints.Max = image.Pt(gtx.Constraints.Max.X, h)
		dims := a.findEd.Layout(ed, a.fonts.shaper, a.fonts.body, st.size,
			colorMaterial(gtx.Ops, st.color),
			colorMaterial(gtx.Ops, withAlpha(colAccent, 0x55)),
		)
		if a.findEd.Text() == "" {
			ph := textStyle{size: unit.Sp(14), color: colTextFaint}
			a.drawLabel(gtx, "查找内容", ph, 0, (h-gtx.Sp(unit.Sp(18)))/2)
		}
		if dims.Size.Y == 0 {
			dims.Size.Y = h
		}
		return dims
	})
	if a.findFocusWait {
		gtx.Execute(key.FocusCmd{Tag: &a.findEd})
		a.findFocusWait = false
	}
	return inner
}

func (a *App) layoutStatusBar(gtx layout.Context) layout.Dimensions {
	h := gtx.Dp(unit.Dp(30))
	w := gtx.Constraints.Max.X
	gtx.Constraints.Min.Y = h
	gtx.Constraints.Max.Y = h
	fillRect(gtx.Ops, image.Rect(0, 0, w, h), colSurface)
	fillRect(gtx.Ops, image.Rect(0, 0, w, 1), colBorderSoft)

	left := a.status
	leftColor := colTextMuted
	switch a.statusColorKey {
	case 1:
		leftColor = colDanger
	case 2:
		leftColor = colSuccess
	}
	if left == "" {
		if a.docOpen {
			left = a.path
		} else {
			left = "就绪 · 打开一个 Markdown 文件开始"
		}
	}
	words := 0
	if a.doc != nil {
		words = a.doc.WordCount()
	}
	right := ""
	if a.docOpen {
		right = fmt.Sprintf("%d 字 · UTF-8 · Markdown", words)
	}
	return layout.Inset{Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.labelTruncated(gtx, left, textStyle{size: unit.Sp(11.5), color: leftColor}, gtx.Constraints.Max.X, text.Start)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.spacer(gtx, unit.Dp(12)) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.labelNow(gtx, right, textStyle{size: unit.Sp(11.5), color: colTextFaint})
				})
			}),
		)
	})
}

func (a *App) layoutViewport(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: size}
	}
	fillRect(gtx.Ops, image.Rectangle{Max: size}, colBackground)
	topOffset := gtx.Dp(unit.Dp(52))
	if a.findOpen {
		topOffset += gtx.Dp(unit.Dp(52))
	}
	a.viewportRect = image.Rect(0, topOffset, size.X, topOffset+size.Y)

	// Consume wheel events. Positive delta scrolls content upward, matching
	// Gio's layout.List convention.
	if d := a.viewportScroll.Update(gtx.Metric, gtx.Source, gtx.Now, gesture.Vertical, pointer.ScrollRange{}, a.scrollRange()); d != 0 {
		a.scroll += d
		gtx.Execute(op.InvalidateCmd{})
	}
	maxScroll := 0
	if a.contentH > size.Y {
		maxScroll = a.contentH - size.Y
	}
	a.scroll = clamp(a.scroll, 0, maxScroll)

	area := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
	a.viewportClick.Add(gtx.Ops)
	a.viewportScroll.Add(gtx.Ops)

	if a.docOpen && a.doc != nil {
		topPad := gtx.Dp(unit.Dp(20))
		bottomPad := gtx.Dp(unit.Dp(110))
		gap := gtx.Dp(unit.Dp(2))
		margin := gtx.Dp(unit.Dp(44))
		contentW := size.X - margin*2
		maxContentW := gtx.Dp(unit.Dp(900))
		if contentW > maxContentW {
			contentW = maxContentW
		}
		if contentW < 220 {
			contentW = size.X - gtx.Dp(unit.Dp(24))
		}
		x := (size.X - contentW) / 2
		y := topPad
		a.blockRects = a.blockRects[:0]
		a.editRect = image.Rectangle{}
		for i := range a.doc.Blocks {
			screenTop := y - a.scroll
			offset := op.Offset(image.Pt(x, screenTop)).Push(gtx.Ops)
			var dims layout.Dimensions
			if a.editIdx == i {
				dims = a.layoutEditCard(gtx, i, contentW)
				a.editRect = a.editRect.Add(image.Pt(x, screenTop))
			} else {
				dims = a.layoutBlock(gtx, i, contentW)
			}
			offset.Pop()
			a.blockRects = append(a.blockRects, image.Rect(x, screenTop, x+contentW, screenTop+dims.Size.Y))
			y += dims.Size.Y + gap
		}
		a.contentH = y + bottomPad
		newMax := 0
		if a.contentH > size.Y {
			newMax = a.contentH - size.Y
		}
		if a.scroll > newMax {
			a.scroll = newMax
			gtx.Execute(op.InvalidateCmd{})
		}

		if a.findOpen && a.pendingScrollBlock >= 0 && a.pendingScrollBlock < len(a.blockRects) {
			r := a.blockRects[a.pendingScrollBlock]
			if r.Min.Y < 0 || r.Max.Y > size.Y-12 {
				target := r.Min.Y + a.scroll - size.Y/3
				a.scroll = clamp(target, 0, newMax)
				a.pendingScrollBlock = -1
				gtx.Execute(op.InvalidateCmd{})
			} else {
				a.pendingScrollBlock = -1
			}
		}
	} else {
		a.blockRects = a.blockRects[:0]
		a.contentH = 0
		a.layoutWelcome(gtx, size)
	}

	area.Pop()

	if a.docOpen && a.contentH > size.Y {
		track := size.Y - gtx.Dp(unit.Dp(16))
		thumb := size.Y * size.Y / a.contentH
		if thumb < gtx.Dp(unit.Dp(34)) {
			thumb = gtx.Dp(unit.Dp(34))
		}
		if thumb > track {
			thumb = track
		}
		denom := a.contentH - size.Y
		if denom < 1 {
			denom = 1
		}
		ty := gtx.Dp(unit.Dp(8)) + (track-thumb)*a.scroll/denom
		x := size.X - gtx.Dp(unit.Dp(9))
		fillRRect(gtx.Ops, image.Rect(x, ty, x+gtx.Dp(unit.Dp(4)), ty+thumb), 2, colBorder)
	}

	// Click handling happens after layout so the hit rectangles are current.
	a.handleViewportClick(gtx)

	if a.editFocusPending && a.editIdx >= 0 && !a.editRect.Empty() {
		gtx.Execute(key.FocusCmd{Tag: &a.blockEditor})
		a.editFocusPending = false
	}
	return layout.Dimensions{Size: size}
}

func (a *App) layoutWelcome(gtx layout.Context, size image.Point) {
	cardW := gtx.Dp(unit.Dp(480))
	if cardW > size.X-gtx.Dp(unit.Dp(40)) {
		cardW = size.X - gtx.Dp(unit.Dp(40))
	}
	if cardW < 260 {
		cardW = size.X - gtx.Dp(unit.Dp(16))
	}
	cardH := gtx.Dp(unit.Dp(286))
	x := (size.X - cardW) / 2
	y := (size.Y - cardH) / 2
	if y < gtx.Dp(unit.Dp(18)) {
		y = gtx.Dp(unit.Dp(18))
	}
	radius := gtx.Dp(unit.Dp(14))
	fillRRect(gtx.Ops, image.Rect(x-1, y-1, x+cardW+1, y+cardH+1), radius+1, colBorderSoft)
	fillRRect(gtx.Ops, image.Rect(x, y, x+cardW, y+cardH), radius, colSurface)

	pad := gtx.Dp(unit.Dp(32))
	// Small accent tile.
	fillRRect(gtx.Ops, image.Rect(x+pad, y+pad, x+pad+gtx.Dp(unit.Dp(46)), y+pad+gtx.Dp(unit.Dp(46))), gtx.Dp(unit.Dp(11)), colAccent)
	a.drawLabel(gtx, "S", textStyle{size: unit.Sp(27), bold: true, color: color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}}, x+pad+gtx.Dp(unit.Dp(14)), y+pad+gtx.Dp(unit.Dp(7)))

	a.drawLabel(gtx, "SailMark", textStyle{size: unit.Sp(30), bold: true, color: colText}, x+pad, y+pad+gtx.Dp(unit.Dp(62)))
	a.drawLabel(gtx, "轻量、专注的 Markdown 编辑器", textStyle{size: unit.Sp(14.5), color: colTextMuted}, x+pad, y+pad+gtx.Dp(unit.Dp(104)))
	a.drawLabel(gtx, "打开文件后，点击任意段落即可编辑；点击空白处恢复渲染。", textStyle{size: unit.Sp(13), color: colTextFaint}, x+pad, y+pad+gtx.Dp(unit.Dp(132)))
	a.drawLabel(gtx, "Ctrl+O 打开  ·  Ctrl+S 保存  ·  Ctrl+F 查找", textStyle{size: unit.Sp(12.5), color: colTextFaint}, x+pad, y+pad+gtx.Dp(unit.Dp(154)))

	btnW := gtx.Dp(unit.Dp(188))
	btnH := gtx.Dp(unit.Dp(40))
	bx := x + pad
	by := y + cardH - pad - btnH
	if !a.dialogBusy && a.welcomeOpen.Clicked(gtx) {
		a.requestOpen()
	}
	o := op.Offset(image.Pt(bx, by)).Push(gtx.Ops)
	c := gtx
	c.Constraints.Min = image.Pt(btnW, btnH)
	c.Constraints.Max = image.Pt(btnW, btnH)
	a.welcomeOpen.Layout(c, func(gtx layout.Context) layout.Dimensions {
		m := op.Record(gtx.Ops)
		dims := layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.labelNow(gtx, "打开 Markdown 文件", textStyle{size: unit.Sp(14), bold: true, color: color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}})
		})
		call := m.Stop()
		fillRRect(gtx.Ops, image.Rect(0, 0, btnW, btnH), gtx.Dp(unit.Dp(8)), colAccent)
		call.Add(gtx.Ops)
		return dims
	})
	o.Pop()
}

func (a *App) layoutModal(gtx layout.Context) {
	size := gtx.Constraints.Max
	overlay := color.NRGBA{R: 0x0F, G: 0x17, B: 0x24, A: 0x66}
	fillRect(gtx.Ops, image.Rectangle{Max: size}, overlay)
	area := clip.Rect(image.Rectangle{Max: size}).Push(gtx.Ops)
	a.modalClick.Add(gtx.Ops)
	area.Pop()
	_, _ = a.modalClick.Update(gtx.Source)

	cardW := gtx.Dp(unit.Dp(440))
	if cardW > size.X-gtx.Dp(unit.Dp(32)) {
		cardW = size.X - gtx.Dp(unit.Dp(32))
	}
	cardH := gtx.Dp(unit.Dp(210))
	x := (size.X - cardW) / 2
	y := (size.Y - cardH) / 2
	radius := gtx.Dp(unit.Dp(12))
	fillRRect(gtx.Ops, image.Rect(x, y, x+cardW, y+cardH), radius, colBorder)
	fillRRect(gtx.Ops, image.Rect(x+1, y+1, x+cardW-1, y+cardH-1), radius-1, colSurface)

	pad := gtx.Dp(unit.Dp(24))
	a.drawLabel(gtx, "文件尚未保存", textStyle{size: unit.Sp(20), bold: true, color: colText}, x+pad, y+pad)
	fname := a.currentFileName()
	a.drawLabel(gtx, "“"+fname+"” 有未保存的更改。", textStyle{size: unit.Sp(13.5), color: colTextMuted}, x+pad, y+pad+gtx.Dp(unit.Dp(38)))
	a.drawLabel(gtx, "是否先保存再继续？", textStyle{size: unit.Sp(13.5), color: colTextMuted}, x+pad, y+pad+gtx.Dp(unit.Dp(60)))

	btnY := y + cardH - gtx.Dp(unit.Dp(64))
	saveW := gtx.Dp(unit.Dp(82))
	discardW := gtx.Dp(unit.Dp(82))
	cancelW := gtx.Dp(unit.Dp(66))
	gap := gtx.Dp(unit.Dp(8))
	total := saveW + discardW + cancelW + gap*2
	bx := x + cardW - pad - total
	button := func(w int, px int, text string, btn *widget.Clickable, primary bool, action func()) {
		o := op.Offset(image.Pt(px, btnY)).Push(gtx.Ops)
		c := gtx
		c.Constraints.Min = image.Pt(w, gtx.Dp(unit.Dp(38)))
		c.Constraints.Max = image.Pt(w, gtx.Dp(unit.Dp(38)))
		_ = a.textButton(c, text, btn, true, primary, action)
		o.Pop()
	}
	button(saveW, bx, "保存", &a.modalSave, true, func() {
		if a.dialogBusy {
			return
		}
		if a.saveDocument() {
			a.performPending()
		}
	})
	button(discardW, bx+saveW+gap, "不保存", &a.modalDiscard, false, func() {
		if !a.dialogBusy {
			a.performPending()
		}
	})
	button(cancelW, bx+saveW+discardW+gap*2, "取消", &a.modalCancel, false, func() {
		if !a.dialogBusy {
			a.closeModal()
		}
	})
}
