package main

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// SailMark uses a quiet, paper-like palette so that Markdown content stays
// the visual focus.
var (
	colBackground   = color.NRGBA{R: 0xFB, G: 0xFC, B: 0xFD, A: 0xFF}
	colSurface      = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colSurfaceAlt   = color.NRGBA{R: 0xF6, G: 0xF8, B: 0xFA, A: 0xFF}
	colBorder       = color.NRGBA{R: 0xD8, G: 0xDE, B: 0xE4, A: 0xFF}
	colBorderSoft   = color.NRGBA{R: 0xEA, G: 0xEE, B: 0xF2, A: 0xFF}
	colText         = color.NRGBA{R: 0x1F, G: 0x23, B: 0x28, A: 0xFF}
	colTextMuted    = color.NRGBA{R: 0x57, G: 0x60, B: 0x6A, A: 0xFF}
	colTextFaint    = color.NRGBA{R: 0x8C, G: 0x95, B: 0x9F, A: 0xFF}
	colAccent       = color.NRGBA{R: 0x2F, G: 0x6F, B: 0xEB, A: 0xFF}
	colAccentSoft   = color.NRGBA{R: 0xE8, G: 0xF1, B: 0xFF, A: 0xFF}
	colAccentHover  = color.NRGBA{R: 0xD7, G: 0xE7, B: 0xFF, A: 0xFF}
	colLink         = color.NRGBA{R: 0x09, G: 0x69, B: 0xDA, A: 0xFF}
	colCode         = color.NRGBA{R: 0x6F, G: 0x42, B: 0xC1, A: 0xFF}
	colCodeBg       = color.NRGBA{R: 0xEE, G: 0xF1, B: 0xF5, A: 0xFF}
	colCodeBlockBg  = color.NRGBA{R: 0xF6, G: 0xF8, B: 0xFA, A: 0xFF}
	colQuoteBar     = color.NRGBA{R: 0xC8, G: 0xD1, B: 0xDA, A: 0xFF}
	colHighlight    = color.NRGBA{R: 0xFF, G: 0xE8, B: 0x8A, A: 0xFF}
	colHighlightCur = color.NRGBA{R: 0xFF, G: 0xC2, B: 0x4D, A: 0xFF}
	colDanger       = color.NRGBA{R: 0xCF, G: 0x22, B: 0x2E, A: 0xFF}
	colSuccess      = color.NRGBA{R: 0x1A, G: 0x7F, B: 0x37, A: 0xFF}
	colMenuHover    = color.NRGBA{R: 0xF1, G: 0xF4, B: 0xF7, A: 0xFF}
	colMenuPressed  = color.NRGBA{R: 0xE6, G: 0xEC, B: 0xF2, A: 0xFF}
)

func fillRect(ops *op.Ops, r image.Rectangle, c color.NRGBA) {
	paint.FillShape(ops, c, clip.Rect(r).Op())
}

func fillRRect(ops *op.Ops, r image.Rectangle, radius int, c color.NRGBA) {
	paint.FillShape(ops, c, clip.UniformRRect(r, radius).Op(ops))
}

// colorMaterial records a paint material for a text label.
func colorMaterial(ops *op.Ops, c color.NRGBA) op.CallOp {
	m := op.Record(ops)
	paint.ColorOp{Color: c}.Add(ops)
	return m.Stop()
}

func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	c.A = a
	return c
}

func px(v unit.Dp, gtx layout.Context) int {
	return gtx.Dp(v)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
