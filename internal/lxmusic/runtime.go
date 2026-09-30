package lxmusic

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/dop251/goja"
)

// Time limits of a running source script (variables for tests).
var (
	initTimeout = 20 * time.Second // until the script sends "inited"
	callTimeout = 20 * time.Second // one request event (lx-music uses 20 s too)
	taskTimeout = 10 * time.Second // one uninterrupted run of script code
)

// Other limits of a running source script.
const (
	maxTimers      = 1000
	maxQueued      = 10_000
	maxInflight    = 8 // concurrent lx.request calls
	maxHTTPTimeout = 60 * time.Second
	maxResponse    = 10 << 20
	maxRequestBody = 4 << 20
	maxErrorText   = 1024
	maxAlertLog    = 1024
	maxURLLength   = 2048
	logBurst       = 60 // console lines logged per minute
)

//go:embed prelude.js
var preludeSource string

var preludeProgram = goja.MustCompile("prelude.js", preludeSource, true)

var errTaskTimeout = errors.New("the source script ran for too long")

// UpdateAlert is a script's "new version available" notice (lx.send('updateAlert')).
type UpdateAlert struct {
	Log string `json:"log"`
	URL string `json:"url"`
	At  int64  `json:"at"`
}

// runtimeConfig configures one script instance.
type runtimeConfig struct {
	id      string // source id (logs)
	info    ScriptInfo
	script  string
	client  *http.Client
	onAlert func(UpdateAlert)
}

type callResult struct {
	value string
	err   error
}

type jsTimer struct {
	t        *time.Timer
	fn       goja.Callable
	args     []goja.Value
	interval time.Duration
}

// runtime is one running source script. The goja interpreter is used by the loop goroutine
// only; other goroutines hand it work through post.
type runtime struct {
	cfg    runtimeConfig
	vm     *goja.Runtime
	log    *slog.Logger
	ctx    context.Context // cancels HTTP requests when the runtime closes
	cancel context.CancelFunc

	qmu     sync.Mutex
	queue   []func()
	stopped bool
	failure error
	wake    chan struct{}
	done    chan struct{}

	initCh chan initResult // receives the outcome of initialisation once

	pmu     sync.Mutex
	pending map[int64]chan callResult
	seq     atomic.Int64

	inflight chan struct{}

	imu     sync.Mutex // guards curTask against the watchdog
	curTask int64
	taskSeq atomic.Int64

	logMu     sync.Mutex
	logWindow time.Time
	logCount  int

	// loop goroutine only
	dispatch  goja.Callable
	inited    bool
	timers    map[int64]*jsTimer
	timerSeq  int64
	requests  map[int64]context.CancelFunc
	unhandled map[*goja.Promise]string
	initSent  bool
}

type initResult struct {
	platforms map[string][]string
	err       error
}

// startRuntime runs the script and waits until it has initialised (sent "inited"). It
// returns the qualities the script provides per platform.
func startRuntime(ctx context.Context, cfg runtimeConfig) (*runtime, map[string][]string, error) {
	rctx, cancel := context.WithCancel(context.Background())
	r := &runtime{
		cfg:       cfg,
		vm:        goja.New(),
		log:       slog.With("source", cfg.info.Name, "sourceId", cfg.id),
		ctx:       rctx,
		cancel:    cancel,
		wake:      make(chan struct{}, 1),
		done:      make(chan struct{}),
		initCh:    make(chan initResult, 1),
		pending:   map[int64]chan callResult{},
		inflight:  make(chan struct{}, maxInflight),
		timers:    map[int64]*jsTimer{},
		requests:  map[int64]context.CancelFunc{},
		unhandled: map[*goja.Promise]string{},
	}
	go r.loop()
	r.post(r.boot)

	timer := time.NewTimer(initTimeout)
	defer timer.Stop()
	select {
	case res := <-r.initCh:
		if res.err != nil {
			r.close(res.err)
			return nil, nil, res.err
		}
		return r, res.platforms, nil
	case <-r.done:
		return nil, nil, r.err()
	case <-timer.C:
		err := fmt.Errorf("%w: the script did not finish initialising within %s", ErrScript, initTimeout)
		r.close(err)
		return nil, nil, err
	case <-ctx.Done():
		r.close(ctx.Err())
		return nil, nil, ctx.Err()
	}
}

// ---- event loop

