package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/text"
)

type fontSet struct {
	shaper *text.Shaper
	body   font.Font
	mono   font.Font
}

func loadFonts() (*fontSet, error) {
	type candidate struct {
		path string
		mono bool
	}
	var candidates []candidate

	if runtime.GOOS == "windows" {
		win := os.Getenv("WINDIR")
		if win == "" {
			win = `C:\Windows`
		}
		candidates = append(candidates,
			candidate{win + `\Fonts\msyh.ttc`, false},
			candidate{win + `\Fonts\msyhbd.ttc`, false},
			candidate{win + `\Fonts\consola.ttf`, true},
			candidate{win + `\Fonts\consolab.ttf`, true},
		)
	} else {
		// The WSL paths make development pleasant on machines that have a
		// Windows font directory mounted. They are harmless elsewhere.
		candidates = append(candidates,
			candidate{"/mnt/c/Windows/Fonts/msyh.ttc", false},
			candidate{"/mnt/c/Windows/Fonts/msyhbd.ttc", false},
			candidate{"/mnt/c/Windows/Fonts/consola.ttf", true},
			candidate{"/mnt/c/Windows/Fonts/consolab.ttf", true},
			candidate{"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", false},
			candidate{"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", false},
			candidate{"/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf", true},
			candidate{"/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf", true},
		)
	}

	var faces []text.FontFace
	bodyFamily := ""
	monoFamily := ""
	for _, cand := range candidates {
		b, err := os.ReadFile(cand.path)
		if err != nil {
			continue
		}
		ffs, err := opentype.ParseCollection(b)
		if err != nil || len(ffs) == 0 {
			continue
		}
		faces = append(faces, ffs...)
		for _, ff := range ffs {
			name := string(ff.Font.Typeface)
			if name == "" {
				continue
			}
			if cand.mono {
				if monoFamily == "" || (!strings.Contains(strings.ToLower(name), "mono") && !containsAny(monoFamily, "consol", "mono")) {
					monoFamily = name
				}
			} else if bodyFamily == "" {
				bodyFamily = name
			}
		}
	}
	// A .ttc usually carries a near-identical "UI" variant next to the family
	// that is actually referenced. Parsing both doubles the font tables kept
	// in memory, so the unused duplicates are dropped here.
	if len(faces) > 1 {
		kept := faces[:0]
		for _, ff := range faces {
			if strings.Contains(string(ff.Font.Typeface), " UI") {
				continue
			}
			kept = append(kept, ff)
		}
		if len(kept) > 0 {
			faces = kept
		}
		if bodyFamily != "" {
			found := false
			for _, ff := range faces {
				if string(ff.Font.Typeface) == bodyFamily {
					found = true
					break
				}
			}
			if !found && len(faces) > 0 {
				bodyFamily = string(faces[0].Font.Typeface)
			}
		}
	}

	if len(faces) == 0 {
		faces = gofont.Regular()
		for _, ff := range faces {
			if bodyFamily == "" {
				bodyFamily = string(ff.Font.Typeface)
			}
		}
	}
	if bodyFamily == "" {
		bodyFamily = "Go"
	}
	if monoFamily == "" {
		monoFamily = bodyFamily
	}

	// A comma separated typeface list lets the shaper fall back to another
	// loaded family when a glyph is missing. Microsoft YaHei covers both
	// Latin and CJK, Consolas covers Latin code, and the fallback covers CJK
	// inside code blocks.
	body := font.Font{
		Typeface: font.Typeface(bodyFamily + ", Microsoft YaHei, DejaVu Sans, Go"),
	}
	mono := font.Font{
		Typeface: font.Typeface(monoFamily + ", Microsoft YaHei, Consolas, DejaVu Sans Mono, Go"),
	}
	shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(faces))
	if shaper == nil {
		return nil, fmt.Errorf("failed to create text shaper")
	}
	return &fontSet{shaper: shaper, body: body, mono: mono}, nil
}

func containsAny(s string, subs ...string) bool {
	s = strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
