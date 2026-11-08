//go:build !windows

package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func openFileDialog(title string) (string, error) {
	path, ok := zenity([]string{"--file-selection", "--title=" + title})
	if ok {
		return path, nil
	}
	return "", fmt.Errorf("当前构建没有图形文件选择器，请从命令行打开文件")
}

func saveFileDialog(title, initial string) (string, error) {
	args := []string{"--file-selection", "--save", "--confirm-overwrite", "--title=" + title}
	if initial != "" {
		args = append(args, "--filename="+initial)
	}
	path, ok := zenity(args)
	if ok {
		return path, nil
	}
	return "", fmt.Errorf("当前构建没有图形保存对话框")
}

func zenity(args []string) (string, bool) {
	bin, err := exec.LookPath("zenity")
	if err != nil {
		return "", false
	}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}
