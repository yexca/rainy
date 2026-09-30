// Environment of an lx-music custom source script inside Rainy (see runtime.go).
//
// Builds window.lx (the lx-music desktop script API, version 2.0.0) and the small set of
// browser globals scripts rely on (timers, console, atob/btoa, TextEncoder/TextDecoder,
// URLSearchParams) on top of the native functions in natives.go. Evaluates to a setup
// function; the returned dispatch function delivers request events to the script.
(function (native, scriptInfo) {
  'use strict'
  const G = globalThis

  // ---- bytes (Buffers are plain Uint8Arrays, as they reach scripts through Electron's bridge)

  const isBinary = (v) => v instanceof ArrayBuffer || ArrayBuffer.isView(v)
  const toBytes = (v, what) => {
    if (v instanceof Uint8Array) return v
    if (v instanceof ArrayBuffer) return new Uint8Array(v)
    if (ArrayBuffer.isView(v)) return new Uint8Array(v.buffer, v.byteOffset, v.byteLength)
    if (Array.isArray(v)) return Uint8Array.from(v, (x) => Number(x) & 255)
    if (v && typeof v === 'object' && v.type === 'Buffer' && Array.isArray(v.data)) return Uint8Array.from(v.data, (x) => Number(x) & 255)
    throw new TypeError(`${what} must be a string, Buffer, Uint8Array or ArrayBuffer`)
  }
  // A copy of the bytes as an ArrayBuffer (what the natives accept).
  const ab = (u8) => u8.buffer.slice(u8.byteOffset, u8.byteOffset + u8.byteLength)
  const wrap = (buffer) => new Uint8Array(buffer)
  const bytesOf = (v, what, enc) => (typeof v === 'string' ? wrap(native.encode(v, enc || 'utf8')) : toBytes(v, what))

  const bufferFrom = (value, encodingOrOffset, length) => {
    if (typeof value === 'string') return wrap(native.encode(value, encodingOrOffset || 'utf8'))
    if (value instanceof ArrayBuffer) {
      const offset = encodingOrOffset === undefined ? 0 : Number(encodingOrOffset) >>> 0
      const end = length === undefined ? value.byteLength : offset + (Number(length) >>> 0)
      return new Uint8Array(value.slice(offset, end))
    }
    return new Uint8Array(toBytes(value, 'value'))
  }
  const bufToString = (buf, format) => {
    const bytes = typeof buf === 'string' ? wrap(native.encode(buf, 'binary')) : toBytes(buf, 'buffer')
    return native.decode(ab(bytes), format === undefined ? 'utf8' : String(format))
  }

  // ---- lx.request (lx-music uses needle: no redirects are followed, JSON bodies are parsed)

  const enc = encodeURIComponent
  const stringify = (obj, prefix) => {
    const out = []
    for (const key of Object.keys(obj)) {
      const value = obj[key]
      if (value === undefined || typeof value === 'function') continue
      const name = prefix ? `${prefix}[${key}]` : key
      if (Array.isArray(value)) {
        for (const item of value) out.push(`${enc(name + '[]')}=${enc(item == null ? '' : String(item))}`)
      } else if (value !== null && typeof value === 'object' && !isBinary(value)) {
        const nested = stringify(value, name)
        if (nested) out.push(nested)
      } else {
        out.push(`${enc(name)}=${enc(value == null ? '' : String(value))}`)
      }
    }
    return out.join('&')
  }
  const multipart = (fields) => {
    const boundary = '--------------------------' + native.decode(native.randomBytes(12), 'hex')
    const parts = []
    for (const key of Object.keys(fields)) {
      let value = fields[key]
      if (value === undefined) continue
      let head = `--${boundary}\r\nContent-Disposition: form-data; name="${key}"`
      let data
      if (value && typeof value === 'object' && !isBinary(value) && (value.buffer !== undefined || value.value !== undefined)) {
        const content = value.buffer !== undefined ? value.buffer : value.value
        head += `; filename="${value.filename || 'file'}"\r\nContent-Type: ${value.content_type || 'application/octet-stream'}`
        data = bytesOf(content, 'form field')
      } else if (isBinary(value)) {
        head += `; filename="file"\r\nContent-Type: application/octet-stream`
        data = toBytes(value, 'form field')
      } else {
        data = bytesOf(String(value), 'form field')
      }
      parts.push(bytesOf(head + '\r\n\r\n', 'part'), data, bytesOf('\r\n', 'part'))
    }
    parts.push(bytesOf(`--${boundary}--\r\n`, 'part'))
    let size = 0
    for (const p of parts) size += p.length
    const out = new Uint8Array(size)
    let offset = 0
    for (const p of parts) {
      out.set(p, offset)
      offset += p.length
    }
    return { type: `multipart/form-data; boundary=${boundary}`, bytes: out }
  }

  // Behaves like lx-music's preload (running on V8): a missing options object throws V8's
  // destructuring TypeError, and a missing callback only fails when the response arrives.
  // Some scripts probe this and compare the error message.
  const request = function (url, options, callback) {
    if (options == null) throw new TypeError(`Cannot read properties of ${options} (reading 'method')`)
    const { method = 'get', timeout, headers, body, form, formData } = options
    const hdrs = {}
    if (headers && typeof headers === 'object') {
      for (const key of Object.keys(headers)) {
        const value = headers[key]
        if (value != null) hdrs[key] = Array.isArray(value) ? value.map(String).join(', ') : String(value)
      }
    }
    const typeKey = Object.keys(hdrs).find((k) => k.toLowerCase() === 'content-type')
    const setType = (type) => {
      if (!typeKey) hdrs['content-type'] = type
    }
    const verb = String(method).toUpperCase()
    let target = String(url)
    let data = null
    const payload = body != null ? body : form != null ? form : formData != null ? formData : null
    if (payload != null) {
      if (verb === 'GET' || verb === 'HEAD') {
        const query = typeof payload === 'string' ? payload : isBinary(payload) ? '' : stringify(payload)
        if (query) target += (target.includes('?') ? '&' : '?') + query
      } else if (body == null && form == null && typeof formData === 'object') {
        const mp = multipart(formData)
        data = mp.bytes
        setType(mp.type)
      } else if (typeof payload === 'string') {
        data = bytesOf(payload, 'body')
        if (body == null) setType('application/x-www-form-urlencoded')
      } else if (isBinary(payload)) {
        data = toBytes(payload, 'body')
      } else if (body != null && typeKey && /json/i.test(hdrs[typeKey])) {
        data = bytesOf(JSON.stringify(payload), 'body')
      } else {
        data = bytesOf(stringify(payload), 'body')
        setType('application/x-www-form-urlencoded')
      }
    }
    const self = this
    const limit = typeof timeout === 'number' && timeout > 0 ? Math.min(timeout, 60000) : 60000
    const id = native.request(target, verb, JSON.stringify(hdrs), data ? ab(data) : null, limit, (error, status, statusText, headersJSON, raw) => {
      if (error != null) {
        callback.call(self, new Error(error), null, null)
        return
      }
      const rawBytes = wrap(raw)
      const text = native.decode(raw, 'utf8')
      let parsed = text
      try {
        parsed = JSON.parse(text)
      } catch (_) {
        // not JSON
      }
      const resp = { statusCode: status, statusMessage: statusText, headers: JSON.parse(headersJSON), bytes: rawBytes.length, raw: rawBytes, body: parsed }
      callback.call(self, null, resp, parsed)
    })
    return () => native.cancel(id)
  }

  // ---- lx events

  const EVENT_NAMES = Object.freeze({ request: 'request', inited: 'inited', updateAlert: 'updateAlert' })
  const eventNames = Object.values(EVENT_NAMES)
  const PLATFORMS = ['kw', 'kg', 'tx', 'wy', 'mg']
  let handler = null
  let inited = false
  let alerted = false

  const normalizeSources = (info) => {
    const out = {}
    const sources = info.sources && typeof info.sources === 'object' ? info.sources : {}
    for (const key of PLATFORMS) {
      const source = sources[key]
      if (!source || source.type !== 'music') continue
      const actions = Array.isArray(source.actions) ? source.actions : ['musicUrl']
      if (!actions.includes('musicUrl')) continue
      out[key] = Array.isArray(source.qualitys) ? source.qualitys.filter((q) => typeof q === 'string') : []
    }
    return JSON.stringify(out)
  }

  const send = function (eventName, data) {
    return new Promise((resolve, reject) => {
      if (!eventNames.includes(eventName)) return reject(new Error('The event is not supported: ' + eventName))
      switch (eventName) {
        case EVENT_NAMES.inited:
          if (inited) return reject(new Error('Script is inited'))
          inited = true
          if (!data || typeof data !== 'object') native.inited(null, 'Missing required parameter init info')
          else native.inited(normalizeSources(data), null)
          resolve()
          break
        case EVENT_NAMES.updateAlert:
          if (alerted) return reject(new Error('The update alert can only be called once.'))
          alerted = true
          if (!data || typeof data !== 'object') return reject(new Error('parameter format error.'))
          if (!data.log || typeof data.log !== 'string') return reject(new Error('log is required.'))
          native.updateAlert(data.log, typeof data.updateUrl === 'string' ? data.updateUrl : '')
          resolve()
          break
        default:
          reject(new Error('Unknown event name: ' + eventName))
      }
    })
  }

  const on = function (eventName, fn) {
    if (!eventNames.includes(eventName) || eventName !== EVENT_NAMES.request) {
      return Promise.reject(new Error('The event is not supported: ' + eventName))
    }
    handler = fn
    return Promise.resolve()
  }

  const errorMessage = (e) => {
    try {
      if (e == null) return String(e)
      if (typeof e === 'string') return e
      if (typeof e.message === 'string' && e.message) return e.message
      return String(e)
    } catch (_) {
      return 'failed'
    }
  }

  // ---- window.lx

  const deepFreeze = (o) => {
    for (const key of Object.keys(o)) {
      const v = o[key]
      if (v && typeof v === 'object' && !Object.isFrozen(v)) deepFreeze(v)
    }
    return Object.freeze(o)
  }
  const lx = deepFreeze({
    EVENT_NAMES,
    request,
    send,
    on,
    utils: {
      crypto: {
        aesEncrypt: (buffer, mode, key, iv) =>
          wrap(native.aes(ab(bytesOf(buffer, 'buffer')), String(mode), ab(bytesOf(key, 'key')), iv == null || iv === '' ? null : ab(bytesOf(iv, 'iv')))),
        rsaEncrypt: (buffer, key) => wrap(native.rsa(ab(bytesOf(buffer, 'buffer')), String(key))),
        randomBytes: (size) => wrap(native.randomBytes(size)),
        md5: (str) => native.md5(ab(bytesOf(str, 'data'))),
      },
      buffer: {
        from: bufferFrom,
        bufToString,
      },
      zlib: {
        inflate: (buf) =>
          new Promise((resolve, reject) => {
            try {
              resolve(wrap(native.inflate(ab(bytesOf(buf, 'buffer')))))
            } catch (e) {
              reject(new Error(errorMessage(e)))
            }
          }),
        deflate: (data) =>
          new Promise((resolve, reject) => {
            try {
              resolve(wrap(native.deflate(ab(bytesOf(data, 'data')))))
            } catch (e) {
              reject(new Error(errorMessage(e)))
            }
          }),
      },
    },
    currentScriptInfo: {
      name: scriptInfo.name,
      description: scriptInfo.description,
      version: scriptInfo.version,
      author: scriptInfo.author,
      homepage: scriptInfo.homepage,
      rawScript: scriptInfo.rawScript,
    },
    version: '2.0.0',
    env: 'desktop',
  })

  // ---- browser globals

  const format = (v) => {
    if (typeof v === 'string') return v
    if (v instanceof Error) return v.stack || `${v.name}: ${v.message}`
    try {
      const s = JSON.stringify(v)
      return s === undefined ? String(v) : s
    } catch (_) {
      return String(v)
    }
  }
  const logger = (level) => (...args) => native.log(level, args.map(format).join(' '))
  const noop = () => {}
  const console = {
    log: logger('log'),
    info: logger('info'),
    warn: logger('warn'),
    error: logger('error'),
    debug: logger('debug'),
    trace: logger('debug'),
    dir: logger('log'),
    table: logger('log'),
    assert: (cond, ...args) => {
      if (!cond) logger('error')('Assertion failed', ...args)
    },
    group: noop,
    groupCollapsed: noop,
    groupEnd: noop,
    time: noop,
    timeEnd: noop,
    timeLog: noop,
    count: noop,
  }

  const timer = (repeat) => (fn, delay, ...args) => native.setTimer(fn, Number(delay) || 0, repeat, args)
  const clearTimer = (id) => {
    if (id != null) native.clearTimer(Number(id))
  }

  class TextEncoder {
    get encoding() {
      return 'utf-8'
    }
    encode(input = '') {
      return wrap(native.encode(String(input), 'utf8'))
    }
  }
  class TextDecoder {
    constructor(label = 'utf-8') {
      this._encoding = native.textLabel(String(label))
    }
    get encoding() {
      return this._encoding
    }
    decode(input) {
      if (input === undefined) return ''
      return native.textDecode(ab(toBytes(input, 'input')), this._encoding)
    }
  }

  const formEncode = (s) => enc(s).replace(/[!'()~]/g, (c) => '%' + c.charCodeAt(0).toString(16).toUpperCase()).replace(/%20/g, '+')
  const formDecode = (s) => {
    try {
      return decodeURIComponent(s.replace(/\+/g, ' '))
    } catch (_) {
      return s
    }
  }
  class URLSearchParams {
    constructor(init) {
      this._list = []
      if (init == null) return
      if (typeof init === 'string') {
        for (const pair of init.replace(/^\?/, '').split('&')) {
          if (!pair) continue
          const i = pair.indexOf('=')
          this._list.push(i < 0 ? [formDecode(pair), ''] : [formDecode(pair.slice(0, i)), formDecode(pair.slice(i + 1))])
        }
      } else if (typeof init === 'object' && typeof init[Symbol.iterator] === 'function') {
        for (const pair of init) {
          const [k, v] = pair
          this._list.push([String(k), String(v)])
        }
      } else if (typeof init === 'object') {
        for (const key of Object.keys(init)) this._list.push([key, String(init[key])])
      }
    }
    get size() {
      return this._list.length
    }
    append(name, value) {
      this._list.push([String(name), String(value)])
    }
    delete(name) {
      this._list = this._list.filter(([k]) => k !== String(name))
    }
    get(name) {
      const hit = this._list.find(([k]) => k === String(name))
      return hit ? hit[1] : null
    }
    getAll(name) {
      return this._list.filter(([k]) => k === String(name)).map(([, v]) => v)
    }
    has(name) {
      return this._list.some(([k]) => k === String(name))
    }
    set(name, value) {
      const key = String(name)
      const i = this._list.findIndex(([k]) => k === key)
      if (i < 0) {
        this._list.push([key, String(value)])
        return
      }
      this._list[i] = [key, String(value)]
      this._list = this._list.filter(([k], j) => k !== key || j === i)
    }
    sort() {
      this._list.sort((a, b) => (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0))
    }
    forEach(fn, thisArg) {
      for (const [k, v] of this._list) fn.call(thisArg, v, k, this)
    }
    keys() {
      return this._list.map(([k]) => k)[Symbol.iterator]()
    }
    values() {
      return this._list.map(([, v]) => v)[Symbol.iterator]()
    }
    entries() {
      return this._list.map(([k, v]) => [k, v])[Symbol.iterator]()
    }
    [Symbol.iterator]() {
      return this.entries()
    }
    toString() {
      return this._list.map(([k, v]) => `${formEncode(k)}=${formEncode(v)}`).join('&')
    }
  }

  const define = (name, value) => Object.defineProperty(G, name, { value, writable: true, configurable: true, enumerable: false })
  define('window', G)
  define('self', G)
  define('console', console)
  define('setTimeout', timer(false))
  define('setInterval', timer(true))
  define('clearTimeout', clearTimer)
  define('clearInterval', clearTimer)
  define('queueMicrotask', (fn) => {
    Promise.resolve().then(fn)
  })
  define('atob', (s) => native.atob(String(s)))
  define('btoa', (s) => native.btoa(String(s)))
  define('TextEncoder', TextEncoder)
  define('TextDecoder', TextDecoder)
  define('URLSearchParams', URLSearchParams)
  define('navigator', Object.freeze({ userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) lx-music-desktop/2.12.6 Chrome/124.0 Electron/30.0 Safari/537.36', language: 'zh-CN', languages: Object.freeze(['zh-CN', 'zh']), platform: 'Win32', onLine: true }))
  Object.defineProperty(G, 'lx', { value: lx, writable: false, configurable: false, enumerable: true })

  // ---- request events from Rainy

  const dispatch = (id, json) => {
    if (typeof handler !== 'function') {
      native.done(id, false, 'Request event is not defined')
      return
    }
    try {
      const pending = handler.call(lx, JSON.parse(json))
      if (!pending || typeof pending.then !== 'function') {
        native.done(id, false, 'the request handler did not return a Promise')
        return
      }
      pending.then(
        (value) => native.done(id, true, value),
        (error) => native.done(id, false, errorMessage(error)),
      )
    } catch (e) {
      native.done(id, false, errorMessage(e))
    }
  }

  return { dispatch }
})
