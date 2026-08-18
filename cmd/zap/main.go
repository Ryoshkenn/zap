package main

import "github.com/Ryoshkenn/zap/internal/cmd"

// version is the fallback for builds made outside a release. Releases override
// it via goreleaser's -X main.version={{.Version}} ldflag.
var version = "v1.2.0"

func main() {
	cmd.Execute(version)
}
