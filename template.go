package main

// reportTemplate は自己完結型の HTML（インライン CSS/JS）。
// 列ヘッダクリックでソート、上部の入力でフィルタできる。
// UI 文言は data-i18n 属性 + JS 辞書で日本語/英語を切り替える（選択は localStorage に保存）。
const reportTemplate = `<!doctype html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
{{if .AutoRefresh}}<meta http-equiv="refresh" content="{{.AutoRefresh}}">{{end}}
<title>Steam × HowLongToBeat プレイ時間リスト</title>
<style>
  :root{
    --bg:#0f1216; --panel:#171c23; --panel2:#1e252e; --line:#2a323d;
    --txt:#e6edf3; --muted:#8b97a7; --accent:#66c0f4; --green:#5ee0a0;
    --amber:#f5c451; --red:#f08a8a;
  }
  *{box-sizing:border-box}
  body{margin:0;background:var(--bg);color:var(--txt);
    font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","Hiragino Sans","Noto Sans JP",sans-serif;
    font-size:14px;line-height:1.45}
  header{padding:24px 28px 8px}
  .topbar{display:flex;justify-content:space-between;align-items:flex-start;gap:16px;flex-wrap:wrap}
  h1{margin:0 0 4px;font-size:20px;letter-spacing:.02em}
  .sub{color:var(--muted);font-size:12px}
  .langtoggle{display:flex;border:1px solid var(--line);border-radius:8px;overflow:hidden;flex-shrink:0}
  .langtoggle button{background:var(--panel);color:var(--muted);border:0;padding:6px 12px;
    font-size:12px;font-weight:600;cursor:pointer;line-height:1.4}
  .langtoggle button+button{border-left:1px solid var(--line)}
  .langtoggle button.active{background:var(--accent);color:#06121c}
  .langtoggle button:not(.active):hover{color:var(--txt)}
  .cards{display:flex;flex-wrap:wrap;gap:12px;padding:16px 28px 8px}
  .card{background:var(--panel);border:1px solid var(--line);border-radius:10px;
    padding:12px 16px;min-width:130px;flex:1}
  .card .n{font-size:22px;font-weight:600}
  .card .l{color:var(--muted);font-size:11px;margin-top:2px}
  .card .n.green{color:var(--green)} .card .n.amber{color:var(--amber)} .card .n.accent{color:var(--accent)}
  .controls{display:flex;gap:14px;align-items:center;flex-wrap:wrap;padding:12px 28px}
  .controls.timefilter{padding-top:0;padding-bottom:4px}
  .controls input[type=text]{background:var(--panel2);border:1px solid var(--line);color:var(--txt);
    border-radius:8px;padding:8px 12px;width:280px;font-size:13px}
  .controls label{color:var(--muted);font-size:12px;display:flex;align-items:center;gap:6px;cursor:pointer}
  .controls button{background:var(--accent);color:#06121c;border:0;border-radius:8px;
    padding:8px 14px;font-size:13px;font-weight:600;cursor:pointer}
  .controls button:hover{filter:brightness(1.1)}
  .controls button:disabled{opacity:.6;cursor:default}
  .tf-label{color:var(--muted);font-size:12px;font-weight:600}
  .timefilter select,.timefilter input[type=number]{background:var(--panel2);border:1px solid var(--line);
    color:var(--txt);border-radius:8px;padding:7px 10px;font-size:13px}
  .timefilter input[type=number]{width:84px}
  .tf-sep{color:var(--muted)}
  .tf-unit{color:var(--muted);font-size:12px}
  .tf-clear{background:var(--panel2)!important;color:var(--muted)!important;font-weight:500!important;
    padding:6px 12px!important}
  .tf-clear:hover{color:var(--txt)!important}
  .wrap{overflow-x:auto;padding:0 28px 40px}
  table{border-collapse:collapse;width:100%;min-width:920px}
  th,td{padding:8px 10px;text-align:left;border-bottom:1px solid var(--line);vertical-align:middle}
  th{position:sticky;top:0;background:var(--panel);color:var(--muted);font-weight:600;
    font-size:11px;text-transform:uppercase;letter-spacing:.04em;cursor:pointer;user-select:none;white-space:nowrap}
  th.num,td.num{text-align:right;font-variant-numeric:tabular-nums}
  th:hover{color:var(--txt)}
  th .arrow{opacity:.4;font-size:10px}
  tbody tr:hover{background:var(--panel)}
  .cap{width:92px;height:35px;object-fit:cover;border-radius:4px;background:var(--panel2);display:block}
  .gname{font-weight:600}
  .gname a{color:var(--txt);text-decoration:none}
  .gname a:hover{color:var(--accent)}
  .meta{color:var(--muted);font-size:11px;margin-top:2px}
  .meta a{color:var(--muted);text-decoration:none}
  .meta a:hover{color:var(--accent)}
  .badge{display:inline-block;padding:1px 7px;border-radius:999px;font-size:10px;font-weight:600}
  .b-matched{background:rgba(94,224,160,.15);color:var(--green)}
  .b-low{background:rgba(245,196,81,.15);color:var(--amber)}
  .b-none{background:rgba(240,138,138,.13);color:var(--red)}
  .bar{position:relative;height:7px;background:var(--panel2);border-radius:999px;overflow:hidden;min-width:64px}
  .bar>span{position:absolute;left:0;top:0;height:100%;background:var(--accent);border-radius:999px}
  .bar.over>span{background:var(--green)}
  .pcap{color:var(--muted);font-size:11px;margin-top:3px;text-align:right}
  .dim{color:var(--muted)}
  footer{color:var(--muted);font-size:11px;padding:0 28px 28px}
  .hidden{display:none}
</style>
</head>
<body>
<header>
  <div class="topbar">
    <div>
      <h1 data-i18n="title">Steam &times; HowLongToBeat プレイ時間リスト</h1>
      <div class="sub">
        <span data-i18n="sub_generated">データ取得</span>: {{.Summary.GeneratedAt}}
        ／ <span data-i18n="sub_note">時間は HowLongToBeat の平均値（時間単位）</span>{{if .AutoRefresh}}
        <span class="i18n-auto" data-n="{{.AutoRefresh}}"></span>{{end}}
      </div>
    </div>
    <div class="langtoggle">
      <button type="button" data-lang="ja" onclick="setLang('ja')">日本語</button>
      <button type="button" data-lang="en" onclick="setLang('en')">English</button>
    </div>
  </div>
</header>

<div class="cards">
  <div class="card"><div class="n">{{.Summary.TotalGames}}</div><div class="l"><span data-i18n="card_total">所有ゲーム数</span></div></div>
  <div class="card"><div class="n amber">{{.Summary.Unplayed}}</div><div class="l"><span data-i18n="card_unplayed">未プレイ本数</span></div></div>
  <div class="card"><div class="n accent">{{.Summary.TotalPlayedH}}</div><div class="l"><span data-i18n="card_played">合計プレイ時間 (h)</span></div></div>
</div>

<div class="controls">
  {{if .Serve}}<button id="reloadBtn" type="button" data-i18n="reload">🔄 最新に更新</button>{{end}}
  <input type="text" id="filter" data-i18n-ph="filter_ph" placeholder="ゲーム名で絞り込み…">
  <label><input type="checkbox" id="onlyMatched"> <span data-i18n="only_matched">照合成功のみ表示</span></label>
  <label><input type="checkbox" id="onlyUnplayed"> <span data-i18n="only_unplayed">未プレイのみ表示</span></label>
  <span class="dim" id="count"></span>
</div>

<div class="controls timefilter">
  <span class="tf-label" data-i18n="time_label">プレイ時間で絞り込み</span>
  <select id="timeCat">
    <option value="main" data-i18n="cat_main">メイン</option>
    <option value="plus" data-i18n="cat_plus">メイン＋サブ</option>
    <option value="hund" data-i18n="cat_hund">完全クリア</option>
  </select>
  <input type="number" id="timeMin" min="0" step="1" inputmode="decimal" data-i18n-ph="min_ph" placeholder="最小">
  <span class="tf-sep">–</span>
  <input type="number" id="timeMax" min="0" step="1" inputmode="decimal" data-i18n-ph="max_ph" placeholder="最大">
  <span class="tf-unit">h</span>
  <button id="timeClear" type="button" class="tf-clear" data-i18n="time_clear">クリア</button>
</div>

<div class="wrap">
<table id="t">
  <thead>
    <tr>
      <th data-type="text"></th>
      <th data-type="text"><span data-i18n="th_game">ゲーム名</span> <span class="arrow"></span></th>
      <th class="num" data-type="num"><span data-i18n="th_played">プレイ時間</span> <span class="arrow"></span></th>
      <th class="num" data-type="num"><span data-i18n="th_main">メイン</span> <span class="arrow"></span></th>
      <th class="num" data-type="num"><span data-i18n="th_plus">メイン+サブ</span> <span class="arrow"></span></th>
      <th class="num" data-type="num"><span data-i18n="th_hund">完全クリア</span> <span class="arrow"></span></th>
    </tr>
  </thead>
  <tbody>
    {{range .Rows}}
    <tr data-name="{{.Name}}" data-matched="{{if eq .Status "matched"}}1{{else}}0{{end}}" data-unplayed="{{if eq .PlayedHours 0.0}}1{{else}}0{{end}}" data-main="{{.MainH}}" data-plus="{{.PlusH}}" data-hund="{{.HundH}}">
      <td><img class="cap" loading="lazy" src="{{.CapsuleURL}}" alt="" onerror="this.style.visibility='hidden'"></td>
      <td>
        <div class="gname"><a href="{{.StoreURL}}" target="_blank" rel="noopener">{{.Name}}</a></div>
        <div class="meta">
          <span class="badge b-{{.Status}}" data-i18n="status_{{.Status}}">{{.StatusLbl}}</span>
          {{if .HasMatch}}&nbsp;<a href="{{.HLTBURL}}" target="_blank" rel="noopener">HLTB: {{.HLTBName}}</a>{{end}}
          {{if .ReleaseYear}}&nbsp;· {{.ReleaseYear}}{{end}}
          {{if .LastPlayed}}&nbsp;· <span data-i18n="row_last">最終</span>: {{.LastPlayed}}{{end}}
        </div>
      </td>
      <td class="num" data-val="{{.PlayedHours}}">{{.PlayedStr}}</td>
      <td class="num" data-val="{{.MainH}}">{{.MainStr}}</td>
      <td class="num" data-val="{{.PlusH}}">{{.PlusStr}}</td>
      <td class="num" data-val="{{.HundH}}">{{.HundStr}}</td>
    </tr>
    {{end}}
  </tbody>
</table>
</div>

<footer>
  <span data-i18n="footer">時間データ出典: HowLongToBeat（コミュニティ平均）。一致度はゲーム名の類似度で、低い場合は別タイトルを拾っている可能性があります（「要確認」バッジ）。</span>
</footer>

<script>
(function(){
  // ---- 多言語辞書（{n}/{shown}/{total} はプレースホルダ） ----
  var I18N = {
    ja: {
      title: "Steam × HowLongToBeat プレイ時間リスト",
      sub_generated: "データ取得",
      sub_note: "時間は HowLongToBeat の平均値（時間単位）",
      autorefresh: "／ {n} 秒ごとに自動更新",
      card_total: "所有ゲーム数",
      card_unplayed: "未プレイ本数",
      card_played: "合計プレイ時間 (h)",
      reload: "🔄 最新に更新",
      reloading: "更新中…",
      filter_ph: "ゲーム名で絞り込み…",
      only_matched: "照合成功のみ表示",
      only_unplayed: "未プレイのみ表示",
      time_label: "プレイ時間で絞り込み",
      cat_main: "メイン",
      cat_plus: "メイン＋サブ",
      cat_hund: "完全クリア",
      min_ph: "最小",
      max_ph: "最大",
      time_clear: "クリア",
      th_game: "ゲーム名",
      th_played: "プレイ時間",
      th_main: "メイン",
      th_plus: "メイン+サブ",
      th_hund: "完全クリア",
      row_last: "最終",
      status_matched: "一致",
      status_low: "要確認",
      status_none: "未一致",
      count: "{shown} 件表示中 / 全 {total} 件",
      footer: "時間データ出典: HowLongToBeat（コミュニティ平均）。一致度はゲーム名の類似度で、低い場合は別タイトルを拾っている可能性があります（「要確認」バッジ）。"
    },
    en: {
      title: "Steam × HowLongToBeat Playtime List",
      sub_generated: "Fetched",
      sub_note: "Times are HowLongToBeat community averages (hours)",
      autorefresh: "／ auto-refresh every {n}s",
      card_total: "Owned games",
      card_unplayed: "Unplayed",
      card_played: "Total playtime (h)",
      reload: "🔄 Refresh",
      reloading: "Refreshing…",
      filter_ph: "Filter by game name…",
      only_matched: "Matched only",
      only_unplayed: "Unplayed only",
      time_label: "Filter by time",
      cat_main: "Main",
      cat_plus: "Main + Extra",
      cat_hund: "Completionist",
      min_ph: "Min",
      max_ph: "Max",
      time_clear: "Clear",
      th_game: "Game",
      th_played: "Playtime",
      th_main: "Main",
      th_plus: "Main+Extra",
      th_hund: "Completionist",
      row_last: "Last",
      status_matched: "Matched",
      status_low: "Check",
      status_none: "No match",
      count: "{shown} shown / {total} total",
      footer: "Time data from HowLongToBeat (community averages). Match score is title similarity; a low score may mean a different title was picked (\"Check\" badge)."
    }
  };

  var lang = "ja";
  try { var saved = localStorage.getItem("lang"); if(saved === "ja" || saved === "en") lang = saved; } catch(e){}

  function t(key){
    var d = I18N[lang] || I18N.ja;
    return (key in d) ? d[key] : (I18N.ja[key] || key);
  }

  var table = document.getElementById('t');
  var tbody = table.tBodies[0];
  var rows = Array.prototype.slice.call(tbody.rows);

  // 並び替え
  var sortState = {col:-1, dir:1};
  var headers = table.tHead.rows[0].cells;
  for(var i=0;i<headers.length;i++){
    (function(idx){
      headers[idx].addEventListener('click', function(){
        var type = headers[idx].getAttribute('data-type');
        if(!type) return;
        sortState.dir = (sortState.col === idx) ? -sortState.dir : 1;
        sortState.col = idx;
        var sorted = rows.slice().sort(function(a,b){
          var av, bv;
          if(type === 'num'){
            av = parseFloat(a.cells[idx].getAttribute('data-val')) || 0;
            bv = parseFloat(b.cells[idx].getAttribute('data-val')) || 0;
            return (av-bv)*sortState.dir;
          } else {
            av = (a.cells[idx].textContent||'').trim().toLowerCase();
            bv = (b.cells[idx].textContent||'').trim().toLowerCase();
            return av<bv ? -sortState.dir : av>bv ? sortState.dir : 0;
          }
        });
        sorted.forEach(function(r){ tbody.appendChild(r); });
        for(var h=0;h<headers.length;h++){
          var ar = headers[h].querySelector('.arrow');
          if(ar) ar.textContent = (h===idx) ? (sortState.dir>0?'▲':'▼') : '';
        }
      });
    })(i);
  }

  // フィルタ（名前・チェックボックス・プレイ時間レンジ）
  var fInput = document.getElementById('filter');
  var cMatched = document.getElementById('onlyMatched');
  var cUnplayed = document.getElementById('onlyUnplayed');
  var counter = document.getElementById('count');
  var timeCat = document.getElementById('timeCat');
  var timeMin = document.getElementById('timeMin');
  var timeMax = document.getElementById('timeMax');
  var timeClear = document.getElementById('timeClear');

  function applyFilter(){
    var q = fInput.value.trim().toLowerCase();
    var onlyM = cMatched.checked, onlyU = cUnplayed.checked;
    var cat = timeCat.value; // main | plus | hund
    var minV = parseFloat(timeMin.value);
    var maxV = parseFloat(timeMax.value);
    var hasMin = !isNaN(minV), hasMax = !isNaN(maxV);
    var shown = 0;
    rows.forEach(function(r){
      var name = (r.getAttribute('data-name')||'').toLowerCase();
      var ok = name.indexOf(q) !== -1;
      if(ok && onlyM && r.getAttribute('data-matched') !== '1') ok = false;
      if(ok && onlyU && r.getAttribute('data-unplayed') !== '1') ok = false;
      if(ok && (hasMin || hasMax)){
        var tv = parseFloat(r.getAttribute('data-'+cat));
        // HLTB の時間データが無い行（0 以下）はレンジ指定時に除外
        if(isNaN(tv) || tv <= 0){ ok = false; }
        else {
          if(hasMin && tv < minV) ok = false;
          if(hasMax && tv > maxV) ok = false;
        }
      }
      r.classList.toggle('hidden', !ok);
      if(ok) shown++;
    });
    counter.textContent = t('count').replace('{shown}', shown).replace('{total}', rows.length);
  }
  fInput.addEventListener('input', applyFilter);
  cMatched.addEventListener('change', applyFilter);
  cUnplayed.addEventListener('change', applyFilter);
  timeCat.addEventListener('change', applyFilter);
  timeMin.addEventListener('input', applyFilter);
  timeMax.addEventListener('input', applyFilter);
  timeClear.addEventListener('click', function(){
    timeMin.value = ''; timeMax.value = ''; applyFilter();
  });

  // リロードボタン（サーバーモードのみ存在）
  var reloadBtn = document.getElementById('reloadBtn');
  if(reloadBtn){
    reloadBtn.addEventListener('click', function(){
      this.disabled = true; this.textContent = t('reloading'); location.reload();
    });
  }

  // ---- 言語適用 ----
  function applyLang(){
    document.documentElement.lang = lang;
    document.title = t('title');
    var els = document.querySelectorAll('[data-i18n]');
    for(var i=0;i<els.length;i++){ els[i].textContent = t(els[i].getAttribute('data-i18n')); }
    var phs = document.querySelectorAll('[data-i18n-ph]');
    for(var j=0;j<phs.length;j++){ phs[j].placeholder = t(phs[j].getAttribute('data-i18n-ph')); }
    var auto = document.querySelector('.i18n-auto');
    if(auto){ auto.textContent = t('autorefresh').replace('{n}', auto.getAttribute('data-n')); }
    var btns = document.querySelectorAll('.langtoggle button');
    for(var k=0;k<btns.length;k++){
      btns[k].classList.toggle('active', btns[k].getAttribute('data-lang') === lang);
    }
    applyFilter(); // 件数表示を現在の言語で更新
  }

  // 言語トグル（インライン onclick から呼ぶためグローバルに公開）
  window.setLang = function(l){
    if(l !== 'ja' && l !== 'en') return;
    lang = l;
    try { localStorage.setItem('lang', l); } catch(e){}
    applyLang();
  };

  applyLang();
})();
</script>
</body>
</html>
`
