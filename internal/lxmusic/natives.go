package lxmusic

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"

	"rainy/internal/util"
)

// Native functions handed to prelude.js, which builds the lx object and the browser
// globals on top of them. They run on the loop goroutine; errors are thrown into the script.

// Request defaults, modelled on the needle client lx-music uses.
const (
	defaultUserAgent = "Needle/3.3.1 (Node.js v20.18.0; win32 x64)"
	maxRandomBytes   = 64 << 10
)

var requestMethods = []string{"GET", "POST", "PUT", "DELETE", "HEAD", "PATCH", "OPTIONS"}

func (r *runtime) natives() *goja.Object {
	o := r.vm.NewObject()
	fns := map[string]func(goja.FunctionCall) goja.Value{
		"encode":      r.jsEncode,
		"decode":      r.jsDecode,
		"textDecode":  r.jsTextDecode,
		"textLabel":   r.jsTextLabel,
		"btoa":        r.jsBtoa,
		"atob":        r.jsAtob,
		"aes":         r.jsAES,
		"rsa":         r.jsRSA,
		"md5":         r.jsMD5,
		"randomBytes": r.jsRandomBytes,
		"inflate":     r.jsInflate,
		"deflate":     r.jsDeflate,
		"request":     r.jsRequest,
		"cancel":      r.jsCancel,
		"setTimer":    r.jsSetTimer,
		"clearTimer":  r.jsClearTimer,
		"log":         r.jsLog,
		"inited":      r.jsInited,
		"updateAlert": r.jsUpdateAlert,
		"done":        r.jsDone,
	}
	for name, fn := range fns {
		_ = o.Set(name, fn)
	}
	return o
}

// throw raises a JavaScript Error with msg.
func (r *runtime) throw(msg string) {
	panic(r.vm.NewGoError(errors.New(msg)))
}

// bytesArg reads an ArrayBuffer argument (prelude.js converts everything else). The result
// aliases script memory: copy it before keeping it.
func (r *runtime) bytesArg(v goja.Value) []byte {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil
	}
	if ab, ok := v.Export().(goja.ArrayBuffer); ok {
		return ab.Bytes()
	}
	panic(r.vm.NewTypeError("expected an ArrayBuffer"))
}

func (r *runtime) buffer(b []byte) goja.Value {
	return r.vm.ToValue(r.vm.NewArrayBuffer(b))
}

func (r *runtime) jsEncode(call goja.FunctionCall) goja.Value {
	b, err := encodeString(call.Argument(0).String(), call.Argument(1).String())
	if err != nil {
		panic(r.vm.NewTypeError(err.Error()))
	}
	return r.buffer(b)
}

func (r *runtime) jsDecode(call goja.FunctionCall) goja.Value {
	s, err := decodeBytes(r.bytesArg(call.Argument(0)), call.Argument(1).String())
	if err != nil {
		panic(r.vm.NewTypeError(err.Error()))
	}
	return r.vm.ToValue(s)
}

func (r *runtime) jsTextDecode(call goja.FunctionCall) goja.Value {
	s, err := textDecode(r.bytesArg(call.Argument(0)), call.Argument(1).String())
	if err != nil {
		panic(r.vm.NewTypeError(err.Error()))
	}
	return r.vm.ToValue(s)
}

func (r *runtime) jsTextLabel(call goja.FunctionCall) goja.Value {
	name, err := textEncoding(call.Argument(0).String())
	if err != nil {
		panic(r.vm.NewTypeError(err.Error()))
	}
	return r.vm.ToValue(name)
}

func (r *runtime) jsBtoa(call goja.FunctionCall) goja.Value {
	s, err := btoa(call.Argument(0).String())
	if err != nil {
		r.throw(err.Error())
	}
	return r.vm.ToValue(s)
}

func (r *runtime) jsAtob(call goja.FunctionCall) goja.Value {
	s, err := atob(call.Argument(0).String())
	if err != nil {
		r.throw(err.Error())
	}
	return r.vm.ToValue(s)
}

