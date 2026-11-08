//go:build windows

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	ofnReadOnly        = 0x00000001
	ofnOverwritePrompt = 0x00000002
	ofnNoChangeDir     = 0x00000008
	ofnPathMustExist   = 0x00000800
	ofnFileMustExist   = 0x00001000
	ofnExplorer        = 0x00080000
)

type openFileNameW struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        unsafe.Pointer
	dwReserved        uint32
	flagsEx           uint32
}

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	comdlg32                = syscall.NewLazyDLL("comdlg32.dll")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetOpenFileName     = comdlg32.NewProc("GetOpenFileNameW")
	procGetSaveFileName     = comdlg32.NewProc("GetSaveFileNameW")
)

func utf16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

func filterPtr() *uint16 {
	filter := "Markdown 文件 (*.md;*.markdown)\x00*.md;*.markdown\x00所有文件 (*.*)\x00*.*\x00\x00"
	buf := utf16.Encode([]rune(filter))
	return &buf[0]
}

func runFileDialog(title, initial string, save bool) (string, error) {
	fileBuf := make([]uint16, 32768)
	if initial != "" {
		encoded := utf16.Encode([]rune(initial))
		if len(encoded) >= len(fileBuf) {
			encoded = encoded[:len(fileBuf)-1]
		}
		copy(fileBuf, encoded)
	}
	filter := filterPtr()
	titlePtr := utf16Ptr(title)
	ext := utf16Ptr("md")
	flags := uint32(ofnExplorer | ofnNoChangeDir | ofnPathMustExist)
	if save {
		flags |= ofnOverwritePrompt
	} else {
		flags |= ofnFileMustExist
	}
	ofn := openFileNameW{
		hwndOwner:    0,
		lpstrFilter:  filter,
		nFilterIndex: 1,
		lpstrFile:    &fileBuf[0],
		nMaxFile:     uint32(len(fileBuf)),
		lpstrTitle:   titlePtr,
		flags:        flags,
	}
	if save {
		ofn.lpstrDefExt = ext
	} else {
		ofn.lpstrDefExt = ext
	}
	ofn.lStructSize = uint32(unsafe.Sizeof(ofn))
	var r uintptr
	if save {
		r, _, _ = procGetSaveFileName.Call(uintptr(unsafe.Pointer(&ofn)))
	} else {
		r, _, _ = procGetOpenFileName.Call(uintptr(unsafe.Pointer(&ofn)))
	}
	runtime.KeepAlive(fileBuf)
	runtime.KeepAlive(filter)
	runtime.KeepAlive(titlePtr)
	runtime.KeepAlive(ext)
	if r == 0 {
		// Zero means cancel or error. The common error path here is a user
		// cancel, so treat it as a no-op rather than an alarming failure.
		return "", nil
	}
	path := syscall.UTF16ToString(fileBuf)
	if path == "" {
		return "", fmt.Errorf("empty file name")
	}
	return path, nil
}

func openFileDialog(title string) (string, error) {
	return runFileDialog(title, "", false)
}

func saveFileDialog(title, initial string) (string, error) {
	return runFileDialog(title, initial, true)
}
