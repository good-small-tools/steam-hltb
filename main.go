package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type config struct {
	apiKey      string
	steamID     string
	outPath     string
	cachePath   string
	noCache     bool
	concurrency int
	minSim      float64
	delay       time.Duration
	limit       int
	serve       bool
	addr        string
	refresh     time.Duration
	open        bool
}

// matchVersion は照合ロジックのバージョン。括弧・ハイフン以降を削るフォールバック
// や商標記号の除去など、マッチング手法を改善したらこの値を上げる。loadCache は
// これより古い「未一致」キャッシュを破棄して再照合させる（一致済みは保持する）。
const matchVersion = 1

// cacheEntry は1ゲームの照合結果（キーは正規化済みクエリ名）。
type cacheEntry struct {
	Found    bool      `json:"found"`
	Score    float64   `json:"score"`
	Match    *HLTBGame `json:"match,omitempty"`
	MatchVer int       `json:"match_ver,omitempty"` // 照合に使ったロジックのバージョン
}

func main() {
	cfg := parseFlags()
	client := &http.Client{Timeout: 30 * time.Second}

	if cfg.serve {
		runServe(cfg, client)
		return
	}
	runOnce(cfg, client)
}

// runOnce は一回だけ取得・照合して HTML ファイルを書き出す。
func runOnce(cfg config, client *http.Client) {
	cache := loadCache(cfg)
	defer saveCache(cfg, cache)
	hltb := NewHLTBClient(client)
	limiter := newRateLimiter(cfg.delay)

	rows, err := buildRows(cfg, client, hltb, cache, limiter)
	if err != nil {
		fatal(err)
	}
	if err := BuildReport(rows, cfg.outPath, cfg.minSim); err != nil {
		fatal(err)
	}

	matched := 0
	for _, r := range rows {
		if r.Matched {
			matched++
		}
	}
	abs, _ := os.Getwd()
	fullPath := filepath.Join(abs, cfg.outPath)
	fmt.Fprintf(os.Stderr, "\n✔ 完了: %d/%d 本を HLTB と照合しました\n", matched, len(rows))
	fmt.Fprintf(os.Stderr, "  レポート: %s\n", fullPath)
	if cfg.open {
		fmt.Fprintf(os.Stderr, "  ブラウザで開いています…\n")
		tryOpen(fullPath)
	} else {
		fmt.Fprintf(os.Stderr, "  ブラウザで開く: %s\n", fullPath)
	}
}

// runServe はローカル HTTP サーバーを起動する。
// ページにアクセス／リロードするたびに Steam を再取得し（HLTB はキャッシュ利用）、最新の表を返す。
func runServe(cfg config, client *http.Client) {
	cache := loadCache(cfg)
	hltb := NewHLTBClient(client)
	limiter := newRateLimiter(cfg.delay)

	var mu sync.Mutex // 同時リロードを直列化（キャッシュ競合・二重取得の回避）

	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		defer mu.Unlock()

		fmt.Fprintf(os.Stderr, "▶ リクエスト: Steam 再取得中…\n")
		rows, err := buildRows(cfg, client, hltb, cache, limiter)
		if err != nil {
			http.Error(w, "取得に失敗しました: "+err.Error(), http.StatusBadGateway)
			fmt.Fprintf(os.Stderr, "  エラー: %v\n", err)
			return
		}
		saveCache(cfg, cache)

		refreshSec := int(cfg.refresh.Seconds())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := RenderReport(w, rows, cfg.minSim, true, refreshSec); err != nil {
			fmt.Fprintf(os.Stderr, "  描画エラー: %v\n", err)
		}
		fmt.Fprintf(os.Stderr, "  完了（%d 本）\n", len(rows))
	}

	http.HandleFunc("/", handler)
	url := "http://" + httpDisplayAddr(cfg.addr) + "/"
	fmt.Fprintf(os.Stderr, "✔ サーバー起動: %s\n", url)
	if cfg.open {
		fmt.Fprintf(os.Stderr, "  ブラウザで開いています…\n")
		// ListenAndServe は即座に待受を開始するが、確実に応答できるよう
		// 少しだけ待ってから別 goroutine で開く（本処理はブロックする）。
		go func() {
			time.Sleep(500 * time.Millisecond)
			tryOpen(url)
		}()
	} else {
		fmt.Fprintf(os.Stderr, "  ブラウザで開く: %s\n", url)
	}
	if cfg.refresh > 0 {
		fmt.Fprintf(os.Stderr, "  自動更新: %s ごと\n", cfg.refresh)
	}
	fmt.Fprintln(os.Stderr, "  停止: Ctrl+C")
	if err := http.ListenAndServe(cfg.addr, nil); err != nil {
		fatal(err)
	}
}

