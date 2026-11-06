//go:build windows

// 番号字幕匹配下载器（SubtitleCat Matcher）—— Windows 图形界面版入口
package main

import (
	"os"
	"strings"
)

const appTitle = "番号字幕匹配下载器 v2.1"

func main() {
	args := os.Args[1:]
	if hasArg(args, "-cli") || hasArg(args, "--cli") {
		os.Exit(runCLI(args))
	}

	// 支持 SubtitleCatMatcher.exe "D:\影片" 直接带目录进界面
	initial := ""
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			initial = a
			break
		}
	}
	os.Exit(runGUI(initial))
}
