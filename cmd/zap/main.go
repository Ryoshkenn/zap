package main

import "github.com/Ryoshkenn/zap/internal/cmd"

var version = "dev"

func main() {
	cmd.Execute(version)
}
