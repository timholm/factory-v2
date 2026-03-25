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
		fmt.Fprint(w, `<!DOCTYPE html><html><head><meta name="viewport" content="width=device-width,initial-scale=1">
<style>body{background:#000;color:#999;font:9px/1.3 monospace;padding:4px;margin:0}
div{white-space:pre-wrap;word-break:break-all}
.e{color:#c33}.s{color:#0a7;font-weight:700}.o{color:#b6a}.v{color:#86b}.r{color:#c46}
.c{color:#444;border-top:1px solid #111;margin-top:2px;padding-top:2px}
.w0{color:#58d}.w1{color:#c84}.w2{color:#4b8}.w3{color:#c4a}.w4{color:#8b4}.w5{color:#84c}
</style></head><body><div id=t></div><script>
var t=document.getElementById('t'),es=new EventSource('/s');
es.onmessage=function(e){var d=document.createElement('div'),x=e.data;
if(x.includes('[overseer]'))d.className='r';
else if(x.includes('[opus:'))d.className='o';
else if(x.includes('[oracle]'))d.className='v';
else if(x.includes('[claude:')){var m=x.match(/\[claude:(\w+)\]/);if(m){var h=0;for(var c of m[1])h=((h<<5)-h)+c.charCodeAt(0);d.className='w'+Math.abs(h%6)}}
else if(x.includes('[worker')){var m=x.match(/\[worker (\d)/);if(m)d.className='w'+m[1]}
else if(x.includes('shipped'))d.className='s';
else if(x.includes('error')||x.includes('failed'))d.className='e';
else if(x.includes('CYCLE'))d.className='c';
d.textContent=x;t.appendChild(d);
while(t.children.length>2000)t.removeChild(t.firstChild);
t.scrollTop=t.scrollHeight};
</script></body></html>`)
	})

	http.HandleFunc("/s", func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok { return }
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		ch := make(chan string, 200)
		mu.Lock(); clients[ch] = true; mu.Unlock()
		defer func() { mu.Lock(); delete(clients, ch); mu.Unlock() }()
		if file, err := os.Open("/tmp/factory-v2.log"); err == nil {
			var lines []string
			s := bufio.NewScanner(file)
			for s.Scan() { lines = append(lines, s.Text()); if len(lines) > 500 { lines = lines[1:] } }
			file.Close()
			for _, l := range lines { fmt.Fprintf(w, "data: %s\n\n", l) }
			f.Flush()
		}
		for { select { case <-r.Context().Done(): return; case l := <-ch: fmt.Fprintf(w, "data: %s\n\n", l); f.Flush() } }
	})

	go tailLog()
	log.Printf("http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

var (clients = make(map[chan string]bool); mu sync.Mutex)

func tailLog() {
	f, _ := os.Open("/tmp/factory-v2.log")
	if f == nil { return }
	defer f.Close()
	f.Seek(0, io.SeekEnd)
	s := bufio.NewScanner(f)
	for { for s.Scan() { l := s.Text(); mu.Lock(); for ch := range clients { select { case ch <- l: default: } }; mu.Unlock() }; s = bufio.NewScanner(f) }
}
