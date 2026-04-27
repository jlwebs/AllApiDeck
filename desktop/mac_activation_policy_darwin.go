//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static void batchApiCheckSetAccessoryActivationPolicy(void) {
	[NSApplication sharedApplication];
	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
}
*/
import "C"

func applyMacActivationPolicy(mode launchMode) {
	if mode == launchModeMain {
		return
	}
	C.batchApiCheckSetAccessoryActivationPolicy()
}
