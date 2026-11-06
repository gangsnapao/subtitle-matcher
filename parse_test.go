package main

import (
	"os"
	"testing"
)

// 用真实抓取的 SubtitleCat 页面验证 Downloads 提取与机翻识别
func TestParseSCSearchReal(t *testing.T) {
	// MIDA-726 页面：4 个结果
	page, err := os.ReadFile("/tmp/sc_MIDA-726.html")
	if err != nil {
		t.Skip("无真实页面，跳过")
	}
	results := parseSCSearch(string(page))
	if len(results) == 0 {
		t.Fatal("应解析出结果")
	}
	// 找到 MIDA-726 主条目（👍 17 downloads）
	found := false
	for _, r := range results {
		if r.name == "MIDA-726" && r.downloads == 17 {
			found = true
		}
	}
	if !found {
		t.Errorf("MIDA-726 应解析出 downloads=17，实际 %+v", results)
	}
	// whisperjav 应被识别为机翻
	for _, r := range results {
		if r.name == "489155.com@MIDA-726-U.ja.whisperjav" && !isMachineTitle(r.name) {
			t.Errorf("whisperjav 应识别为机翻")
		}
	}
}
