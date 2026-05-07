//
// Copyright (c) 2026 Markku Rossi
//
// All rights reserved.
//

package kernel

import (
	"path/filepath"
)

type Params struct {
	Verbose bool
	Ktrace  bool
	Profile bool
	FSRoot  string
}

func (params Params) MakePath(pathname string) string {
	return filepath.Join(params.FSRoot, pathname)
}
