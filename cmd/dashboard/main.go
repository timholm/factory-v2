package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

func main() {
	addr := ":7777"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Factory</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
*{margin:0;padding:0}
body{background:#000;color:#0f0;font:13px/1.5 'SF Mono','Fira Code',monospace;padding:8px}
#t{height:100vh;overflow-y:auto;white-space:pre-wrap;word-wrap:break-word}
.e{color:#f44}.w{color:#fa0}.s{color:#0fa;font-weight:bold}
.o{color:#f8c}.v{color:#c8f}.b{color:#4af}.r{color:#f68;font-weight:bold}
.c{color:#555;border-top:1px solid #222;padding-top:4px;margin-top:4px}
.w0{color:#4af}.w1{color:#fa4}.w2{color:#4fa}.w3{color:#f4a}.w4{color:#af4}.w5{color:#a4f}
</style></head><body><div id="t"></div><script>
const t=document.getElementById('t');
const es=new EventSource('/stream');
es.onmessage=function(e){
  const d=document.createElement('div');
  let x=e.data;
  if(x.includes('[overseer]'))d.className='r';
  else if(x.includes('[opus:'))d.className='o';
  else if(x.includes('[oracle]'))d.className='v';
  else if(x.includes('[claude:')){let m=x.match(/\[claude:(\w+)\]/);if(m){let h=0;for(let c of m[1])h=((h<<5)-h)+c.charCodeAt(0);d.className='w'+Math.abs(h%6)}}
  else if(x.includes('[worker')){let m=x.match(/\[worker (\d)/);if(m)d.className='w'+m[1]}
  else if(x.includes('shipped'))d.className='s';
  else if(x.includes('error')||x.includes('ERROR')||x.includes('failed'))d.className='e';
  else if(x.includes('CYCLE'))d.className='c';
  d.textContent=x;
  t.appendChild(d);
  while(t.children.length>1000)t.removeChild(t.firstChild);
  t.scrollTop=t.scrollHeight;
};
</script></body></html>`)
	})

	http.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "no flush", 500)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := make(chan string, 100)
		mu.Lock()
		clients[ch] = true
		mu.Unlock()
		defer func() { mu.Lock(); delete(clients, ch); mu.Unlock() }()

		// Send last 200 lines
		f, _ := os.Open("/tmp/factory-v2.log")
		if f != nil {
			var lines []string
			s := bufio.NewScanner(f)
			for s.Scan() {
				lines = append(lines, s.Text())
				if len(lines) > 200 {
					lines = lines[1:]
				}
			}
			f.Close()
			for _, l := range lines {
				fmt.Fprintf(w, "data: %s\n\n", l)
			}
			flusher.Flush()
		}

		for {
			select {
			case <-r.Context().Done():
				return
			case line := <-ch:
				fmt.Fprintf(w, "data: %s\n\n", line)
				flusher.Flush()
			}
		}
	})

	go tailLog()
	log.Printf("Factory terminal at http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

var (
	clients = make(map[chan string]bool)
	mu      sync.Mutex
)

func tailLog() {
	f, err := os.Open("/tmp/factory-v2.log")
	if err != nil {
		return
	}
	defer f.Close()
	f.Seek(0, io.SeekEnd)
	scanner := bufio.NewScanner(f)
	for {
		for scanner.Scan() {
			line := scanner.Text()
			mu.Lock()
			for ch := range clients {
				select {
				case ch <- line:
				default:
				}
			}
			mu.Unlock()
		}
		scanner = bufio.NewScanner(f)
	}
}
