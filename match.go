package main

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// ゲーム名の表記ゆれを吸収するための正規化・あいまい一致ロジック。
// Steam 側の名称（"The Witcher 3: Wild Hunt™" など）と HLTB 側の名称を
// 突き合わせるために使う。

// よくあるエディション表記。検索クエリと比較の両方で除去する。
var editionWords = []string{
	"game of the year edition", "game of the year", "goty edition", "goty",
	"definitive edition", "complete edition", "ultimate edition", "deluxe edition",
	"gold edition", "enhanced edition", "special edition", "standard edition",
	"anniversary edition", "legendary edition", "remastered", "remaster",
	"directors cut", "director s cut", "the final cut", "redux",
	"digital deluxe", "deluxe", "collection", "bundle",
}

var (
	// 商標記号や全角記号などを空白に置き換える
	nonAlnum = regexp.MustCompile(`[^a-z0-9 ]+`)
	multiWS  = regexp.MustCompile(`\s+`)

	// 商標・著作権・登録商標記号は明示的に除去する（® ™ © ℠）。
	// nonAlnum でも落ちるが、フォールバックの意図を明確にし、また記号が
	// 直前の語に密着している場合（"Civilization®VI"）でも語を割らずに
	// 消すため、空白ではなく "" に置換して個別に処理する。
	trademarkSyms = strings.NewReplacer("®", "", "™", "", "©", "", "℠", "")
)

// normalize は比較・キャッシュキー用にゲーム名を正規化する。
// 小文字化 → 商標記号除去 → アクセント除去 → 記号除去 → エディション語除去 → 空白圧縮。
func normalize(s string) string {
	s = strings.ToLower(s)
	s = trademarkSyms.Replace(s)
	s = stripAccents(s)
	// 記号類を空白へ
	s = nonAlnum.ReplaceAllString(s, " ")
	s = multiWS.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	// エディション語を除去（前後に空白を付けて部分一致の誤爆を防ぐ）
	padded := " " + s + " "
	for _, w := range editionWords {
		padded = strings.ReplaceAll(padded, " "+w+" ", " ")
	}
	s = multiWS.ReplaceAllString(padded, " ")
	return strings.TrimSpace(s)
}

// stripAccents は é→e のようにダイアクリティカルマークを落とす（簡易版）。
func stripAccents(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Mn, r):
			// 結合用記号はスキップ
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// searchQuery は HLTB 検索に投げる文字列を作る。
// 記号・エディション語を落とすのみ。ローマ数字は変換しない
// （HLTB はタイトルをローマ数字で索引しており、"V"→"5" にすると検索が外れるため）。
// ローマ数字↔アラビア数字の吸収は similarity() 側（比較時）に任せる。
func searchQuery(name string) string {
	return normalize(name)
}

// queryVariants は HLTB 検索に使うタイトル候補を、フォールバックの優先順で返す。
// 先頭は常に元の名前。元の名前で十分に一致しなかった場合に備え、続けて
//
//  1. "(" 以降を削除したタイトル（例: "Resident Evil 4 (2023)" → "Resident Evil 4"）
//  2. " - " 以降を削除したタイトル（例: "DOOM - Deluxe Edition" → "DOOM"）
//
// を返す。ハイフンは前後に空白のある " - "（サブタイトル区切り）だけを対象にし、
// "Half-Life" のような語中のハイフンは分割しない。正規化後に空・重複になる
// 候補は除外するので、元の名前と実質同じ候補で無駄に再検索することはない。
func queryVariants(name string) []string {
	variants := []string{name}
	seen := map[string]bool{normalize(name): true}
	add := func(s string) {
		s = strings.TrimSpace(s)
		n := normalize(s)
		if n == "" || seen[n] {
			return
		}
		seen[n] = true
		variants = append(variants, s)
	}
	if i := strings.IndexByte(name, '('); i > 0 {
		add(name[:i])
	}
	if i := strings.Index(name, " - "); i > 0 {
		add(name[:i])
	}
	return variants
}

// tokens は空白区切りの語に分割する。
func tokens(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}

// levenshtein は編集距離を返す。
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