// buildRows は Steam 取得 → HLTB 照合 までをまとめて行う。
func buildRows(cfg config, client *http.Client, hltb *HLTBClient, cache map[string]cacheEntry, limiter *rateLimiter) ([]Row, error) {
	games, err := FetchOwnedGames(client, cfg.apiKey, cfg.steamID)
	if err != nil {
		return nil, err
	}
	if cfg.limit > 0 && len(games) > cfg.limit {
		games = games[:cfg.limit]
	}
	fmt.Fprintf(os.Stderr, "  %d 本のゲームを取得しました\n", len(games))
	return process(cfg, games, hltb, cache, limiter), nil
}

// httpDisplayAddr は表示用にアドレスを整える（":8765" → "localhost:8765"）。
func httpDisplayAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
}

func parseFlags() config {
	// フラグの既定値を読む前に .env を環境変数へ取り込む（既存の環境変数は上書きしない）。
	loadDotEnv(".env")

	var cfg config
	flag.StringVar(&cfg.apiKey, "key", os.Getenv("STEAM_API_KEY"), "Steam Web API キー（環境変数 STEAM_API_KEY / .env でも可）")
	flag.StringVar(&cfg.steamID, "steamid", os.Getenv("STEAM_ID"), "SteamID64（17桁。環境変数 STEAM_ID / .env でも可）")
	flag.StringVar(&cfg.outPath, "out", "report.html", "出力する HTML ファイル")
	flag.StringVar(&cfg.cachePath, "cache", "hltb_cache.json", "HLTB 照合結果のキャッシュファイル")
	flag.BoolVar(&cfg.noCache, "no-cache", false, "キャッシュを使わず毎回 HLTB に問い合わせる")
	flag.IntVar(&cfg.concurrency, "concurrency", 4, "HLTB への同時リクエスト数")
	flag.Float64Var(&cfg.minSim, "min-similarity", 0.5, "一致とみなす類似度のしきい値（0..1）")
	flag.DurationVar(&cfg.delay, "delay", 300*time.Millisecond, "HLTB リクエスト間の最小間隔（負荷軽減用）")
	flag.IntVar(&cfg.limit, "limit", 0, "処理するゲーム数の上限（0=全件。動作確認用）")
	flag.BoolVar(&cfg.serve, "serve", false, "ローカルサーバーを起動し、リロードで最新化する")
	flag.StringVar(&cfg.addr, "addr", "localhost:8765", "サーバーモードの待受アドレス")
	flag.DurationVar(&cfg.refresh, "refresh", 0, "サーバーモードの自動更新間隔（例 60s。0=自動更新なし）")
	flag.BoolVar(&cfg.open, "open", true, "完了後にレポート（またはサーバー URL）をブラウザで自動的に開く（-open=false で無効化）")
	flag.Parse()

	if cfg.apiKey == "" || cfg.steamID == "" {
		fmt.Fprintln(os.Stderr, "エラー: API キーと SteamID が必要です。")
		fmt.Fprintln(os.Stderr, "  -key / -steamid で渡すか、環境変数 STEAM_API_KEY / STEAM_ID、または .env ファイルに記載してください。")
		fmt.Fprintln(os.Stderr, "\n使い方:")
		flag.PrintDefaults()
		os.Exit(2)
	}
	return cfg
}

