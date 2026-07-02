package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// HowLongToBeat は公式 API を公開していないため、フロントエンドが使う
// /api/bleed エンドポイントを利用する。フローは以下のとおり:
//
//	1. GET  /api/bleed/init?t=<ms>  → {token, hpKey, hpVal}（アンチボット用トークン）
//	2. POST /api/bleed              → ヘッダに x-auth-token / x-hp-key / x-hp-val、
//	                                   本文に検索ペイロード（+ payload[hpKey]=hpVal）を付与
//
// トークンは使い回せるが期限切れで 403 になるため、その場合は再取得して 1 度だけ再試行する。

const (
	hltbBase   = "https://howlongtobeat.com"
	hltbUA     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	hltbPageSz = 20
)

// HLTBGame は検索結果1件。時間系フィールドはすべて「秒」。
type HLTBGame struct {
	GameID          int    `json:"game_id"`
	GameName        string `json:"game_name"`
	GameAlias       string `json:"game_alias"`
	GameImage       string `json:"game_image"`
	GameType        string `json:"game_type"`
	CompMain        int    `json:"comp_main"` // メインストーリー
	CompPlus        int    `json:"comp_plus"` // メイン＋サブ
	Comp100         int    `json:"comp_100"`  // 完全クリア
	CompAll         int    `json:"comp_all"`  // 全スタイル平均
	ReviewScore     int    `json:"review_score"`
	ReleaseWorld    int    `json:"release_world"`
	ProfilePlatform string `json:"profile_platform"`
}

// HoursMain などは秒→時間に変換するヘルパー。
func (g HLTBGame) HoursMain() float64 { return float64(g.CompMain) / 3600.0 }
func (g HLTBGame) HoursPlus() float64 { return float64(g.CompPlus) / 3600.0 }
func (g HLTBGame) Hours100() float64  { return float64(g.Comp100) / 3600.0 }
func (g HLTBGame) HoursAll() float64  { return float64(g.CompAll) / 3600.0 }

// URL は HLTB のゲームページ。
func (g HLTBGame) URL() string {
	return fmt.Sprintf("%s/game/%d", hltbBase, g.GameID)
}

// ImageURL はボックスアート画像 URL。
func (g HLTBGame) ImageURL() string {
	if g.GameImage == "" {
		return ""
	}
	return fmt.Sprintf("%s/games/%s", hltbBase, g.GameImage)
}

type searchResponse struct {
	Count int        `json:"count"`
	Data  []HLTBGame `json:"data"`
}

type bleedInit struct {
	Token string `json:"token"`
	HpKey string `json:"hpKey"`
	HpVal string `json:"hpVal"`
}

// HLTBClient は HLTB 検索クライアント。トークンを保持し、スレッドセーフ。
type HLTBClient struct {
	http *http.Client
	mu   sync.Mutex
	tok  bleedInit
}

func NewHLTBClient(client *http.Client) *HLTBClient {
	return &HLTBClient{http: client}
}

// ensureToken は未取得ならトークンを取得する。
func (c *HLTBClient) ensureToken() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tok.Token != "" {
		return nil
	}
	return c.refreshLocked()
}

// refresh はトークンを強制再取得する（403 時など）。
func (c *HLTBClient) refresh() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.refreshLocked()
}

func (c *HLTBClient) refreshLocked() error {
	url := fmt.Sprintf("%s/api/bleed/init?t=%d", hltbBase, time.Now().UnixMilli())
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	c.setCommonHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("HLTB トークン取得に失敗: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HLTB トークン取得が %d を返しました", resp.StatusCode)
	}
	var init bleedInit
	if err := json.Unmarshal(body, &init); err != nil {
		return fmt.Errorf("HLTB トークン解析に失敗: %w", err)
	}
	if init.Token == "" {
		return fmt.Errorf("HLTB トークンが空でした（サイト仕様変更の可能性）")
	}
	c.tok = init
	return nil
}

func (c *HLTBClient) currentToken() bleedInit {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tok
}

func (c *HLTBClient) setCommonHeaders(req *http.Request) {
	req.Header.Set("User-Agent", hltbUA)
	req.Header.Set("Referer", hltbBase+"/")
	req.Header.Set("Origin", hltbBase)
	req.Header.Set("Accept", "*/*")
}

// buildPayload は /api/bleed に投げる検索ペイロードを組み立てる。
func buildPayload(term string, hpKey, hpVal string) map[string]any {
	p := map[string]any{
		"searchType":  "games",
		"searchTerms": strings.Fields(term),
		"searchPage":  1,
		"size":        hltbPageSz,
		"searchOptions": map[string]any{
			"games": map[string]any{
				"userId":        0,
				"platform":      "",
				"sortCategory":  "popular",
				"rangeCategory": "main",
				"rangeTime":     map[string]any{"min": nil, "max": nil},
				"gameplay": map[string]any{
					"perspective": "", "flow": "", "genre": "", "difficulty": "",
				},
				"rangeYear": map[string]any{"min": "", "max": ""},
				"modifier":  "",
			},
			"users":      map[string]any{"sortCategory": "postcount"},
			"lists":      map[string]any{"sortCategory": "follows"},
			"filter":     "",
			"sort":       0,
			"randomizer": 0,
		},
		"useCache": true,
	}
	// アンチボット値を本文にも注入する（フロントエンドと同じ挙動）
	if hpKey != "" {
		p[hpKey] = hpVal
	}
	return p
}

// Search はゲーム名で検索し、候補一覧を返す。403 のときは 1 度だけトークン再取得して再試行。
func (c *HLTBClient) Search(term string) ([]HLTBGame, error) {
	if err := c.ensureToken(); err != nil {
		return nil, err
	}
	res, status, err := c.doSearch(term)
	if err == nil && status == http.StatusForbidden {
		// トークン期限切れ → 再取得して 1 回だけリトライ
		if rerr := c.refresh(); rerr != nil {
			return nil, rerr
		}
		res, status, err = c.doSearch(term)
	}
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("HLTB 検索が %d を返しました", status)
	}
	return res, nil
}

func (c *HLTBClient) doSearch(term string) ([]HLTBGame, int, error) {
	tok := c.currentToken()
	payload := buildPayload(term, tok.HpKey, tok.HpVal)
	buf, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}

	req, _ := http.NewRequest(http.MethodPost, hltbBase+"/api/bleed", bytes.NewReader(buf))
	c.setCommonHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-auth-token", tok.Token)
	req.Header.Set("x-hp-key", tok.HpKey)
	req.Header.Set("x-hp-val", tok.HpVal)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("HLTB 検索リクエストに失敗: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}
	var sr searchResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("HLTB 検索レスポンス解析に失敗: %w", err)
	}
	return sr.Data, resp.StatusCode, nil
}
