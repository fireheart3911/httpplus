package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type errDef struct {
	Name   string
	Desc   string
	Footer string
}

var defs = map[int]errDef{
	600: {"Skill Issue", "", "RTFM"},
	601: {"Vibe Check Failed", "The server was asked nicely and said no", "nginx"},
	602: {"Insufficient Braincells", "Client allocated insufficient cognitive ressources to this request", "nginx"},
	603: {"Touch Grass", "Oxygen intake recommended", "nginx"},
	604: {"Absolutely Not", "just no", "nginx"},
	605: {"Unscheduled Kinetic Disassembly", "The server has spontaneously separated into its component parts", "nginx"},
	606: {"Rapid Thermal Combustion", "Ressource unavailable due to ongoing pyrolysis", "nginx"},
	607: {"Excessive Cringe", "User is advised to grow up", "nginx"},
	608: {"Pineapple Detected", "javaLangIllegalArgumentException", "nginx"},
	609: {"It is Tuesday", "Check the calendar", "nginx"},
}

var catsDir = "cats"

// is it tuesday yet?
var (
	maintDay      = time.Tuesday
	maintStartMin = 3 * 60
	maintEndMin   = 4 * 60
	downCode      = 609
)

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday, "tues": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday, "thurs": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

func parseHHMM(s string, def int) int {
	var h, m int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d:%d", &h, &m); err != nil {
		return def
	}
	return h*60 + m
}

func inBackupWindow(t time.Time) bool {
	if t.Weekday() != maintDay { return false }
	cur := t.Hour()*60 + t.Minute()
	return cur >= maintStartMin && cur < maintEndMin
}

const pageTmpl = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>%d %s</title>
<style>
html{color-scheme:light dark}
body{font-family:"Times New Roman",Times,serif;margin:0}
h1{font-size:2em;font-weight:bold;margin:.67em 0}
center{display:block;text-align:center}
hr{border:0;border-top:1px solid #888;margin:.5em 0}
</style>
</head>
<body>
<center><h1>%d %s</h1></center>
<hr><center>%s</center>
</body>
</html>
`

func buildPage(code int, d errDef) string { return fmt.Sprintf(pageTmpl, code, d.Name, code, d.Name, d.Footer) }

// steal the header >:D
func writeRaw(conn net.Conn, code int, reason, contentType string, body []byte) {
	w := bufio.NewWriter(conn)
	fmt.Fprintf(w, "HTTP/1.1 %d %s\r\n", code, reason)
	fmt.Fprint(w, "Server: nginx\r\n")
	fmt.Fprintf(w, "Content-Type: %s\r\n", contentType)
	fmt.Fprintf(w, "Content-Length: %d\r\n", len(body))
	fmt.Fprint(w, "Connection: close\r\n\r\n")
	w.Write(body)
	_ = w.Flush()
}

func respond(w http.ResponseWriter, code int, reason, contentType string, body []byte) {
	if hj, ok := w.(http.Hijacker); ok {
		if conn, _, err := hj.Hijack(); err == nil {
			defer conn.Close()
			writeRaw(conn, code, reason, contentType, body)
			return
		}
	}
	w.Header().Set("Server", "nginx")
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(code)
	w.Write(body)
}

func indexPage() string {
	codes := make([]int, 0, len(defs))
	for c := range defs {
		codes = append(codes, c)
	}
	sort.Ints(codes)
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>httpplus</title>`)
	b.WriteString(`<style>body{font-family:"Times New Roman",serif;max-width:60em;margin:2em auto;padding:0 1em}a{text-decoration:none}li{margin:.25em 0;white-space:nowrap}.tag{opacity:.7}footer{margin-top:1.5em;font-size:.9em;opacity:.7}</style></head><body>`)
	b.WriteString("<h1>httpplus</h1>")
	b.WriteString(`<p class="tag">an expansion to classic http codes to address issues occurring in layer 8.</p>`)
	b.WriteString("<ul>")
	for _, c := range codes {
		d := defs[c]
		note := d.Desc
		if c == 600 {
			note = "RTFM" // we love edge cases
		}
		desc := ""
		if note != "" {
			desc = " - " + note
		}
		fmt.Fprintf(&b, `<li><a href="/%d">%d %s</a>%s <a href="/cat/%d">[cat]</a></li>`, c, c, d.Name, desc, c)
	}
	b.WriteString("</ul>")
	b.WriteString(`<footer>Made by <a href="https://gh.fireheart.dev/httpplus">fireheart</a></footer>`)
	b.WriteString("</body></html>")
	return b.String()
}

func serveCat(w http.ResponseWriter, code int) {
	data, err := os.ReadFile(filepath.Join(catsDir, strconv.Itoa(code)+".png")) // any path traversal enjoyers
	if err != nil {
		respond(w, http.StatusAccepted, "Accepted", "text/plain; charset=utf-8",
			[]byte("202 Accepted — cat pending\n"))
		return
	}
	respond(w, code, defs[code].Name, "image/png", data)
}

func handler(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")

	if path == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, indexPage())
		return
	}

	if path == "down" {
		code := downCode
		if inBackupWindow(time.Now()) {
			code = 609
		}
		d := defs[code]
		respond(w, code, d.Name, "text/html; charset=utf-8", []byte(buildPage(code, d)))
		return
	}

	if rest, ok := strings.CutPrefix(path, "cat/"); ok {
		code, err := strconv.Atoi(rest)
		if _, known := defs[code]; err != nil || !known {
			http.Redirect(w, r, "https://http.cat/204", http.StatusFound)
			return
		}
		serveCat(w, code)
		return
	}

	if code, err := strconv.Atoi(path); err == nil {
		if d, ok := defs[code]; ok {
			respond(w, code, d.Name, "text/html; charset=utf-8", []byte(buildPage(code, d)))
			return
		}
	}
	http.NotFound(w, r)
}

// tower of procrastination
func main() {
	port := os.Getenv("SERVER_PORT")
	if port == "" {port = "8080"}
	if cd := os.Getenv("CATS_DIR"); cd != "" {catsDir = cd}
	if v, ok := weekdays[strings.ToLower(os.Getenv("MAINT_DAY"))]; ok {maintDay = v}
	if s := os.Getenv("MAINT_START"); s != "" {maintStartMin = parseHHMM(s, maintStartMin)}
	if s := os.Getenv("MAINT_END"); s != "" {maintEndMin = parseHHMM(s, maintEndMin)}
	if s := os.Getenv("DOWN_CODE"); s != "" {
		if c, err := strconv.Atoi(s); err == nil {
			if _, ok := defs[c]; ok {
				downCode = c
  }}}
	addr := "0.0.0.0:" + port
	http.HandleFunc("/", handler)
	log.Printf("httpplus: serving on %s (cats: %s)", addr, catsDir)
	if err := http.ListenAndServe(addr, nil); err != nil {log.Fatal(err)}
}
