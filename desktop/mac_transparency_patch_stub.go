//go:build !darwin

package main

func ensureTransparentWindowSurface(mode launchMode) {
	_ = mode
}
