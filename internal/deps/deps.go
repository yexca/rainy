// Package deps pins third-party modules that later-wave packages rely on but that the
// foundation does not import yet, so `go mod tidy` keeps them in go.mod. Wave-2 agents are
// not allowed to run `go get`; every dependency they may need must be listed here or be
// imported elsewhere. Remove an import once a real package uses it.
package deps

import (
	_ "go.senan.xyz/taglib"
	_ "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	_ "golang.org/x/sync/errgroup"
	_ "golang.org/x/sync/singleflight"
	_ "golang.org/x/text/encoding"
	_ "golang.org/x/text/encoding/japanese"
	_ "golang.org/x/text/encoding/simplifiedchinese"
	_ "golang.org/x/text/encoding/traditionalchinese"
	_ "golang.org/x/text/encoding/unicode"
	_ "golang.org/x/text/transform"
	_ "golang.org/x/text/unicode/norm"
)