func (r *runtime) loop() {
	defer close(r.done)
	for {
		r.qmu.Lock()
		for len(r.queue) == 0 && !r.stopped {
			r.qmu.Unlock()
			<-r.wake
			r.qmu.Lock()
		}
		if r.stopped {
			r.qmu.Unlock()
			return
		}
		task := r.queue[0]
		r.queue[0] = nil
		r.queue = r.queue[1:]
		r.qmu.Unlock()
		r.run(task)
	}
}

// post queues fn for the loop goroutine. It reports false when the runtime has stopped.
func (r *runtime) post(fn func()) bool {
	r.qmu.Lock()
	if r.stopped {
		r.qmu.Unlock()
		return false
	}
	if len(r.queue) >= maxQueued {
		r.qmu.Unlock()
		r.close(fmt.Errorf("%w: the script queued too much work", ErrScript))
		return false
	}
	r.queue = append(r.queue, fn)
	r.qmu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default:
	}
	return true
}

// run executes one task with the time limit. A script that overruns is stopped for good.
// The watchdog only interrupts the task it was armed for, and the interrupt flag is cleared
// after every task, so a late watchdog cannot hit the next one.
func (r *runtime) run(task func()) {
	tok := r.taskSeq.Add(1)
	r.imu.Lock()
	r.curTask = tok
	r.imu.Unlock()
	watchdog := time.AfterFunc(taskTimeout, func() {
		r.imu.Lock()
		defer r.imu.Unlock()
		if r.curTask == tok {
			r.vm.Interrupt(errTaskTimeout)
		}
	})
	defer func() {
		watchdog.Stop()
		r.imu.Lock()
		r.curTask = 0
		r.vm.ClearInterrupt()
		r.imu.Unlock()
		if p := recover(); p != nil {
			r.close(fmt.Errorf("%w: %v", ErrScript, p))
		}
	}()
	task()
	r.checkRejections()
}

// close stops the runtime (idempotent): pending calls fail with err, timers and requests
// are cancelled.
func (r *runtime) close(err error) {
	if err == nil {
		err = ErrClosed
	}
	r.qmu.Lock()
	if r.stopped {
		r.qmu.Unlock()
		return
	}
	r.stopped = true
	r.failure = err
	r.queue = nil
	r.qmu.Unlock()
	r.cancel()
	r.vm.Interrupt(ErrClosed)
	select {
	case r.wake <- struct{}{}:
	default:
	}
	r.pmu.Lock()
	for id, ch := range r.pending {
		ch <- callResult{err: err}
		delete(r.pending, id)
	}
	r.pmu.Unlock()
	select {
	case r.initCh <- initResult{err: err}:
	default:
	}
	go func() {
		<-r.done
		// Timers belong to the loop goroutine, which has exited.
		for _, t := range r.timers {
			t.t.Stop()
		}
	}()
}

// alive reports whether the runtime still runs.
func (r *runtime) alive() bool {
	r.qmu.Lock()
	defer r.qmu.Unlock()
	return !r.stopped
}

func (r *runtime) err() error {
	r.qmu.Lock()
	defer r.qmu.Unlock()
	if r.failure == nil {
		return ErrClosed
	}
	return r.failure
}

// ---- boot

func (r *runtime) boot() {
	vm := r.vm
	vm.SetPromiseRejectionTracker(r.trackRejection)
	fn, err := vm.RunProgram(preludeProgram)
	if err != nil {
		r.close(fmt.Errorf("%w: preparing the script environment: %v", ErrScript, err))
		return
	}
	setup, ok := goja.AssertFunction(fn)
	if !ok {
		r.close(fmt.Errorf("%w: preparing the script environment", ErrScript))
		return
	}
	info := vm.NewObject()
	_ = info.Set("name", r.cfg.info.Name)
	_ = info.Set("description", r.cfg.info.Description)
	_ = info.Set("version", r.cfg.info.Version)
	_ = info.Set("author", r.cfg.info.Author)
	_ = info.Set("homepage", r.cfg.info.Homepage)
	_ = info.Set("rawScript", r.cfg.script)
	api, err := setup(goja.Undefined(), r.natives(), info)
	if err != nil {
		r.close(fmt.Errorf("%w: preparing the script environment: %v", ErrScript, err))
		return
	}
	dispatch, ok := goja.AssertFunction(api.ToObject(vm).Get("dispatch"))
	if !ok {
		r.close(fmt.Errorf("%w: preparing the script environment", ErrScript))
		return
	}
	r.dispatch = dispatch

	if _, err := vm.RunScript("source.js", r.cfg.script); err != nil {
		r.uncaught(err)
	}
}