func (r *runtime) jsAES(call goja.FunctionCall) goja.Value {
	out, err := aesEncrypt(r.bytesArg(call.Argument(0)), call.Argument(1).String(), r.bytesArg(call.Argument(2)), r.bytesArg(call.Argument(3)))
	if err != nil {
		r.throw(err.Error())
	}
	return r.buffer(out)
}

func (r *runtime) jsRSA(call goja.FunctionCall) goja.Value {
	out, err := rsaEncrypt(r.bytesArg(call.Argument(0)), call.Argument(1).String())
	if err != nil {
		r.throw(err.Error())
	}
	return r.buffer(out)
}

func (r *runtime) jsMD5(call goja.FunctionCall) goja.Value {
	return r.vm.ToValue(md5Hex(r.bytesArg(call.Argument(0))))
}

func (r *runtime) jsRandomBytes(call goja.FunctionCall) goja.Value {
	n := call.Argument(0).ToFloat()
	if math.IsNaN(n) || n < 0 || n > maxRandomBytes || n != math.Trunc(n) {
		panic(r.vm.NewTypeError(fmt.Sprintf("size must be an integer between 0 and %d", maxRandomBytes)))
	}
	b := make([]byte, int(n))
	_, _ = rand.Read(b)
	return r.buffer(b)
}

func (r *runtime) jsInflate(call goja.FunctionCall) goja.Value {
	out, err := inflate(r.bytesArg(call.Argument(0)))
	if err != nil {
		r.throw(err.Error())
	}
	return r.buffer(out)
}

func (r *runtime) jsDeflate(call goja.FunctionCall) goja.Value {
	out, err := deflate(r.bytesArg(call.Argument(0)))
	if err != nil {
		r.throw(err.Error())
	}
	return r.buffer(out)
}

// ---- HTTP (lx.request)

type httpResult struct {
	status     int
	statusText string
	headers    string // JSON object, Node-style (lower-case names; set-cookie is an array)
	body       []byte
	err        string
}

// jsRequest starts request(url, method, headersJSON, body, timeoutMs, callback) and
// returns its id for cancel. The callback runs on the loop with (error) or
// (null, status, statusText, headersJSON, body).
func (r *runtime) jsRequest(call goja.FunctionCall) goja.Value {
	rawURL := strings.TrimSpace(call.Argument(0).String())
	method := strings.ToUpper(strings.TrimSpace(call.Argument(1).String()))
	if !slices.Contains(requestMethods, method) {
		panic(r.vm.NewTypeError("unsupported request method " + method))
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(call.Argument(2).String()), &headers); err != nil {
		panic(r.vm.NewTypeError("invalid request headers"))
	}
	body := bytes.Clone(r.bytesArg(call.Argument(3)))
	if len(body) > maxRequestBody {
		r.throw(fmt.Sprintf("the request body is larger than %d bytes", maxRequestBody))
	}
	timeout := maxHTTPTimeout
	if ms := call.Argument(4).ToFloat(); ms > 0 && !math.IsInf(ms, 1) && time.Duration(ms)*time.Millisecond < timeout {
		timeout = time.Duration(ms) * time.Millisecond
	}
	cb, ok := goja.AssertFunction(call.Argument(5))
	if !ok {
		panic(r.vm.NewTypeError("callback must be a function"))
	}

	r.timerSeq++
	id := r.timerSeq
	ctx, cancel := context.WithTimeout(r.ctx, timeout)
	r.requests[id] = cancel
	go func() {
		res := r.doHTTP(ctx, rawURL, method, headers, body)
		r.post(func() {
			if c, ok := r.requests[id]; ok {
				delete(r.requests, id)
				c()
			}
			var err error
			if res.err != "" {
				_, err = cb(goja.Undefined(), r.vm.ToValue(res.err))
			} else {
				_, err = cb(goja.Undefined(), goja.Null(), r.vm.ToValue(res.status), r.vm.ToValue(res.statusText),
					r.vm.ToValue(res.headers), r.buffer(res.body))
			}
			if err != nil {
				r.uncaught(err)
			}
		})
	}()
	return r.vm.ToValue(id)
}

