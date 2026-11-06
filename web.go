//go:build !windows

// web.go —— 简易 Web 前端（Linux/macOS 命令行版内置）
//
// 提供浏览器界面：
//
//	GET  /                HTML 页面
//	GET  /api/list?root= 列出目录下的子目录
//	POST /api/run        启动字幕匹配作业
//	GET  /api/status     轮询作业进度
//	POST /api/cancel     取消作业
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"
)

// ---- 作业状态 ----

type jobState struct {
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	root    string
	targets []target
	rows    map[int]*jobRow
	order   []int
	summary summary
	started time.Time
}

type jobRow struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Dir       string `json:"dir"`
	Status    string `json:"status"`
	Source    string `json:"source"`
	Lang      string `json:"lang"`
	Size      string `json:"size"`
	Detail    string `json:"detail"`
	Downloads int    `json:"downloads"`
	IsMachine bool   `json:"isMachine"`
}

var (
	job     = &jobState{rows: map[int]*jobRow{}}
	jobOnce sync.Mutex
)

// ---- HTTP 入口 ----

func runWeb(addr string) {
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/api/list", handleList)
	http.HandleFunc("/api/run", handleRun)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/cancel", handleCancel)
	fmt.Printf("字幕匹配下载器 Web 已启动: http://%s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Println("Web 服务启动失败:", err)
		os.Exit(1)
	}
}

// ---- 页面 ----

const indexHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>番号字幕匹配下载器</title>
<style>
:root{--bg:#0f1115;--card:#1a1d24;--line:#2a2f3a;--txt:#e6e9ef;--dim:#8b93a3;--acc:#4f8cff;--ok:#34c77b;--warn:#f0a020;--err:#f05050}
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,"PingFang SC","Microsoft YaHei",sans-serif;background:var(--bg);color:var(--txt);padding:24px;min-height:100vh}
h1{font-size:20px;margin-bottom:20px;display:flex;align-items:center;gap:10px}
h1 .dot{width:10px;height:10px;border-radius:50%;background:var(--acc);box-shadow:0 0 12px var(--acc)}
.card{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:18px;margin-bottom:16px}
label{display:block;font-size:13px;color:var(--dim);margin-bottom:6px}
.row{display:flex;gap:10px;flex-wrap:wrap;align-items:flex-end}
input[type=text]{flex:1;min-width:260px;background:#12151b;border:1px solid var(--line);border-radius:8px;color:var(--txt);padding:10px 12px;font-size:14px}
input[type=text]:focus{outline:none;border-color:var(--acc)}
button{background:var(--acc);border:none;border-radius:8px;color:#fff;padding:10px 18px;font-size:14px;cursor:pointer;white-space:nowrap}
button:hover{filter:brightness(1.1)}
button:disabled{opacity:.45;cursor:not-allowed}
button.ghost{background:transparent;border:1px solid var(--line);color:var(--dim)}
button.ghost:hover{color:var(--txt);border-color:var(--dim)}
.chk{display:flex;gap:18px;margin-top:12px;flex-wrap:wrap}
.chk label{display:flex;align-items:center;gap:6px;color:var(--txt);font-size:13px;margin:0;cursor:pointer}
.proxy{margin-top:12px;font-size:12px;color:var(--dim)}
.proxy input{background:#12151b;border:1px solid var(--line);border-radius:6px;color:var(--txt);padding:6px 10px;width:100%;font-size:13px;margin-top:4px}
.summary{display:flex;gap:24px;font-size:14px;margin-bottom:8px;flex-wrap:wrap}
.summary b{font-size:18px}
.summary .ok{color:var(--ok)}.summary .skip{color:var(--dim)}.summary .fail{color:var(--err)}.summary .total{color:var(--txt)}
table{width:100%;border-collapse:collapse;font-size:13px}
th,td{padding:8px 10px;text-align:left;border-bottom:1px solid var(--line);white-space:nowrap}
th{color:var(--dim);font-weight:500;position:sticky;top:0;background:var(--card)}
td:nth-child(2){max-width:200px;overflow:hidden;text-overflow:ellipsis}
.st{font-weight:500}
.st-等待中{color:var(--dim)}.st-搜索中{color:var(--acc)}.st-下载中{color:var(--warn)}.st-已完成{color:var(--ok)}.st-已跳过{color:var(--dim)}.st-失败{color:var(--err)}
#list{display:flex;flex-wrap:wrap;gap:6px;margin-top:8px}
#list .d{background:#12151b;border:1px solid var(--line);border-radius:6px;padding:5px 10px;font-size:12px;cursor:pointer;color:var(--txt)}
#list .d:hover{border-color:var(--acc)}
#list .d.sel{border-color:var(--acc);color:var(--acc)}
.empty{color:var(--dim);font-size:13px;padding:8px 0}
#log{font-family:ui-monospace,Menlo,monospace;font-size:12px;color:var(--dim);max-height:120px;overflow-y:auto;margin-top:10px;line-height:1.7}
#status-line{font-size:13px;color:var(--dim);margin-bottom:12px}
</style>
</head>
<body>
<h1><span class="dot"></span>番号字幕匹配下载器</h1>

<div class="card">
  <div class="row">
    <div style="flex:1">
      <label>媒体库目录</label>
      <input type="text" id="root" placeholder="/volume1/New/LTX" value="/volume1/New/LTX">
    </div>
    <button class="ghost" onclick="loadList()">浏览…</button>
    <button id="runBtn" onclick="runJob()">开始匹配</button>
    <button class="ghost" id="cancelBtn" onclick="cancelJob()" style="display:none">取消</button>
  </div>
  <div id="list"></div>
  <div class="chk">
    <label><input type="checkbox" id="onlyCodes" checked>只处理番号视频</label>
    <label><input type="checkbox" id="normalize" checked>自动修正时间轴</label>
    <label><input type="checkbox" id="skipExist" checked>已有字幕则跳过</label>
  </div>
  <div class="proxy">
    <label>代理（可选，如 http://172.17.0.4:7890，留空不使用）</label>
    <input type="text" id="proxy" placeholder="http://172.17.0.4:7890">
  </div>
</div>

<div class="card">
  <div id="status-line">尚未开始</div>
  <div class="summary">
    <span class="total">总数 <b id="s-total">0</b></span>
    <span class="ok">成功 <b id="s-done">0</b></span>
    <span class="skip">跳过 <b id="s-skip">0</b></span>
    <span class="fail">失败 <b id="s-fail">0</b></span>
  </div>
  <table>
    <thead><tr><th>状态</th><th>文件</th><th>目录</th><th>来源</th><th>语言</th><th>下载</th><th>说明</th></tr></thead>
    <tbody id="tbody"></tbody>
  </table>
  <div class="empty" id="empty">选择目录后点击「开始匹配」</div>
  <div id="log"></div>
</div>

<script>
let busy=false;
const $=s=>document.querySelector(s);
function setProxy(){
  const p=$('#proxy').value.trim();
  if(p){window.__proxy=p;}else{delete window.__proxy;}
}
async function loadList(){
  const root=$('#root').value.trim();
  if(!root){return;}
  try{
    const r=await fetch('/api/list?root='+encodeURIComponent(root));
    const j=await r.json();
    const el=$('#list');el.innerHTML='';
    (j.dirs||[]).forEach(d=>{
      const s=document.createElement('span');
      s.className='d';s.textContent=d;
      s.onclick=()=>{$('#root').value=root.replace(/\/$/,'')+'/'+d;s.className='d sel';loadList();};
      el.appendChild(s);
    });
    if(!(j.dirs||[]).length)el.innerHTML='<span class="empty">无子目录或路径无效</span>';
  }catch(e){$('#list').innerHTML='<span class="empty">读取失败</span>';}
}
async function runJob(){
  if(busy)return;
  busy=true;
  setProxy();
  $('#runBtn').disabled=true;$('#cancelBtn').style.display='';
  $('#tbody').innerHTML='';$('#log').textContent='';$('#empty').style.display='none';
  $('#s-total').textContent=0;$('#s-done').textContent=0;$('#s-skip').textContent=0;$('#s-fail').textContent=0;
  $('#status-line').textContent='正在扫描…';
  try{
    const body={root:$('#root').value.trim(),onlyCodes:$('#onlyCodes').checked,normalize:$('#normalize').checked,skipExist:$('#skipExist').checked};
    if(window.__proxy)body.proxy=window.__proxy;
    const r=await fetch('/api/run',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
    const j=await r.json();
    if(!j.ok){alert(j.error||'启动失败');busy=false;$('#runBtn').disabled=false;$('#cancelBtn').style.display='none';return;}
    poll();
  }catch(e){alert('启动失败: '+e);busy=false;$('#runBtn').disabled=false;$('#cancelBtn').style.display='none';}
}
async function poll(){
  try{
    const r=await fetch('/api/status');
    const j=await r.json();
    render(j);
    if(j.running){setTimeout(poll,800);}else{busy=false;$('#runBtn').disabled=false;$('#cancelBtn').style.display='none';}
  }catch(e){setTimeout(poll,1500);}
}
function render(j){
  $('#status-line').textContent=j.running?'运行中（'+j.root+'）':'已完成';
  $('#s-total').textContent=j.summary.total;
  $('#s-done').textContent=j.summary.done;
  $('#s-skip').textContent=j.summary.skipped;
  $('#s-fail').textContent=j.summary.failed;
  const tb=$('#tbody');tb.innerHTML='';
  (j.rows||[]).forEach(r=>{
    const tr=document.createElement('tr');
    const dlTxt=(r.downloads?r.downloads+'次':'—')+(r.isMachine?' <span style="color:var(--warn)">⚠机翻</span>':'');
    tr.innerHTML='<td class="st st-'+r.status+'">'+r.status+'</td><td>'+esc(r.name)+'</td><td>'+esc(r.dir)+'</td><td>'+esc(r.source)+'</td><td>'+esc(r.lang)+'</td><td>'+dlTxt+'</td><td>'+esc(r.detail)+'</td>';
    tb.appendChild(tr);
  });
  const log=$('#log');if(j.log&&j.log.length){log.textContent=j.log.slice(-2000);}
}
function esc(s){return String(s==null?'':s).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));}
async function cancelJob(){
  if(!confirm('确定取消当前任务？'))return;
  try{await fetch('/api/cancel',{method:'POST'});}catch(e){}
}
$('#root').addEventListener('change',loadList);
window.addEventListener('load',()=>{setProxy();poll();});
</script>
</body>
</html>
`

func handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

// ---- API: 列出子目录 ----

func handleList(w http.ResponseWriter, r *http.Request) {
	root := r.URL.Query().Get("root")
	if root == "" {
		writeJSON(w, map[string]interface{}{"dirs": []string{}})
		return
	}
	var dirs []string
	entries, err := os.ReadDir(root)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				dirs = append(dirs, e.Name())
			}
		}
	}
	sort.Strings(dirs)
	writeJSON(w, map[string]interface{}{"dirs": dirs})
}

// ---- API: 启动作业 ----

func handleRun(w http.ResponseWriter, r *http.Request) {
	jobOnce.Lock()
	defer jobOnce.Unlock()
	if job.running {
		writeJSON(w, map[string]interface{}{"ok": false, "error": "已有任务在运行"})
		return
	}
	var req struct {
		Root      string `json:"root"`
		OnlyCodes *bool  `json:"onlyCodes"`
		Normalize *bool  `json:"normalize"`
		SkipExist *bool  `json:"skipExist"`
		Proxy     string `json:"proxy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "error": "请求解析失败"})
		return
	}
	if req.Root == "" {
		writeJSON(w, map[string]interface{}{"ok": false, "error": "目录不能为空"})
		return
	}
	st, err := os.Stat(req.Root)
	if err != nil || !st.IsDir() {
		writeJSON(w, map[string]interface{}{"ok": false, "error": "目录不存在或不可访问: " + req.Root})
		return
	}

	// 代理：如果前端传了 proxy，应用到环境变量（net 的默认 Transport 会自动读取）
	if req.Proxy != "" {
		os.Setenv("HTTP_PROXY", req.Proxy)
		os.Setenv("HTTPS_PROXY", req.Proxy)
		os.Setenv("http_proxy", req.Proxy)
		os.Setenv("https_proxy", req.Proxy)
	} else {
		// 未指定时清除，避免继承上次的代理
		os.Unsetenv("HTTP_PROXY")
		os.Unsetenv("HTTPS_PROXY")
		os.Unsetenv("http_proxy")
		os.Unsetenv("https_proxy")
	}

	cfg := defaultConfig()
	if req.OnlyCodes != nil {
		cfg.OnlyCodes = *req.OnlyCodes
	}
	if req.Normalize != nil {
		cfg.Normalize = *req.Normalize
	}
	if req.SkipExist != nil {
		cfg.SkipExisting = *req.SkipExist
	}

	targets, ignored, err := scanVideos(req.Root, cfg.OnlyCodes)
	if err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	if len(targets) == 0 {
		writeJSON(w, map[string]interface{}{"ok": false, "error": "没有找到需要配字幕的视频文件"})
		return
	}

	// 重置状态
	job.root = req.Root
	job.rows = map[int]*jobRow{}
	job.order = nil
	job.summary = summary{}
	for i, t := range targets {
		job.rows[i] = &jobRow{Index: i, Name: t.Name, Dir: t.Dir}
		job.order = append(job.order, i)
	}
	job.summary.Total = len(targets)

	ctx, cancel := context.WithCancel(context.Background())
	job.cancel = cancel
	job.running = true

	logBuf := &syncBuffer{}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("作业 panic:", r)
				debug.PrintStack()
			}
			job.mu.Lock()
			job.running = false
			job.mu.Unlock()
		}()
		runAll(ctx, targets, cfg, func(p progress) {
			job.mu.Lock()
			row := job.rows[p.Index]
			if row != nil {
				row.Status = p.Status.Text()
				row.Source = p.Source
				row.Lang = p.Lang
				row.Size = p.Size
				row.Detail = p.Detail
				row.Downloads = p.Downloads
				row.IsMachine = p.IsMachine
			}
			// 记录日志行
			line := fmt.Sprintf("[%d/%d] %-22s %-9s %s", p.Index+1, len(targets), targets[p.Index].Name, p.Status.Text(), p.Detail)
			logBuf.append(line)
			job.mu.Unlock()
		})
		job.mu.Lock()
		job.summary = summarize(ctx, targets)
		job.mu.Unlock()
	}()

	writeJSON(w, map[string]interface{}{"ok": true, "ignored": len(ignored), "total": len(targets)})
}

