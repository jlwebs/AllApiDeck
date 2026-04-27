//go:build !windows

package main

func (a *App) initTray() error {
	a.tray = noopTrayController{}
	return nil
}

func (a *App) closeTray() {
	if a.tray != nil {
		a.tray.Close()
	}
}
