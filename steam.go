package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// SteamGame は Steam の所有ゲーム1件。
type SteamGame struct {
	AppID           int    `json:"appid"`
	Name            string `json:"name"`
	PlaytimeForever int    `json:"playtime_forever"` // 累計プレイ時間（分）
	ImgIconURL      string `json:"img_icon_url"`
	RTimeLastPlayed int64  `json:"rtime_last_played"` // 最終プレイ時刻（Unix秒, 0=未プレイ）
}

// PlaytimeHours はプレイ時間を時間単位で返す。
func (g SteamGame) PlaytimeHours() float64 {
	return float64(g.PlaytimeForever) / 60.0
}

// CapsuleURL はテーブル表示用の小さなカプセル画像 URL。
func (g SteamGame) CapsuleURL() string {
	return fmt.Sprintf("https://cdn.cloudflare.steamstatic.com/steam/apps/%d/capsule_184x69.jpg", g.AppID)
}

// StoreURL は Steam ストアページ URL。
func (g SteamGame) StoreURL() string {
	return fmt.Sprintf("https://store.steampowered.com/app/%d/", g.AppID)
}

type ownedGamesResponse struct {
	Response struct {
		GameCount int         `json:"game_count"`
		Games     []SteamGame `json:"games"`
	} `json:"response"`
}

// FetchOwnedGames は Steam Web API から所有ゲーム一覧を取得する。
// プロフィールが非公開でも、APIキー所有者本人の steamID なら取得できる。
func FetchOwnedGames(client *http.Client, apiKey, steamID string) ([]SteamGame, error) {
	q := url.Values{}
	q.Set("key", apiKey)
	q.Set("steamid", steamID)
	q.Set("include_appinfo", "1")           // name / img を含める
	q.Set("include_played_free_games", "1") // 無料ゲームも含める
	q.Set("format", "json")

	endpoint := "https://api.steampowered.com/IPlayerService/GetOwnedGames/v1/?" + q.Encode()

	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("Steam API へのリクエストに失敗: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("Steam API が 403 を返しました。APIキーが正しいか確認してください")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Steam API が想定外のステータス %d を返しました: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var parsed ownedGamesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("Steam API のレスポンス解析に失敗: %w", err)
	}

	games := parsed.Response.Games
	if len(games) == 0 {
		return nil, fmt.Errorf("所有ゲームが0件でした。steamID が正しいか、プロフィールのゲーム詳細が「公開」になっているか確認してください（本人のキーなら非公開でも可）")
	}
	return games, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
