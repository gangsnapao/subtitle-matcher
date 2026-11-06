// sortutil.go —— 排序与状态辅助，跨平台共享（不依赖 Win32）
package main

import "strings"

// statusRank 让「需要关注」的状态排在前面，而不是按拼音排。
func statusRank(s string) int {
	switch s {
	case stFailed.Text():
		return 0
	case stDownloading.Text():
		return 1
	case stSearching.Text():
		return 2
	case stWaiting.Text():
		return 3
	case stSkipped.Text():
		return 4
	case stDone.Text():
		return 5
	}
	return 6
}

// naturalLess 让 ABF-2 排在 ABF-10 前面（纯字符串比较会反过来）。
func naturalLess(a, b string) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		da, db := isDigitByte(a[i]), isDigitByte(b[j])
		if da && db {
			si, sj := i, j
			for i < len(a) && isDigitByte(a[i]) {
				i++
			}
			for j < len(b) && isDigitByte(b[j]) {
				j++
			}
			na := strings.TrimLeft(a[si:i], "0")
			nb := strings.TrimLeft(b[sj:j], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			continue
		}
		ca, cb := lowerByte(a[i]), lowerByte(b[j])
		if ca != cb {
			return ca < cb
		}
		i++
		j++
	}
	return len(a)-i < len(b)-j
}

func isDigitByte(c byte) bool { return c >= '0' && c <= '9' }

func lowerByte(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}
