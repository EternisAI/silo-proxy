package main

import (
	"log/slog"
)

var AppVersion string

func main() {
	slog.Info("Silo Proxy Server", "version", AppVersion)
}