func (r *runtime) jsCancel(call goja.FunctionCall) goja.Value {
	id := call.Argument(0).ToInteger()
	if c, ok := r.requests[id]; ok {
		c()
	}
	return goja.Undefined()
}

func (r *runtime) doHTTP(ctx context.Context, rawURL, method string, headers map[string]string, body []byte) httpResult {
	select {
	case r.inflight <- struct{}{}:
		defer func() { <-r.inflight }()
	case <-ctx.Done():
		return httpResult{err: ctxMessage(ctx)}
	}
	if len(rawURL) > 8192 {
		return httpResult{err: "the URL is too long"}
	}
	u, err := url.Parse(escapeURL(rawURL))
	if err != nil {
		return httpResult{err: "Invalid URL: " + truncate(rawURL, 200)}
	}
	if err := checkURL(u, true); err != nil { // scheme and host; addresses are checked when dialling
		return httpResult{err: err.Error()}
	}
	var rd io.Reader
	if len(body) > 0 {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return httpResult{err: err.Error()}
	}
	for k, v := range headers {
		if strings.EqualFold(k, "host") {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", defaultUserAgent)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "*/*")
	}
	resp, err := r.cfg.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return httpResult{err: ctxMessage(ctx)}
		}
		return httpResult{err: requestError(u.Hostname(), err).Error()}
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		if ctx.Err() != nil {
			return httpResult{err: ctxMessage(ctx)}
		}
		return httpResult{err: fmt.Sprintf("reading the response of %s: %v", u.Hostname(), err)}
	}
	if len(data) > maxResponse {
		return httpResult{err: fmt.Sprintf("the response of %s is larger than %d bytes", u.Hostname(), maxResponse)}
	}
	hdr := map[string]any{}
	for k, v := range resp.Header {
		name := strings.ToLower(k)
		if name == "set-cookie" {
			hdr[name] = v
		} else {
			hdr[name] = strings.Join(v, ", ")
		}
	}
	hj, _ := json.Marshal(hdr)
	return httpResult{
		status:     resp.StatusCode,
		statusText: strings.TrimSpace(strings.TrimPrefix(resp.Status, strconv.Itoa(resp.StatusCode))),
		headers:    string(hj),
		body:       data,
	}
}

func ctxMessage(ctx context.Context) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "timeout"
	}
	return "request aborted"
}

// escapeURL percent-encodes characters that browsers escape in URLs (non-ASCII, spaces,
// quotes, …) so scripts may pass URLs with raw Chinese text.
func escapeURL(s string) string {
	const special = "\"<>\\^`{|} "
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x80 || c < 0x20 || c == 0x7f || strings.IndexByte(special, c) >= 0 {
			fmt.Fprintf(&b, "%%%02X", c)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// ---- timers

func (r *runtime) jsSetTimer(call goja.FunctionCall) goja.Value {
	fn, ok := goja.AssertFunction(call.Argument(0))
	if !ok {
		panic(r.vm.NewTypeError("the timer callback must be a function"))
	}
	if len(r.timers) >= maxTimers {
		r.throw("too many timers")
	}
	ms := call.Argument(1).ToFloat()
	if math.IsNaN(ms) || ms < 0 {
		ms = 0
	}
	d := time.Duration(min(ms, float64(24*time.Hour/time.Millisecond))) * time.Millisecond
	repeat := call.Argument(2).ToBoolean()
	if repeat && d < 10*time.Millisecond {
		d = 10 * time.Millisecond
	}
	var args []goja.Value
	if obj, ok := call.Argument(3).(*goja.Object); ok {
		n := obj.Get("length").ToInteger()
		for i := int64(0); i < n && i < 64; i++ {
			args = append(args, obj.Get(strconv.FormatInt(i, 10)))
		}
	}
	r.timerSeq++
	id := r.timerSeq
	t := &jsTimer{fn: fn, args: args}
	if repeat {
		t.interval = d
	}
	t.t = time.AfterFunc(d, func() { r.post(func() { r.fire(id) }) })
	r.timers[id] = t
	return r.vm.ToValue(id)
}

func (r *runtime) fire(id int64) {
	t, ok := r.timers[id]
	if !ok {
		return
	}
	if t.interval > 0 {
		t.t = time.AfterFunc(t.interval, func() { r.post(func() { r.fire(id) }) })
	} else {
		delete(r.timers, id)
	}
	if _, err := t.fn(goja.Undefined(), t.args...); err != nil {
		r.uncaught(err)
	}
}

func (r *runtime) jsClearTimer(call goja.FunctionCall) goja.Value {
	id := call.Argument(0).ToInteger()
	if t, ok := r.timers[id]; ok {
		t.t.Stop()
		delete(r.timers, id)
	}
	return goja.Undefined()
}

// ---- console

func (r *runtime) jsLog(call goja.FunctionCall) goja.Value {
	r.logMu.Lock()
	now := time.Now()
	if now.Sub(r.logWindow) > time.Minute {
		r.logWindow, r.logCount = now, 0
	}
	r.logCount++
	allowed := r.logCount <= logBurst
	r.logMu.Unlock()
	if allowed {
		r.log.Debug("lx source: console", "level", call.Argument(0).String(), "msg", truncate(call.Argument(1).String(), maxErrorText))
	}
	return goja.Undefined()
}

// ---- lx events

// jsInited receives lx.send('inited', info): the qualities per platform as JSON
// (normalised by prelude.js), or an error message.
func (r *runtime) jsInited(call goja.FunctionCall) goja.Value {
	if r.initSent {
		return goja.Undefined()
	}
	if msg := call.Argument(1); !goja.IsUndefined(msg) && !goja.IsNull(msg) && msg.String() != "" {
		r.failInit(fmt.Errorf("%w: %s", ErrScript, truncate(msg.String(), maxErrorText)))
		return goja.Undefined()
	}
	var raw map[string][]string
	_ = json.Unmarshal([]byte(call.Argument(0).String()), &raw)
	platforms := map[string][]string{}
	for _, p := range Platforms {
		var qs []string
		for _, q := range Qualities {
			if slices.Contains(raw[p], q) {
				qs = append(qs, q)
			}
		}
		if len(qs) > 0 {
			platforms[p] = qs
		}
	}
	if len(platforms) == 0 {
		r.failInit(fmt.Errorf("%w: the script offers links for none of the platforms Rainy supports (kw, kg, tx, wy, mg)", ErrScript))
		return goja.Undefined()
	}
	r.inited = true
	r.initSent = true
	select {
	case r.initCh <- initResult{platforms: platforms}:
	default:
	}
	return goja.Undefined()
}

func (r *runtime) jsUpdateAlert(call goja.FunctionCall) goja.Value {
	alert := UpdateAlert{Log: truncate(call.Argument(0).String(), maxAlertLog), At: util.NowMs()}
	if u := strings.TrimSpace(call.Argument(1).String()); len(u) <= 1024 && validHomepage(u) {
		alert.URL = u
	}
	if r.cfg.onAlert != nil {
		go r.cfg.onAlert(alert)
	}
	return goja.Undefined()
}

// jsDone receives the outcome of a request event: done(id, ok, value).
func (r *runtime) jsDone(call goja.FunctionCall) goja.Value {
	id := call.Argument(0).ToInteger()
	v := call.Argument(2)
	if !call.Argument(1).ToBoolean() {
		msg := strings.TrimPrefix(v.String(), "Error: ")
		if msg == "" {
			msg = "failed"
		}
		r.finish(id, callResult{err: fmt.Errorf("%w: %s", ErrScript, truncate(msg, maxErrorText))})
		return goja.Undefined()
	}
	s, ok := v.Export().(string)
	if !ok || len(s) > maxURLLength || !isHTTPLink(s) {
		r.finish(id, callResult{err: fmt.Errorf("%w: the script returned no valid link", ErrScript)})
		return goja.Undefined()
	}
	r.finish(id, callResult{value: s})
	return goja.Undefined()
}

// isHTTPLink applies lx-music's check for returned links (/^https?:/).
func isHTTPLink(s string) bool {
	return strings.HasPrefix(s, "http:") || strings.HasPrefix(s, "https:")
}
