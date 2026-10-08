package uitests

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// longUnbreakableToken mirrors the pathological preset data that triggers the
// bug: a command fragment pasted into a preset description with no soft wrap
// opportunities (no spaces, slashes or hyphens inside the token).
const longUnbreakableToken = `blk\.((0|1|2|4|5|6|8|9|10|12|13|14|16|17|18|20|21|22|24|25|26|28|29|30|32|33|34|36|37|38|40|41|42|44|45|46|48|49|50|52|53|54|56|57|58|60|61|62|64|65|66))\.(ffn_up|ffn_down)\.weight=CPU`

// browserCandidates returns the headless-browser binaries to try, in order.
func browserCandidates() []string {
	var candidates []string
	if env := os.Getenv("PRESETDOCK_BROWSER"); env != "" {
		candidates = append(candidates, env)
	}
	switch runtime.GOOS {
	case "windows":
		candidates = append(candidates,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		)
	case "darwin":
		candidates = append(candidates,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		)
	default:
		candidates = append(candidates,
			"/usr/bin/google-chrome",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
		)
	}
	return candidates
}

func findBrowser(t *testing.T) string {
	t.Helper()
	for _, c := range browserCandidates() {
		if c == "" {
			continue
		}
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	t.Skip("no Chrome/Edge binary found for layout test (set PRESETDOCK_BROWSER to override)")
	return ""
}

func frontendDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "frontend"))
	if err != nil {
		t.Fatalf("resolve frontend dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		t.Fatalf("frontend dir not found at %s: %v", dir, err)
	}
	return dir
}

var scriptTagRE = regexp.MustCompile(`(?m)^[ \t]*<script src="/[^"]+" defer></script>\r?\n`)

// buildHarness copies the real frontend assets into a temp dir and derives a
// harness page from the real index.html markup. The harness renders cards
// through the real panes.js code and writes a JSON measurement report into
// <pre id="result">.
func buildHarness(t *testing.T) string {
	t.Helper()
	src := frontendDir(t)
	dst := t.TempDir()

	for _, name := range []string{"styles.css", "state.js", "dom.js", "panes.js"} {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dst, name), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	raw, err := os.ReadFile(filepath.Join(src, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	page := scriptTagRE.ReplaceAllString(string(raw), "")
	page = strings.Replace(page, `href="/styles.css"`, `href="styles.css"`, 1)

	head := strings.Replace(harnessHead, `"__TOKEN__"`, strconv.Quote(longUnbreakableToken), 1)
	page = strings.Replace(page, "</head>", head+"</head>", 1)

	harnessPath := filepath.Join(dst, "harness.html")
	if err := os.WriteFile(harnessPath, []byte(page), 0o644); err != nil {
		t.Fatalf("write harness: %v", err)
	}
	return harnessPath
}

// harnessHead is injected after the original scripts were stripped: it loads
// only the modules the layout depends on and runs the measurement. The mode is
// selected with the #dual URL fragment.
const harnessHead = `
<script src="state.js" defer></script>
<script src="dom.js" defer></script>
<script src="panes.js" defer></script>
<script>
window.addEventListener('load', function () {
  var report = { mode: 'single', panes: [], error: null };
  try {
    report.mode = location.hash === '#dual' ? 'dual' : 'single';
    var token = "__TOKEN__";
    setPresets([
      { id: 'normal-1', name: 'Normal preset', model: 'model.gguf', tags: ['demo'],
        description: 'A normal description.', command: 'llama-server -m model.gguf', engine: 'llama-server' },
      { id: 'long-1', name: 'Preset with a long description token', model: '', tags: [],
        description: token, command: 'llama-server -ot "' + token + '"', engine: 'llama-server' },
      { id: 'normal-2', name: 'Another preset', model: 'other.gguf', tags: ['demo'],
        description: 'Short.', command: 'llama-cli -m other.gguf', engine: 'llama-cli' }
    ]);
    setDecks([]);
    setFavourites([]);
    leftPane = createPaneState(document.getElementById('pane-left'));
    rightPane = createPaneState(document.getElementById('pane-right'));
    setViewModeUI(report.mode);
    renderPane(leftPane);
    ['left', 'right'].forEach(function (side) {
      var pane = document.getElementById('pane-' + side);
      if (pane.hidden) return;
      var pr = pane.getBoundingClientRect();
      var p = { name: side, left: pr.left, right: pr.right, cards: [] };
      Array.prototype.forEach.call(pane.querySelectorAll('.card'), function (el) {
        var cr = el.getBoundingClientRect();
        var title = el.querySelector('h2');
        p.cards.push({ title: title ? title.textContent : '', left: cr.left, right: cr.right });
      });
      report.panes.push(p);
    });
  } catch (e) {
    report.error = String((e && e.stack) || e);
  }
  var pre = document.createElement('pre');
  pre.id = 'result';
  pre.textContent = JSON.stringify(report);
  document.body.appendChild(pre);
});
</script>
`

type cardReport struct {
	Title string  `json:"title"`
	Left  float64 `json:"left"`
	Right float64 `json:"right"`
}

type paneReport struct {
	Name  string       `json:"name"`
	Left  float64      `json:"left"`
	Right float64      `json:"right"`
	Cards []cardReport `json:"cards"`
}

type layoutReport struct {
	Mode  string       `json:"mode"`
	Panes []paneReport `json:"panes"`
	Error string       `json:"error"`
}

var resultRE = regexp.MustCompile(`(?s)<pre id="result">(.*?)</pre>`)

// runHarness opens the harness page in a headless browser and returns the
// measurement report written by the page.
func runHarness(t *testing.T, browser, harnessPath, hash string, width, height int) layoutReport {
	t.Helper()

	harnessURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(harnessPath)}).String() + hash

	cmd := exec.Command(browser,
		"--headless=new",
		"--disable-gpu",
		"--hide-scrollbars",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-extensions",
		fmt.Sprintf("--window-size=%d,%d", width, height),
		"--virtual-time-budget=4000",
		"--dump-dom",
		harnessURL,
	)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		t.Fatalf("start browser: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("browser run failed: %v\noutput:\n%s", err, out.String())
		}
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		t.Fatalf("browser timed out\noutput:\n%s", out.String())
	}

	dumped := out.String()
	m := resultRE.FindStringSubmatch(dumped)
	if m == nil {
		t.Fatalf("no measurement result in browser output:\n%s", dumped)
	}

	var report layoutReport
	if err := json.Unmarshal([]byte(html.UnescapeString(m[1])), &report); err != nil {
		t.Fatalf("parse measurement: %v\nraw: %s", err, m[1])
	}
	if report.Error != "" {
		t.Fatalf("harness script error: %s", report.Error)
	}
	return report
}