// summarize 重新计算汇总。调用方必须已持有 job.mu。
func summarize(ctx context.Context, targets []target) summary {
	var s summary
	s.Total = len(targets)
	for _, r := range job.rows {
		switch r.Status {
		case "已完成":
			s.Done++
		case "已跳过":
			s.Skipped++
		case "失败":
			s.Failed++
		}
	}
	return s
}

// ---- API: 状态 ----

func handleStatus(w http.ResponseWriter, r *http.Request) {
	job.mu.Lock()
	defer job.mu.Unlock()
	rows := make([]*jobRow, 0, len(job.order))
	for _, idx := range job.order {
		rows = append(rows, job.rows[idx])
	}
	writeJSON(w, map[string]interface{}{
		"running": job.running,
		"root":    job.root,
		"rows":    rows,
		"summary": job.summary,
	})
}

// ---- API: 取消 ----

func handleCancel(w http.ResponseWriter, r *http.Request) {
	job.mu.Lock()
	cancel := job.cancel
	job.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	writeJSON(w, map[string]interface{}{"ok": true})
}

// ---- 工具 ----

type syncBuffer struct {
	mu    sync.Mutex
	lines []string
}

func (b *syncBuffer) append(s string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, s)
	if len(b.lines) > 2000 {
		b.lines = b.lines[len(b.lines)-2000:]
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

var _ = fs.ErrNotExist
var _ = filepath.Join
var _ = time.Now
