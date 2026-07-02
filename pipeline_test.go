package main

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"The Witcher 3: Wild Hunt™":                           "the witcher 3 wild hunt",
		"DARK SOULS™ III":                                     "dark souls 3",
		"Sid Meier's Civilization® VI":                        "sid meier s civilization 6",
		"The Witcher 3: Wild Hunt - Game of the Year Edition": "the witcher 3 wild hunt",
		"Resident Evil 4 (Remastered)":                        "resident evil 4",
	}
	for in, want := range cases {
		got := romanToArabicTokens(normalize(in))
		if got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQueryVariants(t *testing.T) {
	// 期待値は正規化後の文字列で比較する（前後の空白などの揺れを無視するため）。
	cases := []struct {
		in       string
		wantNorm []string
	}{
		// 分割対象なし → 元の名前のみ
		{"The Witcher 3: Wild Hunt", []string{"the witcher 3 wild hunt"}},
		// 語中のハイフンは分割しない（" - " のみ対象）
		{"Half-Life", []string{"half life"}},
		// "(" 以降を削除した候補が追加される
		{"Foo Bar (Bonus)", []string{"foo bar bonus", "foo bar"}},
		// " - " 以降を削除した候補が追加される
		{"Foo Bar - Bonus Pack", []string{"foo bar bonus pack", "foo bar"}},
		// "(" と " - " の両方
		{"Foo (Bonus) - Extra", []string{"foo bonus extra", "foo", "foo bonus"}},
		// 正規化後に元と同じになる候補は重複として除外される
		// （"deluxe edition" はエディション語として除去されるため "doom" に収束）
		{"DOOM - Deluxe Edition", []string{"doom"}},
	}
	for _, c := range cases {
		got := queryVariants(c.in)
		var gotNorm []string
		for _, v := range got {
			gotNorm = append(gotNorm, normalize(v))
		}
		if strings.Join(gotNorm, "|") != strings.Join(c.wantNorm, "|") {
			t.Errorf("queryVariants(%q) → %v, want %v", c.in, gotNorm, c.wantNorm)
		}
	}
}

func TestSimilarity(t *testing.T) {
	// 部分包含・表記ゆれでも高スコアになること
	pairs := []struct {
		a, b   string
		minVal float64
	}{
		{"The Witcher 3", "The Witcher 3: Wild Hunt", 0.7},
		{"DARK SOULS III", "Dark Souls III", 0.95},
		{"FINAL FANTASY VII", "Final Fantasy 7", 0.95},
		{"Portal 2", "Portal", 0.6},
	}
	for _, p := range pairs {
		s := similarity(p.a, p.b)
		if s < p.minVal {
			t.Errorf("similarity(%q,%q) = %.2f, want >= %.2f", p.a, p.b, s, p.minVal)
		}
	}
	// 無関係なものは低スコア
	if s := similarity("Portal 2", "Grand Theft Auto V"); s > 0.4 {
		t.Errorf("無関係なペアの類似度が高すぎる: %.2f", s)
	}
}

func TestHLTBLiveSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("ネットワークテストのため -short ではスキップ")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	hltb := NewHLTBClient(client)

	res, err := hltb.Search(searchQuery("The Witcher 3: Wild Hunt"))
	if err != nil {
		t.Fatalf("HLTB 検索に失敗: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("検索結果が0件")
	}
	best, score := BestMatch("The Witcher 3: Wild Hunt", res)
	if best == nil || score < 0.8 {
		t.Fatalf("Witcher 3 を高スコアで一致できず: %v score=%.2f", best, score)
	}
	if best.HoursMain() < 30 || best.HoursMain() > 120 {
		t.Errorf("Witcher 3 のメイン時間が不自然: %.1f h", best.HoursMain())
	}
	t.Logf("一致: %q score=%.2f main=%.1fh plus=%.1fh 100%%=%.1fh",
		best.GameName, score, best.HoursMain(), best.HoursPlus(), best.Hours100())
}

// TestGenerateDemoReport は有名タイトルのモック Steam データを実際の HLTB と
// 照合し、サンプル HTML を出力する（目視確認用）。
func TestGenerateDemoReport(t *testing.T) {
	if testing.Short() {
		t.Skip("ネットワークテストのため -short ではスキップ")
	}
	out := os.Getenv("DEMO_OUT")
	if out == "" {
		t.Skip("DEMO_OUT 未指定のためスキップ")
	}

	mock := []SteamGame{
		{AppID: 292030, Name: "The Witcher 3: Wild Hunt", PlaytimeForever: 6200, RTimeLastPlayed: 1700000000},
		{AppID: 374320, Name: "DARK SOULS III", PlaytimeForever: 3000},
		{AppID: 620, Name: "Portal 2", PlaytimeForever: 540},
		{AppID: 271590, Name: "Grand Theft Auto V", PlaytimeForever: 12000, RTimeLastPlayed: 1710000000},
		{AppID: 1245620, Name: "ELDEN RING", PlaytimeForever: 0},
		{AppID: 1091500, Name: "Cyberpunk 2077", PlaytimeForever: 1800},
		{AppID: 413150, Name: "Stardew Valley", PlaytimeForever: 4500},
		{AppID: 570, Name: "Dota 2", PlaytimeForever: 0},
		{AppID: 252490, Name: "Rust", PlaytimeForever: 200},
		{AppID: 1086940, Name: "Baldur's Gate 3", PlaytimeForever: 7200, RTimeLastPlayed: 1715000000},
		{AppID: 8930, Name: "Sid Meier's Civilization V", PlaytimeForever: 9000},
		{AppID: 322330, Name: "Don't Starve Together", PlaytimeForever: 0},
	}

	client := &http.Client{Timeout: 30 * time.Second}
	hltb := NewHLTBClient(client)
	limiter := newRateLimiter(250 * time.Millisecond)

	var rows []Row
	for _, g := range mock {
		limiter.wait()
		cands, err := hltb.Search(searchQuery(g.Name))
		if err != nil {
			t.Logf("検索失敗 %s: %v", g.Name, err)
			rows = append(rows, Row{Steam: g})
			continue
		}
		best, score := BestMatch(g.Name, cands)
		rows = append(rows, Row{Steam: g, Match: best, Score: score, Matched: best != nil && score >= 0.5})
		t.Logf("%-32s -> %-34s score=%.2f main=%.1fh", g.Name, nameOf(best), score, hoursOf(best))
	}

	if err := BuildReport(rows, out, 0.5); err != nil {
		t.Fatalf("レポート生成失敗: %v", err)
	}
	t.Logf("デモレポート出力: %s", out)
}

// TestRenderSample は固定のモックデータ（ネットワーク不要）から sample.html を
// 生成する。テンプレート（プレイ時間フィルタ・言語切替）のデザイン確認用。
// 生成するには: SAMPLE_OUT=sample.html go test -run TestRenderSample
func TestRenderSample(t *testing.T) {
	out := os.Getenv("SAMPLE_OUT")
	if out == "" {
		t.Skip("SAMPLE_OUT 未指定のためスキップ")
	}
	hToSec := func(h float64) int { return int(h*3600 + 0.5) }

	type mock struct {
		appID            int
		steam            string
		playMin          int
		lastPlayed       int64
		hltbID           int
		hltbName         string
		year             int
		main, plus, hund float64
		score            float64 // 0 のとき未一致（Match=nil 扱い）
	}
	data := []mock{
		{271590, "Grand Theft Auto V", 12000, 1710000000, 4064, "Grand Theft Auto V", 2013, 32.0, 51.5, 88.8, 1.0},
		{8930, "Sid Meier's Civilization V", 9000, 0, 8510, "Sid Meier's Civilization V", 2010, 39.7, 123.7, 430.8, 1.0},
		{1086940, "Baldur's Gate 3", 7200, 1715000000, 68033, "Baldur's Gate 3", 2023, 72.9, 116.3, 180.9, 1.0},
		{292030, "The Witcher 3: Wild Hunt", 6200, 1700000000, 10270, "The Witcher 3: Wild Hunt", 2015, 51.6, 103.7, 174.5, 1.0},
		{413150, "Stardew Valley", 4500, 0, 34716, "Stardew Valley", 2016, 53.4, 94.7, 171.8, 1.0},
		{374320, "DARK SOULS III", 3000, 0, 26803, "Dark Souls III", 2016, 31.2, 48.5, 100.4, 1.0},
		{1091500, "Cyberpunk 2077", 1800, 0, 2127, "Cyberpunk 2077", 2020, 26.0, 63.1, 108.6, 1.0},
		{431960, "Wallpaper Engine", 1500, 0, 0, "", 0, 0, 0, 0, 0},                 // 未一致（HLTB に無い）
		{1145350, "Hades II", 600, 0, 64419, "Hades", 2020, 21.2, 47.1, 95.6, 0.42}, // 要確認（低スコア）
		{620, "Portal 2", 540, 0, 7231, "Portal 2", 2011, 8.5, 13.7, 22.5, 1.0},
		{252490, "Rust", 200, 0, 15596, "Rust", 2018, 30.0, 105.8, 116.7, 1.0},
		{322330, "Don't Starve Together", 0, 0, 23828, "Don't Starve Together", 2014, 34.1, 109.9, 213.7, 1.0},
		{570, "Dota 2", 0, 0, 2721, "Dota 2", 2013, 746.8, 1140.4, 2332.7, 1.0},
		{1245620, "ELDEN RING", 0, 0, 68151, "Elden Ring", 2022, 60.0, 101.2, 135.6, 1.0},
	}

	const threshold = 0.5
	var rows []Row
	for _, m := range data {
		r := Row{Steam: SteamGame{AppID: m.appID, Name: m.steam, PlaytimeForever: m.playMin, RTimeLastPlayed: m.lastPlayed}}
		if m.score > 0 {
			r.Match = &HLTBGame{
				GameID: m.hltbID, GameName: m.hltbName, ReleaseWorld: m.year,
				CompMain: hToSec(m.main), CompPlus: hToSec(m.plus), Comp100: hToSec(m.hund),
			}
			r.Score = m.score
			r.Matched = m.score >= threshold
		}
		rows = append(rows, r)
	}

	if err := BuildReport(rows, out, threshold); err != nil {
		t.Fatalf("sample 生成失敗: %v", err)
	}
	t.Logf("sample 出力: %s（%d 行）", out, len(rows))
}

func nameOf(g *HLTBGame) string {
	if g == nil {
		return "(no match)"
	}
	return g.GameName
}
func hoursOf(g *HLTBGame) float64 {
	if g == nil {
		return 0
	}
	return g.HoursMain()
}
