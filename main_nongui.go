//go:build !windows

// 番号字幕匹配下载器（SubtitleCat Matcher）—— Linux / macOS 命令行 + Web 前端
//
// 用法:
//
//	subtitle-matcher -cli [选项] [目录]     命令行模式
//	subtitle-matcher -web [地址]            启动 Web 前端（默认 :8080）
//
// 命令行选项:
//
//	-all          不过滤番号，处理所有视频文件
//	-raw          原样保存字幕，不修正时间轴格式
//	-jobs N       同时处理的视频文件数（默认 3）
//	-h            显示本帮助
package main

import (
	"os"
)

func main() {
	args := os.Args[1:]
	// Web 模式：subtitle-matcher -web [:8080]
	for i := 0; i < len(args); i++ {
		if args[i] == "-web" || args[i] == "--web" {
			addr := ":8080"
			if i+1 < len(args) && !isFlag(args[i+1]) {
				addr = args[i+1]
			}
			runWeb(addr)
			return
		}
	}
	// 默认命令行模式
	os.Exit(runCLI(args))
}

func isFlag(s string) bool {
	return len(s) > 0 && s[0] == '-'
}