// trackRejection records promises rejected without a handler (lx-music treats one before
// "inited" as a failed initialisation).
func (r *runtime) trackRejection(p *goja.Promise, op goja.PromiseRejectionOperation) {
	switch op {
	case goja.PromiseRejectionReject:
		r.unhandled[p] = valueMessage(p.Result())
	case goja.PromiseRejectionHandle:
		delete(r.unhandled, p)
	}
}

func (r *runtime) checkRejections() {
	if len(r.unhandled) == 0 {
		return
	}
	for p, msg := range r.unhandled {
		delete(r.unhandled, p)
		r.scriptError(msg)
	}
}

// uncaught handles an exception that escaped script code.
func (r *runtime) uncaught(err error) {
	var ie *goja.InterruptedError
	if errors.As(err, &ie) {
		cause := errTaskTimeout
		if v, ok := ie.Value().(error); ok {
			cause = v
		}
		r.close(fmt.Errorf("%w: %v", ErrScript, cause))
		return
	}
	var ex *goja.Exception
	if errors.As(err, &ex) {
		r.scriptError(valueMessage(ex.Value()))
		return
	}
	r.scriptError(err.Error())
}

// scriptError reports an uncaught error: before "inited" it fails the initialisation
// (like lx-music), afterwards it is only logged.
func (r *runtime) scriptError(msg string) {
	msg = truncate(strings.TrimPrefix(strings.TrimPrefix(msg, "Uncaught "), "Error: "), maxErrorText)
	if !r.inited {
		r.failInit(fmt.Errorf("%w: %s", ErrScript, msg))
		return
	}
	r.log.Debug("lx source: uncaught error", "err", msg)
}

func (r *runtime) failInit(err error) {
	if r.initSent {
		return
	}
	r.initSent = true
	r.inited = true
	select {
	case r.initCh <- initResult{err: err}:
	default:
	}
}

// ---- requests into the script

// call sends a request event (action musicUrl) and waits for the link.
func (r *runtime) call(ctx context.Context, platform, quality string, musicInfo map[string]any) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"source": platform,
		"action": "musicUrl",
		"info":   map[string]any{"type": quality, "musicInfo": musicInfo},
	})
	if err != nil {
		return "", err
	}
	id := r.seq.Add(1)
	ch := make(chan callResult, 1)
	r.pmu.Lock()
	r.pending[id] = ch
	r.pmu.Unlock()
	defer func() {
		r.pmu.Lock()
		delete(r.pending, id)
		r.pmu.Unlock()
	}()
	if !r.post(func() {
		if _, err := r.dispatch(goja.Undefined(), r.vm.ToValue(float64(id)), r.vm.ToValue(string(payload))); err != nil {
			r.uncaught(err)
			r.finish(id, callResult{err: fmt.Errorf("%w: %s", ErrScript, errorText(err))})
		}
	}) {
		return "", r.err()
	}
	timer := time.NewTimer(callTimeout)
	defer timer.Stop()
	select {
	case res := <-ch:
		return res.value, res.err
	case <-timer.C:
		return "", fmt.Errorf("%w: no answer within %s", ErrScript, callTimeout)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (r *runtime) finish(id int64, res callResult) {
	r.pmu.Lock()
	ch, ok := r.pending[id]
	delete(r.pending, id)
	r.pmu.Unlock()
	if ok {
		ch <- res
	}
}

// ---- helpers

// valueMessage turns a thrown JavaScript value into a message.
func valueMessage(v goja.Value) (msg string) {
	defer func() {
		if recover() != nil {
			msg = "unknown error"
		}
	}()
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return fmt.Sprint(v)
	}
	if o, ok := v.(*goja.Object); ok {
		if m := o.Get("message"); m != nil && !goja.IsUndefined(m) && !goja.IsNull(m) {
			return m.String()
		}
	}
	return v.String()
}

func errorText(err error) string {
	var ex *goja.Exception
	if errors.As(err, &ex) {
		return truncate(valueMessage(ex.Value()), maxErrorText)
	}
	return truncate(err.Error(), maxErrorText)
}

// truncate shortens s to at most n bytes (on a rune boundary) and appends "...".
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}