// loadDotEnv は KEY=VALUE 形式の .env を読み、未設定の環境変数だけを設定する。
// 行頭 # はコメント、値の前後の引用符は除去する。ファイルが無ければ何もしない。
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		val = strings.Trim(val, `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

// matchGame は1ゲームを HLTB と照合する。まず元のタイトルで検索し、しきい値
// 以上の一致が得られなければ queryVariants が返すフォールバック（"(" 以降や
// " - " 以降を削った短いタイトル）で順に再検索する。全候補のうち最良の一致を返す。
// 1 件でも検索に成功すれば「見つからなかった」も結果として返し（呼び出し側が
// キャッシュできる）、全候補が通信エラーになった場合のみエラーを返す。
func matchGame(hltb *HLTBClient, limiter *rateLimiter, name string, minSim float64) (cacheEntry, error) {
	var best cacheEntry
	var firstErr error
	anySuccess := false

	for _, v := range queryVariants(name) {
		limiter.wait()
		candidates, err := hltb.Search(searchQuery(v))
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		anySuccess = true
		if m, score := BestMatch(v, candidates); m != nil && score > best.Score {
			best = cacheEntry{Found: true, Score: score, Match: m}
		}
		// しきい値以上の一致が得られたら、後続のフォールバックは試さない。
		if best.Found && best.Score >= minSim {
			break
		}
	}

	if !anySuccess && firstErr != nil {
		return cacheEntry{}, firstErr
	}
	best.MatchVer = matchVersion
	return best, nil
}

// process はワーカープールで全ゲームを HLTB と照合する。
func process(cfg config, games []SteamGame, hltb *HLTBClient, cache map[string]cacheEntry, limiter *rateLimiter) []Row {
	rows := make([]Row, len(games))
	var cacheMu sync.Mutex
	var done int64

	jobs := make(chan int)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for i := range jobs {
			g := games[i]
			row := Row{Steam: g}
			key := searchQuery(g.Name)

			// キャッシュ参照
			var entry cacheEntry
			hit := false
			if !cfg.noCache && key != "" {
				cacheMu.Lock()
				entry, hit = cache[key]
				cacheMu.Unlock()
			}

			if !hit {
				e, err := matchGame(hltb, limiter, g.Name, cfg.minSim)
				if err != nil {
					fmt.Fprintf(os.Stderr, "\n  ⚠ %s の検索に失敗: %v\n", g.Name, err)
				} else {
					entry = e
					if key != "" {
						cacheMu.Lock()
						cache[key] = entry
						cacheMu.Unlock()
					}
				}
			}

			if entry.Found && entry.Match != nil {
				row.Match = entry.Match
				row.Score = entry.Score
				row.Matched = entry.Score >= cfg.minSim
			}
			rows[i] = row

			n := atomic.AddInt64(&done, 1)
			fmt.Fprintf(os.Stderr, "\r  照合中… %d/%d", n, len(games))
		}
	}

	wg.Add(cfg.concurrency)
	for w := 0; w < cfg.concurrency; w++ {
		go worker()
	}
	for i := range games {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return rows
}

// rateLimiter は呼び出し間隔を最小 interval に保つ簡易リミッタ。
type rateLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func newRateLimiter(interval time.Duration) *rateLimiter {
	return &rateLimiter{interval: interval}
}

func (r *rateLimiter) wait() {
	if r.interval <= 0 {
		return
	}
	r.mu.Lock()
	now := time.Now()
	if now.Before(r.next) {
		wait := r.next.Sub(now)
		r.next = r.next.Add(r.interval)
		r.mu.Unlock()
		time.Sleep(wait)
		return
	}
	r.next = now.Add(r.interval)
	r.mu.Unlock()
}

func loadCache(cfg config) map[string]cacheEntry {
	m := map[string]cacheEntry{}
	if cfg.noCache {
		return m
	}
	data, err := os.ReadFile(cfg.cachePath)
	if err != nil {
		return m
	}
	_ = json.Unmarshal(data, &m)
	// 照合ロジックが更新されている場合、古いバージョンで付いた「未一致」は
	// 取りこぼしの可能性があるため破棄し、次回再照合させる。一致済みは保持する。
	for k, e := range m {
		if !e.Found && e.MatchVer < matchVersion {
			delete(m, k)
		}
	}
	return m
}

func saveCache(cfg config, m map[string]cacheEntry) {
	if cfg.noCache {
		return
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(cfg.cachePath, data, 0644)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "エラー:", err)
	os.Exit(1)
}
