package main

import "testing"

// 验证字幕筛选：非机翻优先 → 下载次数多优先 → Size 兜底
func TestRankCandidatesFilter(t *testing.T) {
	in := []candidate{
		{Source: "机翻-下载多", Lang: langSimp, LangRank: 2, Size: 60 * 1024, Downloads: 50, IsMachine: true},
		{Source: "机翻-下载少", Lang: langSimp, LangRank: 2, Size: 70 * 1024, Downloads: 5, IsMachine: true},
		{Source: "人工-下载少", Lang: langSimp, LangRank: 2, Size: 80 * 1024, Downloads: 3, IsMachine: false},
		{Source: "人工-下载多", Lang: langSimp, LangRank: 2, Size: 10 * 1024, Downloads: 20, IsMachine: false},
		{Source: "人工-下载中", Lang: langSimp, LangRank: 2, Size: 90 * 1024, Downloads: 10, IsMachine: false},
	}
	got := rankCandidates(in)
	want := []string{"人工-下载多", "人工-下载中", "人工-下载少", "机翻-下载多", "机翻-下载少"}
	if len(got) != len(want) {
		t.Fatalf("应保留 %d 个，got %d", len(want), len(got))
	}
	for i, w := range want {
		if got[i].Source != w {
			t.Errorf("第 %d 位应为 %s，got %s（完整 %v）", i, w, got[i].Source, srcs(got))
		}
	}
}

// 验证机翻关键词识别
func TestIsMachineTitle(t *testing.T) {
	cases := []struct {
		title string
		want  bool
	}{
		{"489155.com@MIDA-726-U.ja.whisperjav", true},
		{"CAWB-040.zh-cn(by transub)", true},
		{"MIDA-726", false},
		{"MIDA-726-简体中文", false},
		{"CAWB-040-chs NOT AI", false},
		{"SNOS-115", false},
	}
	for _, c := range cases {
		if got := isMachineTitle(c.title); got != c.want {
			t.Errorf("isMachineTitle(%q) = %v, want %v", c.title, got, c.want)
		}
	}
}
