package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

func AdminDist() fs.FS {
	return dist()
}

func DefaultThemeDist() fs.FS {
	return dist()
}

func Dist() fs.FS {
	return AdminDist()
}

func dist() fs.FS {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil
	}
	return dist
}
