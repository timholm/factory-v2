package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

func main() {
	addr := ":7777"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	http.HandleFunc("/", handleDashboard)
	http.HandleFunc("/stream", handleStream)
	http.HandleFunc("/stats", handleStats)
	http.HandleFunc("/overseer", handleOverseer)
	http.HandleFunc("/sessions", handleSessions)

	log.Printf("Factory dashboard at http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
<title>Factory v2 — Command Center</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { background: #0a0a0f; color: #00ff88; font-family: 'SF Mono', 'Fira Code', monospace; }
#header { padding: 12px 20px; background: #111118; border-bottom: 1px solid #222; display: flex; justify-content: space-between; align-items: center; }
#header h1 { font-size: 16px; color: #00ff88; }
#stats { display: flex; gap: 24px; font-size: 13px; color: #888; }
#stats .val { color: #00ff88; font-weight: bold; }
#stats .bad { color: #ff4444; }
.panels { display: grid; grid-template-columns: 1fr 1fr; grid-template-rows: 1fr 1fr; height: calc(100vh - 50px); gap: 1px; background: #222; }
.panel { background: #0a0a0f; overflow-y: auto; padding: 8px 12px; font-size: 12px; line-height: 1.5; white-space: pre-wrap; word-wrap: break-word; }
.panel-header { position: sticky; top: 0; background: #111118; padding: 4px 8px; font-size: 11px; color: #888; text-transform: uppercase; letter-spacing: 1px; border-bottom: 1px solid #333; margin: -8px -12px 8px; }
#terminal { grid-column: 1; grid-row: 1 / 3; }
#overseer { grid-column: 2; grid-row: 1; }
#sessions { grid-column: 2; grid-row: 2; }
@media (max-width: 800px) { .panels { grid-template-columns: 1fr; grid-template-rows: 1fr 1fr 1fr; } #terminal { grid-column: 1; grid-row: 1; } #overseer { grid-column: 1; grid-row: 2; } #sessions { grid-column: 1; grid-row: 3; } }
.line { }
.time { color: #555; }
.info { color: #00ff88; }
.warn { color: #ffaa00; }
.err { color: #ff4444; }
.build { color: #44aaff; }
.oracle { color: #ff88ff; }
.ship { color: #00ffaa; font-weight: bold; }
.cycle { color: #888; border-top: 1px solid #222; padding-top: 8px; margin-top: 8px; }
</style>
</head>
<body>
<div id="header">
  <h1>FACTORY v2</h1>
  <div id="stats">
    <span>Repos: <span class="val" id="repos">—</span></span>
    <span>Shipped: <span class="val" id="shipped">—</span></span>
    <span>Building: <span class="val" id="building">—</span></span>
    <span>Failed: <span class="val bad" id="failed">—</span></span>
    <span>Embeddings: <span class="val" id="embeddings">—</span></span>
    <span>Pods: <span class="val" id="pods">—</span></span>
  </div>
</div>
<div class="panels">
<div id="terminal" class="panel"><div class="panel-header">Factory Log (live)</div></div>
<div id="overseer" class="panel"><div class="panel-header">Overseer (Opus critique)</div><div id="overseer-content">Loading...</div></div>
<div id="sessions" class="panel"><div class="panel-header">Active Claude Sessions</div><div id="sessions-content">Loading...</div></div>
</div>
<script>
const term = document.getElementById('terminal');
const evtSource = new EventSource('/stream');
evtSource.onmessage = function(e) {
  const line = document.createElement('div');
  line.className = 'line';
  let text = e.data;
  if (text.includes('[oracle]')) line.classList.add('oracle');
  else if (text.includes('[build]')) line.classList.add('build');
  else if (text.includes('shipped')) line.classList.add('ship');
  else if (text.includes('error') || text.includes('ERROR') || text.includes('failed')) line.classList.add('err');
  else if (text.includes('warning') || text.includes('WARN')) line.classList.add('warn');
  else if (text.includes('CYCLE')) line.classList.add('cycle');
  line.textContent = text;
  term.appendChild(line);
  // Keep last 500 lines
  while (term.children.length > 500) term.removeChild(term.firstChild);
  term.scrollTop = term.scrollHeight;
};
// Poll stats every 10s
function updateStats() {
  fetch('/stats').then(r => r.json()).then(d => {
    document.getElementById('repos').textContent = d.repos || '—';
    document.getElementById('shipped').textContent = d.shipped || '—';
    document.getElementById('building').textContent = d.building || '—';
    document.getElementById('failed').textContent = d.failed || '—';
    document.getElementById('embeddings').textContent = d.embeddings || '—';
    document.getElementById('pods').textContent = d.pods || '—';
  }).catch(() => {});
}
updateStats();
setInterval(updateStats, 10000);

// Poll overseer
function updateOverseer() {
  fetch('/overseer').then(r => r.text()).then(t => {
    document.getElementById('overseer-content').innerHTML = t.split('\n').map(line => {
      let cls = '';
      if (line.includes('[critical]')) cls = 'err';
      else if (line.includes('[high]')) cls = 'warn';
      else if (line.includes('Tim would say')) cls = 'ship';
      else if (line.includes('Frustration')) cls = 'oracle';
      return '<div class="line ' + cls + '">' + line.replace(/</g,'&lt;') + '</div>';
    }).join('');
  }).catch(() => {});
}
updateOverseer();
setInterval(updateOverseer, 15000);

// Poll active tmux sessions
function updateSessions() {
  fetch('/sessions').then(r => r.json()).then(sessions => {
    let html = '';
    if (sessions.length === 0) {
      html = '<div class="line" style="color:#555">No active build sessions</div>';
    }
    sessions.forEach(s => {
      html += '<div class="line build"><b>' + s.name + '</b></div>';
      html += '<div class="line" style="color:#666;margin-left:8px">' + (s.preview||'').replace(/</g,'&lt;') + '</div>';
      html += '<div style="height:4px"></div>';
    });
    document.getElementById('sessions-content').innerHTML = html;
  }).catch(() => {});
}
updateSessions();
setInterval(updateSessions, 5000);
</script>
</body>
</html>`)
}

var (
	clients   = make(map[chan string]bool)
	clientsMu sync.Mutex
)

func handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan string, 100)
	clientsMu.Lock()
	clients[ch] = true
	clientsMu.Unlock()

	defer func() {
		clientsMu.Lock()
		delete(clients, ch)
		clientsMu.Unlock()
	}()

	// Send last 100 lines of existing log
	sendTail(w, flusher)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case line := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		}
	}
}

func sendTail(w http.ResponseWriter, flusher http.Flusher) {
	f, err := os.Open("/tmp/factory-v2.log")
	if err != nil {
		return
	}
	defer f.Close()

	// Read last 100 lines
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > 100 {
			lines = lines[1:]
		}
	}
	for _, line := range lines {
		fmt.Fprintf(w, "data: %s\n\n", line)
	}
	flusher.Flush()
}

func init() {
	// Background goroutine that tails the log and broadcasts to all SSE clients
	go func() {
		for {
			tailLog()
			time.Sleep(time.Second)
		}
	}()
}

func tailLog() {
	f, err := os.Open("/tmp/factory-v2.log")
	if err != nil {
		return
	}
	defer f.Close()

	// Seek to end
	f.Seek(0, io.SeekEnd)

	scanner := bufio.NewScanner(f)
	for {
		for scanner.Scan() {
			line := scanner.Text()
			clientsMu.Lock()
			for ch := range clients {
				select {
				case ch <- line:
				default:
				}
			}
			clientsMu.Unlock()
		}
		time.Sleep(500 * time.Millisecond)

		// Re-read from current position
		scanner = bufio.NewScanner(f)
	}
}

func handleOverseer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	out, err := exec.Command("tail", "-50", "/tmp/factory-overseer.log").CombinedOutput()
	if err != nil {
		fmt.Fprint(w, "No overseer output yet")
		return
	}
	w.Write(out)
}

type sessionInfo struct {
	Name    string `json:"name"`
	Preview string `json:"preview"`
}

func handleSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	out, err := exec.Command("tmux", "list-sessions", "-F", "#{session_name}").CombinedOutput()
	if err != nil {
		json.NewEncoder(w).Encode([]sessionInfo{})
		return
	}

	var sessions []sessionInfo
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name == "" || name == "claude-ui" {
			continue
		}
		// Capture last 3 lines of the tmux pane
		preview, _ := exec.Command("tmux", "capture-pane", "-t", name, "-p", "-l", "3").CombinedOutput()
		sessions = append(sessions, sessionInfo{
			Name:    name,
			Preview: strings.TrimSpace(string(preview)),
		})
	}

	json.NewEncoder(w).Encode(sessions)
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]string{}

	// GitHub repos
	if out, err := exec.CommandContext(context.Background(), "gh", "repo", "list", "timholm", "--limit", "200", "--json", "name,isArchived", "--jq", `[.[] | select(.isArchived == false)] | length`).Output(); err == nil {
		stats["repos"] = strings.TrimSpace(string(out))
	}

	// K8s pods
	if out, err := exec.Command("kubectl", "get", "pods", "-n", "factory", "--no-headers").Output(); err == nil {
		running := 0
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "Running") {
				running++
			}
		}
		stats["pods"] = fmt.Sprintf("%d", running)
	}

	// Embeddings
	if out, err := exec.Command("kubectl", "exec", "postgres-0", "-n", "factory", "--", "psql", "-U", "factory", "-d", "arxiv", "-tAc",
		"SELECT ROUND(COUNT(*)::numeric * 100 / (SELECT COUNT(*) FROM papers), 1) || '%' FROM papers WHERE embedding IS NOT NULL").Output(); err == nil {
		stats["embeddings"] = strings.TrimSpace(string(out))
	}

	// Build queue (from Postgres)
	pgURL := os.Getenv("POSTGRES_URL")
	if pgURL == "" {
		stats["shipped"] = "?"
		stats["building"] = "?"
		stats["failed"] = "?"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
