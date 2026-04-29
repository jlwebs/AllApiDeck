//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework QuartzCore
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#import <QuartzCore/QuartzCore.h>

static void batchApiCheckClearViewTree(NSView *view) {
	if (view == nil) {
		return;
	}

	[view setWantsLayer:YES];
	if (view.layer != nil) {
		view.layer.backgroundColor = NSColor.clearColor.CGColor;
	}

	if ([view isKindOfClass:[WKWebView class]]) {
		WKWebView *webview = (WKWebView *)view;
		[webview setOpaque:NO];
		[webview setBackgroundColor:NSColor.clearColor];
		@try {
			[webview setValue:@NO forKey:@"drawsBackground"];
		} @catch (NSException *exception) {
		}
		if (webview.scrollView != nil) {
			[webview.scrollView setDrawsBackground:NO];
			[webview.scrollView setBackgroundColor:NSColor.clearColor];
			[webview.scrollView setWantsLayer:YES];
			if (webview.scrollView.layer != nil) {
				webview.scrollView.layer.backgroundColor = NSColor.clearColor.CGColor;
			}
		}
	}

	for (NSView *subview in view.subviews) {
		batchApiCheckClearViewTree(subview);
	}
}

static void batchApiCheckEnsureTransparentWindows(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApplication sharedApplication];
		for (NSWindow *window in [NSApp windows]) {
			if (window == nil) {
				continue;
			}
			[window setOpaque:NO];
			[window setBackgroundColor:NSColor.clearColor];
			if ([window respondsToSelector:@selector(setTitlebarAppearsTransparent:)]) {
				[window setTitlebarAppearsTransparent:YES];
			}
			NSView *contentView = [window contentView];
			if (contentView != nil) {
				[contentView setWantsLayer:YES];
				if (contentView.layer != nil) {
					contentView.layer.backgroundColor = NSColor.clearColor.CGColor;
				}
				batchApiCheckClearViewTree(contentView);
			}
			[window invalidateShadow];
		}
	});
}
*/
import "C"

import "time"

func ensureTransparentWindowSurface(mode launchMode) {
	if mode != launchModePanel && mode != launchModeEditor {
		return
	}
	go func() {
		delays := []time.Duration{
			0,
			80 * time.Millisecond,
			220 * time.Millisecond,
			650 * time.Millisecond,
		}
		for _, delay := range delays {
			if delay > 0 {
				time.Sleep(delay)
			}
			C.batchApiCheckEnsureTransparentWindows()
		}
	}()
}
