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

	log.Printf("Factory dashboard at http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
<title>Factory v2 — Live Terminal</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { background: #0a0a0f; color: #00ff88; font-family: 'SF Mono', 'Fira Code', monospace; }
#header { padding: 12px 20px; background: #111118; border-bottom: 1px solid #222; display: flex; justify-content: space-between; align-items: center; }
#header h1 { font-size: 16px; color: #00ff88; }
#stats { display: flex; gap: 24px; font-size: 13px; color: #888; }
#stats .val { color: #00ff88; font-weight: bold; }
#stats .bad { color: #ff4444; }
#terminal { padding: 12px 20px; overflow-y: auto; height: calc(100vh - 50px); font-size: 13px; line-height: 1.6; white-space: pre-wrap; word-wrap: break-word; }
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
<div id="terminal"></div>
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
