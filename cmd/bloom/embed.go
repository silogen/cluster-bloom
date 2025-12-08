package main

import "embed"

//go:embed all:web/static
var WebFS embed.FS
