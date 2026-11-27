package main

import (
	"fmt"
	"image"
	"image/color"
	"strings"

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
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.menuButton(gtx, "目录", &a.tocBtn, enabled, a.toggleTOC)
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
				return a.textButton(gtx, "×", &a.findClose, true, false, a.closeFind)
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
		words = a.wordCountValue()
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

func (a *App) layoutTOC(gtx layout.Context) layout.Dimensions {
	w := gtx.Dp(unit.Dp(268))
	if w > gtx.Constraints.Max.X {
		w = gtx.Constraints.Max.X
	}
	gtx.Constraints.Min = image.Pt(w, gtx.Constraints.Max.Y)
	gtx.Constraints.Max = image.Pt(w, gtx.Constraints.Max.Y)

	// Derive the active entry from the heading positions the last viewport
	// pass recorded. Content-space tops are scroll invariant, so combining
	// them with the current scroll keeps the highlight in sync even when the
	// scroll position just settled and no further frame was scheduled.
	a.tocCurrent = -1
	if len(a.tocTops) == len(a.tocEntries) {
		limit := a.scroll + gtx.Constraints.Max.Y/4
		for ei, top := range a.tocTops {
			if top >= 0 && top <= limit {
				a.tocCurrent = ei
			}
		}
	}

	// Follow the document: when the highlighted heading changes, scroll the
	// outline so that heading is visible. The list is only moved when the
	// active entry left the visible range, so manually browsing the outline
	// is not fought over.
	headerH := gtx.Dp(unit.Dp(46))
	listH := gtx.Constraints.Max.Y - headerH
	entryH := gtx.Dp(unit.Dp(30))
	if a.tocCurrent != a.tocShown {
		a.tocShown = a.tocCurrent
		if a.tocCurrent >= 0 && entryH > 0 && listH > 0 {
			visible := listH / entryH
			if visible < 1 {
				visible = 1
			}
			first := a.tocList.Position.First
			if a.tocCurrent < first || a.tocCurrent >= first+visible {
				target := a.tocCurrent - visible/2
				if maxFirst := len(a.tocEntries) - visible; target > maxFirst {
					target = maxFirst
				}
				if target < 0 {
					target = 0
				}
				a.tocList.Position.First = target
				a.tocList.Position.Offset = 0
			}
		}
	}

	fillRect(gtx.Ops, image.Rect(0, 0, w, gtx.Constraints.Max.Y), colSurface)
	fillRect(gtx.Ops, image.Rect(w-1, 0, w, gtx.Constraints.Max.Y), colBorderSoft)

	fillRect(gtx.Ops, image.Rect(0, 0, w, headerH), colSurfaceAlt)
	fillRect(gtx.Ops, image.Rect(0, headerH-1, w, headerH), colBorderSoft)

	titleStyle := textStyle{size: unit.Sp(14), bold: true, color: colText}
	titleDims, titleCall := a.measureText(gtx, "目录", titleStyle)
	off := op.Offset(image.Pt(gtx.Dp(unit.Dp(16)), (headerH-titleDims.Size.Y)/2)).Push(gtx.Ops)
	titleCall.Add(gtx.Ops)
	off.Pop()
	if n := len(a.tocEntries); n > 0 {
		countStyle := textStyle{size: unit.Sp(11.5), color: colTextFaint}
		cx := gtx.Dp(unit.Dp(16)) + titleDims.Size.X + gtx.Dp(unit.Dp(10))
		a.drawLabel(gtx, fmt.Sprintf("%d 个标题", n), countStyle, cx, (headerH-gtx.Sp(unit.Sp(16)))/2)
	}

	btnW := gtx.Dp(unit.Dp(34))
	btnH := gtx.Dp(unit.Dp(30))
	bo := op.Offset(image.Pt(w-btnW-gtx.Dp(unit.Dp(8)), (headerH-btnH)/2)).Push(gtx.Ops)
	bc := gtx
	bc.Constraints.Min = image.Pt(btnW, btnH)
	bc.Constraints.Max = image.Pt(btnW, btnH)
	_ = a.textButton(bc, "×", &a.tocClose, true, false, func() {
		a.tocOpen = false
		a.invalidate()
	})
	bo.Pop()

	lc := gtx
	lc.Constraints.Min = image.Pt(w, 0)
	lc.Constraints.Max = image.Pt(w, gtx.Constraints.Max.Y-headerH)
	lo := op.Offset(image.Pt(0, headerH)).Push(gtx.Ops)
	if len(a.tocEntries) == 0 {
		a.drawLabel(lc, "文档中没有标题", textStyle{size: unit.Sp(12.5), color: colTextFaint}, gtx.Dp(unit.Dp(16)), gtx.Dp(unit.Dp(14)))
	} else {
		a.tocList.Axis = layout.Vertical
		a.tocList.Layout(lc, len(a.tocEntries), a.layoutTOCEntry)
	}
	lo.Pop()
	return layout.Dimensions{Size: image.Pt(w, gtx.Constraints.Max.Y)}
}

func (a *App) layoutTOCEntry(gtx layout.Context, i int) layout.Dimensions {
	ent := a.tocEntries[i]
	btn := &a.tocClicks[i]
	if btn.Clicked(gtx) {
		a.tocJump = i
		gtx.Execute(op.InvalidateCmd{})
	}
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		w := gtx.Constraints.Max.X
		h := gtx.Dp(unit.Dp(30))
		active := i == a.tocCurrent
		st := textStyle{size: unit.Sp(12.5), color: colTextMuted}
		if ent.Level <= 2 {
			st.bold = true
		}
		if active {
			st.color = colAccent
		} else if btn.Hovered() {
			st.color = colText
		}
		if active || btn.Hovered() {
			bg := colMenuHover
			if active {
				bg = colAccentSoft
			}
			fillRect(gtx.Ops, image.Rect(0, 0, w, h), bg)
		}
		if active {
			fillRect(gtx.Ops, image.Rect(0, 0, gtx.Dp(unit.Dp(3)), h), colAccent)
		}
		indent := gtx.Dp(unit.Dp(12)) + gtx.Dp(unit.Dp(float32(ent.Level-1)*13))
		maxW := w - indent - gtx.Dp(unit.Dp(12))
		if maxW < 40 {
			maxW = 40
		}
		m := op.Record(gtx.Ops)
		dims := a.labelTruncated(gtx, ent.Title, st, maxW, text.Start)
		call := m.Stop()
		y := (h - dims.Size.Y) / 2
		if y < 0 {
			y = 0
		}
		off := op.Offset(image.Pt(indent, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		off.Pop()
		return layout.Dimensions{Size: image.Pt(w, h)}
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

		// Pass 1: position every block from its measured height, or from an
		// estimate while it has never been on screen. Pass 2 then lays out
		// only the blocks near the viewport, so a frame costs what is visible
		// rather than what the file contains.
		if a.syncBlockHeights(contentW) {
			a.anchorPending = true
		}
		a.blockTops = a.blockTops[:0]
		y := topPad
		for i := range a.doc.Blocks {
			h := a.blockHeights[i]
			if h <= 0 {
				h = a.estimateBlockHeight(gtx, a.doc.Blocks[i], contentW)
				a.blockHeights[i] = h
			}
			a.blockTops = append(a.blockTops, y)
			y += h + gap
		}
		a.contentH = y - gap + bottomPad

		// Keep the reading position when the content width changes (outline
		// toggled, window resized): the block at the top stays where it was.
		if a.anchorPending {
			a.anchorPending = false
			if a.scroll > 0 && len(a.blockTops) > 0 {
				blk := clamp(a.anchorBlock, 0, len(a.blockTops)-1)
				a.scroll = a.blockTops[blk] + a.anchorOffset
			}
		}
		maxScrollNow := 0
		if a.contentH > size.Y {
			maxScrollNow = a.contentH - size.Y
		}
		a.scroll = clamp(a.scroll, 0, maxScrollNow)

		overscan := size.Y
		viewTop := a.scroll - overscan
		viewBottom := a.scroll + size.Y + overscan
		forceIdx := -1
		if a.tocJump >= 0 && a.tocJump < len(a.tocEntries) {
			forceIdx = a.tocEntries[a.tocJump].Block
		}
		if a.pendingScrollBlock >= 0 {
			forceIdx = a.pendingScrollBlock
		}
		a.blockRects = a.blockRects[:0]
		a.editRect = image.Rectangle{}
		scrollAdjust := 0
		for i := range a.doc.Blocks {
			top := a.blockTops[i]
			h := a.blockHeights[i]
			screenTop := top - a.scroll
			if a.editIdx != i && i != forceIdx && (top+h < viewTop || top > viewBottom) {
				a.blockRects = append(a.blockRects, image.Rect(x, screenTop, x+contentW, screenTop+h))
				continue
			}
			offset := op.Offset(image.Pt(x, screenTop)).Push(gtx.Ops)
			var dims layout.Dimensions
			if a.editIdx == i {
				dims = a.layoutEditCard(gtx, i, contentW)
				a.editRect = a.editRect.Add(image.Pt(x, screenTop))
			} else {
				dims = a.layoutBlock(gtx, i, contentW)
			}
			offset.Pop()
			if dims.Size.Y > 0 {
				if top+h <= a.scroll && dims.Size.Y != h {
					// A block above the viewport was taller or shorter than
					// estimated; compensate so the visible text does not jump.
					scrollAdjust += dims.Size.Y - h
				}
				a.blockHeights[i] = dims.Size.Y
			}
			a.blockRects = append(a.blockRects, image.Rect(x, screenTop, x+contentW, screenTop+dims.Size.Y))
		}

		// Remember the block at the top of the viewport so a later width
		// change can restore the same reading position.
		if len(a.blockTops) > 0 {
			blk := 0
			for i := len(a.blockTops) - 1; i >= 0; i-- {
				if a.blockTops[i] <= a.scroll {
					blk = i
					break
				}
			}
			a.anchorBlock = blk
			a.anchorOffset = a.scroll - a.blockTops[blk]
		}

		if scrollAdjust != 0 {
			a.scroll = clamp(a.scroll+scrollAdjust, 0, maxScrollNow)
			a.anchorPending = true
			gtx.Execute(op.InvalidateCmd{})
		}

		// Record heading tops in content space. The outline panel reads these
		// when it draws, so its highlight follows scrolling without waiting
		// for another viewport pass.
		if len(a.tocEntries) > 0 {
			if cap(a.tocTops) < len(a.tocEntries) {
				a.tocTops = make([]int, len(a.tocEntries))
			}
			a.tocTops = a.tocTops[:len(a.tocEntries)]
			for ei, ent := range a.tocEntries {
				a.tocTops[ei] = -1
				if ent.Block >= 0 && ent.Block < len(a.blockRects) {
					a.tocTops[ei] = a.blockRects[ent.Block].Min.Y + a.scroll
				}
			}
		}

		newMax := 0
		if a.contentH > size.Y {
			newMax = a.contentH - size.Y
		}
		if a.scroll > newMax {
			a.scroll = newMax
			gtx.Execute(op.InvalidateCmd{})
		}

		// A click on an outline entry lands the heading near the top. While a
		// block is being edited, the click commits the edit right after this
		// frame and may rebuild the blocks, so keep the jump pending until the
		// document is consistent again.
		if a.tocJump >= 0 && a.editIdx < 0 && a.tocJump < len(a.tocEntries) {
			blk := a.tocEntries[a.tocJump].Block
			a.tocJump = -1
			if blk >= 0 && blk < len(a.blockRects) {
				a.scroll = clamp(a.blockRects[blk].Min.Y+a.scroll-gtx.Dp(unit.Dp(16)), 0, newMax)
			}
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
	a.drawLabel(gtx, "Ctrl+O 打开  ·  Ctrl+S 保存  ·  Ctrl+F 查找  ·  Ctrl+T 目录", textStyle{size: unit.Sp(12.5), color: colTextFaint}, x+pad, y+pad+gtx.Dp(unit.Dp(154)))

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

// estimateBlockHeight approximates a block's height without shaping its text.
// Blocks outside the viewport are positioned with this estimate until they
// scroll into view and get measured for real, which keeps the scrollbar and
// the scroll range close to the truth without paying for a full layout.
func (a *App) estimateBlockHeight(gtx layout.Context, b *Block, width int) int {
	if b == nil {
		return gtx.Dp(unit.Dp(30))
	}
	display := b.Display
	if display == nil {
		return gtx.Dp(unit.Dp(30))
	}

	switch b.Kind {
	case kindRule:
		return gtx.Dp(unit.Dp(28))
	case kindCode:
		lines := 1 + strings.Count(display.Code, "\n")
		h := gtx.Dp(unit.Dp(16+28)) + lines*gtx.Sp(unit.Sp(21))
		if strings.TrimSpace(display.CodeLang) != "" {
			h += gtx.Sp(unit.Sp(19))
		}
		return h
	case kindTable:
		rows := 1
		if display.Table != nil {
			rows += len(display.Table.Rows)
		}
		return rows*gtx.Sp(unit.Sp(34)) + gtx.Dp(unit.Dp(16))
	}

	size := unit.Sp(16)
	lineSp := unit.Sp(28)
	padTop := 12
	padBottom := 12
	avail := width
	switch b.Kind {
	case kindHeading:
		level := clamp(b.Level, 1, 6)
		size = headingSize(level)
		lineSp = unit.Sp(float32(size) * 1.75)
		padTop = 20
		padBottom = 9
		if level <= 2 {
			padBottom += 4
		}
	case kindQuote:
		size = unit.Sp(15.5)
		lineSp = unit.Sp(26)
		padTop = 10
		padBottom = 10
		avail = width - gtx.Dp(unit.Dp(18))
	case kindList:
		lineSp = unit.Sp(27)
		padTop = 9
		padBottom = 9
		avail = width - gtx.Dp(unit.Dp(34))
	}
	if avail < 80 {
		avail = 80
	}

	textW := 0
	for _, r := range display.Plain {
		if isCJK(r) {
			textW += gtx.Sp(size)
		} else {
			textW += gtx.Sp(unit.Sp(float32(size) * 0.52))
		}
	}
	lines := 1
	if textW > avail {
		lines = (textW + avail - 1) / avail
	}
	return lines*gtx.Sp(lineSp) + gtx.Dp(unit.Dp(float32(padTop+padBottom)))
}
