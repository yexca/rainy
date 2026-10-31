module rainy

go 1.26.0

toolchain go1.26.10

// Navidrome's fork includes non-throwing UTF-8 conversion for the TagLib WASM build.
replace go.senan.xyz/taglib => github.com/deluan/go-taglib v0.0.0-20260913142955-d55e0c9353cb

require (
	github.com/dop251/goja v0.0.0-20260926152631-39ec2650adc9
	github.com/go-chi/chi/v5 v5.5.0
	github.com/jmoiron/sqlx v1.4.0
	github.com/mozillazg/go-pinyin v0.21.0
	go.senan.xyz/taglib v0.14.0
	golang.org/x/image v0.47.0
	golang.org/x/sync v0.24.0
	golang.org/x/text v0.43.0
	modernc.org/sqlite v1.61.0
)

require (
	github.com/dlclark/regexp2/v2 v2.5.2 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/google/pprof v0.0.0-20261016150608-1facf8f91b50 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.1.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/tetratelabs/wazero v1.12.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	modernc.org/libc v1.79.0 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)
