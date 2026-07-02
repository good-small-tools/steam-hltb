package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderServeMode(t *testing.T) {
	rows := []Row{
		{Steam: SteamGame{AppID: 620, Name: "Portal 2", PlaytimeForever: 300},
			Match: &HLTBGame{GameID: 7231, GameName: "Portal 2", CompMain: 30600}, Score: 1.0, Matched: true},
	}

	// サーバーモード: 更新ボタンと自動更新メタが入る
	var serveBuf bytes.Buffer
	if err := RenderReport(&serveBuf, rows, 0.5, true, 60); err != nil {
		t.Fatalf("RenderReport(serve) 失敗: %v", err)
	}
	html := serveBuf.String()
	// 更新ボタン要素の有無で判定する（ラベル文言は i18n 辞書に常に含まれるため、
	// テキスト一致ではモード差を判別できない）。
	if !strings.Contains(html, `id="reloadBtn"`) {
		t.Error("サーバーモードなのに更新ボタンが無い")
	}
	if !strings.Contains(html, `http-equiv="refresh" content="60"`) {
		t.Error("自動更新メタ(60s)が無い")
	}

	// ファイルモード: 更新ボタンも自動更新メタも無い
	var fileBuf bytes.Buffer
	if err := RenderReport(&fileBuf, rows, 0.5, false, 0); err != nil {
		t.Fatalf("RenderReport(file) 失敗: %v", err)
	}
	if strings.Contains(fileBuf.String(), `id="reloadBtn"`) {
		t.Error("ファイルモードに更新ボタンが出ている")
	}
	if strings.Contains(fileBuf.String(), "http-equiv=\"refresh\"") {
		t.Error("ファイルモードに自動更新メタが出ている")
	}
}

// TestSortDataValues は各数値列のソート用 data-val が表示値に対応していることを確認する
// （メイン+サブ・完全クリアが 0 固定で並ばなかった不具合の回帰テスト）。
func TestSortDataValues(t *testing.T) {
	rows := []Row{
		{
			Steam: SteamGame{AppID: 1, Name: "Test Game", PlaytimeForever: 120}, // 2.0h
			Match: &HLTBGame{GameID: 1, GameName: "Test Game",
				CompMain: 36000, CompPlus: 72000, Comp100: 108000}, // 10 / 20 / 30 h
			Score: 1.0, Matched: true,
		},
	}
	var buf bytes.Buffer
	if err := RenderReport(&buf, rows, 0.5, false, 0); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	for _, want := range []string{
		`data-val="2">2.0`,   // プレイ時間
		`data-val="10">10.0`, // メイン
		`data-val="20">20.0`, // メイン+サブ
		`data-val="30">30.0`, // 完全クリア
	} {
		if !strings.Contains(html, want) {
			t.Errorf("data-val が表示値と対応していない: %q が見つからない", want)
		}
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	content := "# コメント行\nSTEAM_API_KEY=\"abc123\"\nexport STEAM_ID = 7656119000000000\nEMPTY=\n"
	if err := os.WriteFile(envPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	// 既存の環境変数は上書きしないことを確認
	os.Setenv("STEAM_ID", "preset-wins")
	defer os.Unsetenv("STEAM_ID")
	os.Unsetenv("STEAM_API_KEY")
	defer os.Unsetenv("STEAM_API_KEY")

	loadDotEnv(envPath)

	if got := os.Getenv("STEAM_API_KEY"); got != "abc123" {
		t.Errorf("STEAM_API_KEY = %q, want abc123（引用符除去）", got)
	}
	if got := os.Getenv("STEAM_ID"); got != "preset-wins" {
		t.Errorf("STEAM_ID = %q, want preset-wins（既存env優先）", got)
	}
}
