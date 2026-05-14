package main

import (
	"runtime"
	"testing"
)

func TestSelectBestReleaseAssetPrefersWindowsMSI(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific asset preference")
	}

	assets := []githubReleaseAsset{
		{Name: "allapideck-windows-amd64.exe"},
		{Name: "allapideck-windows-amd64.msi"},
	}

	best := selectBestReleaseAsset(assets)
	if best == nil {
		t.Fatal("expected asset to be selected")
	}
	if best.Name != "allapideck-windows-amd64.msi" {
		t.Fatalf("expected MSI to be preferred on Windows, got %q", best.Name)
	}
}