// assertCardsFit verifies every rendered card stays inside its own pane, i.e.
// cards never extend past the pane and overlap the neighbouring pane's cards.
func assertCardsFit(t *testing.T, report layoutReport, wantPanes int) {
	t.Helper()

	if len(report.Panes) != wantPanes {
		t.Fatalf("expected %d measured panes, got %d", wantPanes, len(report.Panes))
	}
	total := 0
	for _, p := range report.Panes {
		if p.Right <= p.Left {
			t.Errorf("pane %q has non-positive width: [%f, %f]", p.Name, p.Left, p.Right)
		}
		for _, c := range p.Cards {
			total++
			// 1px tolerance for device-pixel rounding.
			if c.Left < p.Left-1 || c.Right > p.Right+1 {
				t.Errorf("card %q extends outside pane %q: card x=[%.1f, %.1f], pane x=[%.1f, %.1f] (overflow left %.1fpx, right %.1fpx)",
					c.Title, p.Name, c.Left, c.Right, p.Left, p.Right, p.Left-c.Left, c.Right-p.Right)
			}
		}
	}
	if total == 0 {
		t.Fatal("no cards measured")
	}
}

// TestCardsStayWithinPanes renders presets that contain long unbreakable
// description tokens and checks that cards never grow wider than their pane,
// in dual view (the reported bug) and in single view on a narrow window.
func TestCardsStayWithinPanes(t *testing.T) {
	browser := findBrowser(t)
	harness := buildHarness(t)

	cases := []struct {
		name      string
		hash      string
		width     int
		height    int
		wantPanes int
	}{
		{name: "dual view", hash: "#dual", width: 1400, height: 900, wantPanes: 2},
		{name: "single view narrow window", hash: "", width: 1000, height: 800, wantPanes: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := runHarness(t, browser, harness, tc.hash, tc.width, tc.height)
			assertCardsFit(t, report, tc.wantPanes)
		})
	}
}
