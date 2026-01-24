package main

import (
	"log/slog"
)

var AppVersion string

func main() {
	slog.Info("Silo Proxy Agent", "version", AppVersion)

	// TODO: Implement agent logic
}
