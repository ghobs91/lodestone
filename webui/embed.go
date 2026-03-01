package webui

import "embed"

//go:embed dist/lodestone/browser/*
var staticFS embed.FS

func StaticFS() embed.FS {
	return staticFS
}
