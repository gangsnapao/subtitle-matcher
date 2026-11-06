// cli.go —— 命令行流程，Windows 与 Linux 共用
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

const cliUsage = `番号字幕匹配下载器（控制台模式）

用法:
  subtitle-matcher -cli [选项] [目录]

选项:
  -all          不过滤番号，处理所有视频文件
  -raw          原样保存字幕，不修正时间轴格式
  -jobs N       同时处理的视频文件数（默认 3）
  -h            显示本帮助
`

// runCLI 命令行流程，扫描目录并下载中文字幕。
func runCLI(args []string) int {
	root := ""
	onlyCodes := true
	normalize := true
	jobs := 3

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-cli", "--cli":
		case "-all":
			onlyCodes = false
		case "-raw":
			normalize = false
		case "-jobs":
			if i+1 < len(args) {
				i++
				if n, err := atoi(args[i]); err == nil && n > 0 {
					jobs = n
				}
			}
		case "-h", "--help":
			fmt.Print(cliUsage)
			return 0
		default:
			if !strings.HasPrefix(a, "-") {
				root = a
			}
		}
	}

	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Println("无法确定当前目录:", err)
			return 1
		}
		root = wd
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		fmt.Printf("目录不存在或不可访问: %s\n", abs)
		return 1
	}

	cfg := config{OnlyCodes: onlyCodes, Normalize: normalize, SkipExisting: true, Concurrency: jobs}

	targets, ignored, err := scanVideos(abs, cfg.OnlyCodes)
	if err != nil {
		fmt.Println("读取目录失败:", err)
		return 1
	}
	if len(ignored) > 0 {
		fmt.Printf("已忽略 %d 个不像番号的视频\n", len(ignored))
	}
	if len(targets) == 0 {
		fmt.Println("没有找到需要配字幕的视频文件")
		return 0
	}

	fmt.Printf("目录: %s\n共 %d 个视频文件，并发 %d\n\n", abs, len(targets), cfg.Concurrency)

	var mu sync.Mutex
	printed := map[int]string{}

	sum := runAll(context.Background(), targets, cfg, func(p progress) {
		if p.Status == stSearching {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		line := fmt.Sprintf("[%d/%d] %-22s %-9s %s",
			p.Index+1, len(targets), targets[p.Index].Name,
			p.Status.Text(), p.Detail)
		if printed[p.Index] == line {
			return
		}
		printed[p.Index] = line
		fmt.Println(line)
	})

	fmt.Printf("\n──────── 汇总 ────────\n成功 %d ｜ 跳过 %d ｜ 失败 %d ｜ 合计 %d\n",
		sum.Done, sum.Skipped, sum.Failed, sum.Total)

	if sum.Failed > 0 {
		return 2
	}
	return 0
}

func atoi(s string) (int, error) {
	n := 0
	if s == "" {
		return 0, fmt.Errorf("空字符串")
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("非数字")
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, nil
}
