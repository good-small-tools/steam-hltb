package main

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"sort"
	"time"
)

// Row はレポート1行分（Steam ゲーム + HLTB 照合結果）。
type Row struct {
	Steam   SteamGame
	Match   *HLTBGame // 照合できなければ nil
	Score   float64   // 類似度 0..1
	Matched bool      // しきい値を超えたか
}

// RowView はテンプレート描画用に整形済みの1行。
type RowView struct {
	Name        string
	StoreURL    string
	CapsuleURL  string
	PlayedHours float64
	PlayedStr   string
	LastPlayed  string

	HasMatch  bool
	HLTBName  string
	HLTBURL   string
	HLTBImage string
	MainH     float64 // ソート用の数値（時間）
	PlusH     float64
	HundH     float64
	MainStr   string
	PlusStr   string
	HundStr   string
	AllStr    string
	Score     int // %
	ScoreStr  string
	Status    string // "matched" | "low" | "none"
	StatusLbl string

	ProgressPct int // Steam/Main の進捗 %
	ProgressStr string
	ReleaseYear string
	ReviewScore string
}

type summary struct {
	TotalGames   int
	MatchedGames int
	Unplayed     int
	TotalPlayedH string
	TotalMainH   string
	BacklogH     string
	GeneratedAt  string
	MatchedPct   int
}

type reportData struct {
	Summary     summary
	Rows        []RowView
	Serve       bool // サーバーモードか（更新ボタンを表示）
	AutoRefresh int  // 自動更新の秒数（0=なし）
}

func hoursStr(h float64) string {
	if h <= 0 {
		return "–"
	}
	return fmt.Sprintf("%.1f", h)
}

// BuildReport は行データから HTML を生成してファイルに書き出す（一回実行モード用）。
func BuildReport(rows []Row, outPath string, threshold float64) error {
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("出力ファイル作成に失敗: %w", err)
	}
	defer f.Close()
	return RenderReport(f, rows, threshold, false, 0)
}

// RenderReport は行データから HTML を生成して w に書き出す。
// serve=true でサーバーモード用の「更新」ボタンを表示し、refreshSec>0 で自動更新する。
func RenderReport(w io.Writer, rows []Row, threshold float64, serve bool, refreshSec int) error {
	// 並び順: Steam プレイ時間の降順（よく遊んだ順）。同値は名前順。
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Steam.PlaytimeForever != rows[j].Steam.PlaytimeForever {
			return rows[i].Steam.PlaytimeForever > rows[j].Steam.PlaytimeForever
		}
		return rows[i].Steam.Name < rows[j].Steam.Name
	})

	var (
		views                        []RowView
		totalPlayedMin, matchedCount int
		unplayed                     int
		totalMainSec, backlogSec     float64
	)

	for _, r := range rows {
		totalPlayedMin += r.Steam.PlaytimeForever
		if r.Steam.PlaytimeForever == 0 {
			unplayed++
		}

		v := RowView{
			Name:        r.Steam.Name,
			StoreURL:    r.Steam.StoreURL(),
			CapsuleURL:  r.Steam.CapsuleURL(),
			PlayedHours: r.Steam.PlaytimeHours(),
			PlayedStr:   hoursStr(r.Steam.PlaytimeHours()),
			LastPlayed:  lastPlayedStr(r.Steam.RTimeLastPlayed),
		}

		if r.Match != nil {
			m := r.Match
			v.HasMatch = true
			v.HLTBName = m.GameName
			v.HLTBURL = m.URL()
			v.HLTBImage = m.ImageURL()
			v.MainH = m.HoursMain()
			v.PlusH = m.HoursPlus()
			v.HundH = m.Hours100()
			v.MainStr = hoursStr(m.HoursMain())
			v.PlusStr = hoursStr(m.HoursPlus())
			v.HundStr = hoursStr(m.Hours100())
			v.AllStr = hoursStr(m.HoursAll())
			v.Score = int(r.Score*100 + 0.5)
			v.ScoreStr = fmt.Sprintf("%d%%", v.Score)
			if m.ReleaseWorld > 0 {
				v.ReleaseYear = fmt.Sprintf("%d", m.ReleaseWorld)
			}
			if m.ReviewScore > 0 {
				v.ReviewScore = fmt.Sprintf("%d", m.ReviewScore)
			}

			if r.Matched {
				matchedCount++
				v.Status = "matched"
				v.StatusLbl = "一致"
				if m.CompMain > 0 {
					totalMainSec += float64(m.CompMain)
					// バックログ: メインクリアまでの残り時間
					rem := float64(m.CompMain) - float64(r.Steam.PlaytimeForever)*60
					if rem > 0 {
						backlogSec += rem
					}
				}
				// 進捗
				if m.CompMain > 0 {
					pct := int(float64(r.Steam.PlaytimeForever)*60/float64(m.CompMain)*100 + 0.5)
					v.ProgressPct = pct
					v.ProgressStr = fmt.Sprintf("%d%%", pct)
				}
			} else {
				v.Status = "low"
				v.StatusLbl = "要確認"
			}
		} else {
			v.Status = "none"
			v.StatusLbl = "未一致"
			v.ScoreStr = "–"
		}
		views = append(views, v)
	}

	data := reportData{
		Summary: summary{
			TotalGames:   len(rows),
			MatchedGames: matchedCount,
			Unplayed:     unplayed,
			TotalPlayedH: fmt.Sprintf("%.0f", float64(totalPlayedMin)/60.0),
			TotalMainH:   fmt.Sprintf("%.0f", totalMainSec/3600.0),
			BacklogH:     fmt.Sprintf("%.0f", backlogSec/3600.0),
			GeneratedAt:  time.Now().Format("2006-01-02 15:04:05"),
			MatchedPct:   pctOf(matchedCount, len(rows)),
		},
		Rows:        views,
		Serve:       serve,
		AutoRefresh: refreshSec,
	}

	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("テンプレート解析に失敗: %w", err)
	}
	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("HTML 生成に失敗: %w", err)
	}
	return nil
}

func pctOf(a, b int) int {
	if b == 0 {
		return 0
	}
	return int(float64(a)/float64(b)*100 + 0.5)
}

func lastPlayedStr(t int64) string {
	if t <= 0 {
		return "" // 最終プレイ日時が無ければ「最終:」行ごと省略（テンプレート側で {{if}} ガード）
	}
	return time.Unix(t, 0).Format("2006-01-02")
}