// ratio は 0..1 の類似度（1.0 が完全一致）。編集距離ベース。
func ratio(a, b string) float64 {
	if a == "" && b == "" {
		return 1.0
	}
	maxLen := len([]rune(a))
	if l := len([]rune(b)); l > maxLen {
		maxLen = l
	}
	if maxLen == 0 {
		return 1.0
	}
	d := levenshtein(a, b)
	return 1.0 - float64(d)/float64(maxLen)
}

// tokenSetRatio は fuzzywuzzy の token_set_ratio 相当。
// "the witcher 3" と "the witcher 3 wild hunt" のような部分包含に強い。
func tokenSetRatio(a, b string) float64 {
	ta := uniqueSortedTokens(a)
	tb := uniqueSortedTokens(b)
	setA := toSet(ta)
	setB := toSet(tb)

	var inter, diffA, diffB []string
	for _, t := range ta {
		if setB[t] {
			inter = append(inter, t)
		} else {
			diffA = append(diffA, t)
		}
	}
	for _, t := range tb {
		if !setA[t] {
			diffB = append(diffB, t)
		}
	}
	sort.Strings(inter)
	sort.Strings(diffA)
	sort.Strings(diffB)

	interStr := strings.Join(inter, " ")
	combAB := strings.TrimSpace(interStr + " " + strings.Join(diffA, " "))
	combBA := strings.TrimSpace(interStr + " " + strings.Join(diffB, " "))

	r1 := ratio(interStr, combAB)
	r2 := ratio(interStr, combBA)
	r3 := ratio(combAB, combBA)
	return maxF(r1, maxF(r2, r3))
}

func uniqueSortedTokens(s string) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tokens(s) {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	sort.Strings(out)
	return out
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// similarity は 2 つのゲーム名の総合類似度（0..1）。
// 正規化したうえで「全体の編集距離」と「トークン集合一致」の良い方を採る。
func similarity(steamName, hltbName string) float64 {
	a := romanToArabicTokens(normalize(steamName))
	b := romanToArabicTokens(normalize(hltbName))
	return maxF(ratio(a, b), tokenSetRatio(a, b))
}

// romanToArabicTokens は単語単位でローマ数字をアラビア数字に寄せる。
// "final fantasy vii" → "final fantasy 7"。誤爆を避けるため完全一致のトークンのみ変換。
var romanMap = map[string]string{
	"i": "1", "ii": "2", "iii": "3", "iv": "4", "v": "5",
	"vi": "6", "vii": "7", "viii": "8", "ix": "9", "x": "10",
	"xi": "11", "xii": "12", "xiii": "13", "xiv": "14", "xv": "15",
}

func romanToArabicTokens(s string) string {
	parts := tokens(s)
	for i, p := range parts {
		if v, ok := romanMap[p]; ok {
			parts[i] = v
		}
	}
	return strings.Join(parts, " ")
}

// BestMatch は候補群から Steam 名に最も近いものを選ぶ。
// game_name と game_alias の両方を比較し、高い方の類似度を採用する。
// 同スコアのとき（"Portal" が "Portal 2" を拾う等）は、編集距離ベースの
// 素の一致率が高い＝より完全一致に近い候補を優先する。
func BestMatch(steamName string, candidates []HLTBGame) (best *HLTBGame, score float64) {
	a := romanToArabicTokens(normalize(steamName))
	var bestRatio float64
	for i := range candidates {
		c := &candidates[i]
		s, r := pairScore(a, c.GameName)
		if c.GameAlias != "" {
			if sa, ra := pairScore(a, c.GameAlias); sa > s || (sa == s && ra > r) {
				s, r = sa, ra
			}
		}
		const eps = 1e-9
		if best == nil || s > score+eps || (s > score-eps && r > bestRatio) {
			best, score, bestRatio = c, s, r
		}
	}
	return best, score
}

// pairScore は正規化済みの Steam 名 a と HLTB 候補名 b を比較し、
// 総合スコア（編集距離 or トークン集合の良い方）と素の編集距離一致率を返す。
func pairScore(a, candidateName string) (combined, plainRatio float64) {
	b := romanToArabicTokens(normalize(candidateName))
	plainRatio = ratio(a, b)
	combined = maxF(plainRatio, tokenSetRatio(a, b))
	return combined, plainRatio
}
