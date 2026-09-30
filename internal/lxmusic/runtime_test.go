package lxmusic

import (
	"context"
	"crypto/aes"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const header = "/**\n * @name Test source\n * @version 1.0.0\n */\n"

// startTest runs script with a client that may reach loopback test servers.
func startTest(t *testing.T, script string) (*runtime, map[string][]string, error) {
	t.Helper()
	client, _ := newClients(true)
	r, platforms, err := startRuntime(context.Background(), runtimeConfig{
		id: "test", info: ScriptInfo{Name: "Test source"}, script: header + script, client: client,
	})
	if r != nil {
		t.Cleanup(func() { r.close(nil) })
	}
	return r, platforms, err
}

func shortLimits(t *testing.T) {
	t.Helper()
	oldInit, oldCall, oldTask := initTimeout, callTimeout, taskTimeout
	initTimeout, callTimeout, taskTimeout = 2*time.Second, 2*time.Second, 300*time.Millisecond
	t.Cleanup(func() { initTimeout, callTimeout, taskTimeout = oldInit, oldCall, oldTask })
}

const initKw = `
lx.send(lx.EVENT_NAMES.inited, { sources: {
  kw: { name: 'kw', type: 'music', actions: ['musicUrl'], qualitys: ['flac', '128k', '320k', 'bogus'] },
  kg: { type: 'music', actions: ['lyric'], qualitys: ['128k'] },
  xm: { type: 'music', actions: ['musicUrl'], qualitys: ['128k'] },
} })
`

func TestRuntimeResolvesThroughRequest(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.Header.Get("X-Test") != "1" || r.URL.Query().Get("id") != "123" || r.URL.Query().Get("q") != "320k" {
			http.Error(w, "bad request "+r.URL.RawQuery, http.StatusBadRequest)
			return
		}
		w.Header().Set("Set-Cookie", "a=1")
		w.Header().Add("Set-Cookie", "b=2")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"url":"https://cdn.example.com/%s.mp3"}`, r.URL.Query().Get("id"))
	}))
	defer srv.Close()

	r, platforms, err := startTest(t, `
const { EVENT_NAMES, request, on, send } = window.lx
on(EVENT_NAMES.request, ({ source, action, info }) => new Promise((resolve, reject) => {
  if (source !== 'kw' || action !== 'musicUrl') return reject(new Error('bad event'))
  request('`+srv.URL+`/api', { method: 'get', headers: { 'X-Test': 1 }, body: { id: info.musicInfo.songmid, q: info.type } }, (err, resp, body) => {
    if (err) return reject(err)
    if (resp.statusCode !== 200 || resp.headers['content-type'] !== 'application/json' || resp.headers['set-cookie'].length !== 2) return reject(new Error('bad response'))
    if (!(resp.raw instanceof Uint8Array) || resp.bytes !== resp.raw.length) return reject(new Error('bad raw'))
    resolve(body.url)
  })
}))
`+initKw)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(platforms); got != "map[kw:[128k 320k flac]]" {
		t.Fatalf("platforms %s", got)
	}
	link, err := r.call(context.Background(), "kw", "320k", map[string]any{"songmid": "123"})
	if err != nil {
		t.Fatal(err)
	}
	if link != "https://cdn.example.com/123.mp3" || hits.Load() != 1 {
		t.Fatalf("link %q hits %d", link, hits.Load())
	}
}

func TestRuntimeUtils(t *testing.T) {
	key := []byte("0123456789abcdef")
	block, _ := aes.NewCipher(key)
	want := make([]byte, 16)
	block.Encrypt(want, append([]byte("hello"), 11, 11, 11, 11, 11, 11, 11, 11, 11, 11, 11))
	r, _, err := startTest(t, `
const { utils } = lx
const check = (name, got, want) => { if (got !== want) throw new Error(name + ': ' + got + ' != ' + want) }
check('md5', utils.crypto.md5('abc'), '900150983cd24fb0d6963f7d28e17f72')
check('md5 bytes', utils.crypto.md5(utils.buffer.from('abc')), '900150983cd24fb0d6963f7d28e17f72')
check('hex', utils.buffer.bufToString(utils.buffer.from('中文'), 'hex'), 'e4b8ade69687')
check('base64', utils.buffer.bufToString(utils.buffer.from('aGk=', 'base64'), 'utf8'), 'hi')
check('binary string', utils.buffer.bufToString('\xff\x01', 'hex'), 'ff01')
check('aes ecb', utils.buffer.bufToString(utils.crypto.aesEncrypt(utils.buffer.from('hello'), 'aes-128-ecb', '0123456789abcdef', ''), 'hex'), '`+hex.EncodeToString(want)+`')
check('random', utils.crypto.randomBytes(16).length, 16)
check('atob', atob(btoa('ab\xff')), 'ab\xff')
check('text', new TextDecoder().decode(new TextEncoder().encode('雨')), '雨')
check('gbk', new TextDecoder('gbk').decode(new Uint8Array([0xd3, 0xea])), '雨')
check('search params', new URLSearchParams({ a: '1 2', b: '雨' }).toString(), 'a=1+2&b=%E9%9B%A8')
check('frozen', Object.isFrozen(lx) && Object.isFrozen(lx.utils.crypto), true)
check('env', lx.env + lx.version + lx.currentScriptInfo.name, 'desktop2.0.0Test source')
utils.zlib.deflate('zipped').then(utils.zlib.inflate).then((out) => {
  check('zlib', utils.buffer.bufToString(out, 'utf8'), 'zipped')
  setTimeout((a, b) => {
    check('timer args', a + b, 3)
    lx.on('request', () => Promise.resolve('https://example.com/ok'))
    lx.send('inited', { sources: { tx: { type: 'music', actions: ['musicUrl'], qualitys: ['flac24bit'] } } })
  }, 10, 1, 2)
})
`)
	if err != nil {
		t.Fatal(err)
	}
	if link, err := r.call(context.Background(), "tx", "flac24bit", map[string]any{}); err != nil || link != "https://example.com/ok" {
		t.Fatalf("link %q err %v", link, err)
	}
}

func TestRuntimeInitFailures(t *testing.T) {
	shortLimits(t)
	cases := []struct {
		name, script, want string
	}{
		{"throws", `throw new Error('broken script')`, "broken script"},
		{"syntax", `const = 1`, "SyntaxError"},
		{"rejection", `Promise.reject(new Error('async failure'))`, "async failure"},
		{"callback throws", `setTimeout(() => { throw new Error('late failure') }, 1)`, "late failure"},
		{"no platforms", `lx.send('inited', { sources: { kw: { type: 'music', actions: ['musicUrl'], qualitys: ['bogus'] } } })`, "none of the platforms"},
		{"no info", `lx.send('inited')`, "Missing required parameter"},
		{"never inited", `lx.on('request', () => Promise.resolve('x'))`, "did not finish initialising"},
		{"endless loop", `for (;;) {}`, "ran for too long"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := startTest(t, c.script)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err %v, want %q", err, c.want)
			}
			if !errors.Is(err, ErrScript) {
				t.Fatalf("err %v is not ErrScript", err)
			}
		})
	}
}

func TestRuntimeCallFailures(t *testing.T) {
	shortLimits(t)
	r, _, err := startTest(t, `
lx.on(lx.EVENT_NAMES.request, ({ info }) => {
  switch (info.musicInfo.mode) {
    case 'reject': return Promise.reject(new Error('服务器繁忙'))
    case 'string': return Promise.reject('plain reason')
    case 'bad': return Promise.resolve('ftp://example.com/x')
    case 'sync': return 'not a promise'
    case 'hang': return new Promise(() => {})
    case 'spin': for (;;) {}
  }
})
lx.send(lx.EVENT_NAMES.inited, { sources: { wy: { type: 'music', actions: ['musicUrl'], qualitys: ['128k'] } } })
`)
	if err != nil {
		t.Fatal(err)
	}
	for mode, want := range map[string]string{
		"reject": "服务器繁忙",
		"string": "plain reason",
		"bad":    "no valid link",
		"sync":   "did not return a Promise",
		"hang":   "no answer within",
	} {
		_, err := r.call(context.Background(), "wy", "128k", map[string]any{"mode": mode})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err %v, want %q", mode, err, want)
		}
	}
	if !r.alive() {
		t.Fatal("runtime stopped after recoverable failures")
	}
	if _, err := r.call(context.Background(), "wy", "128k", map[string]any{"mode": "spin"}); err == nil || !strings.Contains(err.Error(), "too long") {
		t.Fatalf("spin: %v", err)
	}
	if r.alive() {
		t.Fatal("runtime survived an endless loop")
	}
	if _, err := r.call(context.Background(), "wy", "128k", map[string]any{"mode": "reject"}); err == nil {
		t.Fatal("stopped runtime answered")
	}
}

func TestRuntimeRequestsStayPublic(t *testing.T) {
	shortLimits(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("the script reached a loopback server")
	}))
	defer srv.Close()
	guarded, _ := newClients(false)
	r, _, err := startRuntime(context.Background(), runtimeConfig{
		id: "test", info: ScriptInfo{Name: "t"}, client: guarded, script: header + `
const get = (url) => new Promise((resolve) => lx.request(url, {}, (err, resp) => resolve(err ? err.message : 'status ' + resp.statusCode)))
lx.on('request', ({ info }) => get(info.musicInfo.url).then((m) => Promise.reject(new Error(m))))
lx.send('inited', { sources: { kw: { type: 'music', actions: ['musicUrl'], qualitys: ['128k'] } } })
`})
	if err != nil {
		t.Fatal(err)
	}
	defer r.close(nil)
	metadata := "http://" + netip.AddrFrom4([4]byte{169, 254, 169, 254}).String() + "/latest" // cloud metadata service
	private := "http://" + netip.AddrFrom4([4]byte{10, 0, 0, 1}).String() + "/"
	for _, u := range []string{srv.URL, "http://localhost:1/", "http://[::1]:1/", metadata, private, "file:///etc/passwd"} {
		_, err := r.call(context.Background(), "kw", "128k", map[string]any{"url": u})
		if err == nil || !strings.Contains(err.Error(), "not allowed") {
			t.Errorf("%s: %v", u, err)
		}
	}
}

// lx.request behaves like lx-music's (V8) version, which some scripts probe: missing options
// throw V8's TypeError at once, a missing callback only matters when the response arrives.
func TestRuntimeRequestArguments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) }))
	defer srv.Close()
	r, _, err := startTest(t, `
let probe = ''
try { lx.request() } catch (e) { probe = e.constructor.name + ': ' + e.message }
lx.request('`+srv.URL+`', {}) // fire and forget: its response arrives after init and is ignored
lx.on('request', () => Promise.resolve('https://example.com/?' + encodeURIComponent(probe)))
lx.send('inited', { sources: { kw: { type: 'music', actions: ['musicUrl'], qualitys: ['128k'] } } })
`)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // let the callback-less response arrive
	link, err := r.call(context.Background(), "kw", "128k", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := url.PathUnescape(strings.TrimPrefix(link, "https://example.com/?"))
	if want := "TypeError: Cannot read properties of undefined (reading 'method')"; got != want {
		t.Fatalf("probe %q, want %q", got, want)
	}
	if !r.alive() {
		t.Fatal("a response without callback stopped the runtime")
	}
}

func TestRuntimeDoesNotFollowRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/moved" {
			http.Redirect(w, r, "/target", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("target"))
	}))
	defer srv.Close()
	r, _, err := startTest(t, `
lx.on('request', () => new Promise((resolve, reject) => lx.request('`+srv.URL+`/moved', { method: 'get' }, (err, resp) => {
  if (err) return reject(err)
  resolve('https://example.com/' + resp.statusCode + '?to=' + encodeURIComponent(resp.headers.location))
})))
lx.send('inited', { sources: { mg: { type: 'music', actions: ['musicUrl'], qualitys: ['320k'] } } })
`)
	if err != nil {
		t.Fatal(err)
	}
	link, err := r.call(context.Background(), "mg", "320k", nil)
	if err != nil || link != "https://example.com/302?to=%2Ftarget" {
		t.Fatalf("link %q err %v", link, err)
	}
}

func TestRuntimeUpdateAlert(t *testing.T) {
	got := make(chan UpdateAlert, 1)
	client, _ := newClients(true)
	r, _, err := startRuntime(context.Background(), runtimeConfig{
		id: "test", info: ScriptInfo{Name: "t"}, client: client, onAlert: func(a UpdateAlert) { got <- a },
		script: header + `
lx.send(lx.EVENT_NAMES.updateAlert, { log: 'v2 is out', updateUrl: 'javascript:alert(1)' })
lx.send(lx.EVENT_NAMES.updateAlert, { log: 'again' }).catch(() => {})
lx.send(lx.EVENT_NAMES.inited, { sources: { kg: { type: 'music', actions: ['musicUrl'], qualitys: ['128k'] } } })
`})
	if err != nil {
		t.Fatal(err)
	}
	defer r.close(nil)
	select {
	case a := <-got:
		if a.Log != "v2 is out" || a.URL != "" {
			t.Fatalf("alert %+v", a)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no update alert")
	}
}
