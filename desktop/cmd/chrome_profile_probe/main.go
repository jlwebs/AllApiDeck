package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/syndtr/goleveldb/leveldb"
	"golang.org/x/sys/windows"
	_ "modernc.org/sqlite"
)

var candidateStorageKeys = []string{
	"auth_user",
	"user",
	"auth_token",
	"access_token",
	"token",
	"authToken",
	"refresh_token",
	"token_expires_at",
}

type localStateFile struct {
	OSCrypt struct {
		EncryptedKey string `json:"encrypted_key"`
	} `json:"os_crypt"`
}

type backupFile struct {
	Accounts struct {
		Accounts []struct {
			SiteName string `json:"site_name"`
			SiteURL  string `json:"site_url"`
		} `json:"accounts"`
	} `json:"accounts"`
}

type probeTarget struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Origin string `json:"origin"`
	Host   string `json:"host"`
}

type cookieHit struct {
	Target        string `json:"target"`
	HostKey       string `json:"hostKey"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Source        string `json:"source"`
	ValuePreview  string `json:"valuePreview"`
	ExpiresUTC    int64  `json:"expiresUtc"`
	IsPersistent  bool   `json:"isPersistent"`
	IsSecure      bool   `json:"isSecure"`
	IsHttpOnly    bool   `json:"isHttpOnly"`
	DecryptFailed string `json:"decryptFailed,omitempty"`
}

type storageHit struct {
	Store       string `json:"store"`
	Target      string `json:"target"`
	Parsed      bool   `json:"parsed"`
	Origin      string `json:"origin,omitempty"`
	StorageKey  string `json:"storageKey,omitempty"`
	RawKeyHex   string `json:"rawKeyHex,omitempty"`
	RawKeyText  string `json:"rawKeyText,omitempty"`
	ValueText   string `json:"valueText,omitempty"`
	ValueHex    string `json:"valueHex,omitempty"`
	ValueFormat string `json:"valueFormat,omitempty"`
}

type probeReport struct {
	GeneratedAt     string        `json:"generatedAt"`
	ChromeUserData  string        `json:"chromeUserData"`
	DefaultProfile  string        `json:"defaultProfile"`
	LocalStatePath  string        `json:"localStatePath"`
	CookiesPath     string        `json:"cookiesPath"`
	LocalStorageDir string        `json:"localStorageDir"`
	SessionStoreDir string        `json:"sessionStorageDir"`
	Targets         []probeTarget `json:"targets"`
	Cookies         []cookieHit   `json:"cookies"`
	LocalStorage    []storageHit  `json:"localStorage"`
	SessionStorage  []storageHit  `json:"sessionStorage"`
	Warnings        []string      `json:"warnings"`
}

func main() {
	var (
		backupPath = flag.String("backup", "backup/accounts-backup-2026-04-01.json", "Path to account backup json")
		siteURL    = flag.String("site", "", "Optional site URL to inspect in addition to backup targets")
		limit      = flag.Int("limit", 12, "Max hits per storage bucket")
		writePath  = flag.String("out", ".codex-temp/chrome_profile_probe_report.json", "Where to write the JSON report")
	)
	flag.Parse()

	report, err := runProbe(*backupPath, *siteURL, *limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(*writePath), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create report directory: %v\n", err)
		os.Exit(1)
	}

	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal report: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*writePath, body, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write report: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("report written: %s\n", *writePath)
	fmt.Printf("targets=%d cookies=%d localStorage=%d sessionStorage=%d\n",
		len(report.Targets), len(report.Cookies), len(report.LocalStorage), len(report.SessionStorage))
}

func runProbe(backupPath string, siteURL string, limit int) (*probeReport, error) {
	userDataDir := filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "User Data")
	defaultProfile := filepath.Join(userDataDir, "Default")
	localStatePath := filepath.Join(userDataDir, "Local State")
	cookiesPath := filepath.Join(defaultProfile, "Network", "Cookies")
	localStorageDir := filepath.Join(defaultProfile, "Local Storage", "leveldb")
	sessionStorageDir := filepath.Join(defaultProfile, "Session Storage")

	targets, warnings, err := loadTargets(backupPath, siteURL)
	if err != nil {
		return nil, err
	}

	report := &probeReport{
		GeneratedAt:     time.Now().Format(time.RFC3339),
		ChromeUserData:  userDataDir,
		DefaultProfile:  defaultProfile,
		LocalStatePath:  localStatePath,
		CookiesPath:     cookiesPath,
		LocalStorageDir: localStorageDir,
		SessionStoreDir: sessionStorageDir,
		Targets:         targets,
		Warnings:        warnings,
	}

	masterKey, err := readChromeMasterKey(localStatePath)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("master key read failed: %v", err))
	} else {
		cookies, cookieWarnings := collectCookies(cookiesPath, masterKey, targets, limit)
		report.Cookies = cookies
		report.Warnings = append(report.Warnings, cookieWarnings...)
	}

	localHits, localWarnings := collectLevelDBHits(localStorageDir, "localStorage", targets, limit)
	sessionHits, sessionWarnings := collectLevelDBHits(sessionStorageDir, "sessionStorage", targets, limit)
	report.LocalStorage = localHits
	report.SessionStorage = sessionHits
	report.Warnings = append(report.Warnings, localWarnings...)
	report.Warnings = append(report.Warnings, sessionWarnings...)

	return report, nil
}

func loadTargets(backupPath string, siteURL string) ([]probeTarget, []string, error) {
	targetMap := map[string]probeTarget{}
	warnings := []string{}

	addTarget := func(name string, rawURL string) {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			return
		}
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Host == "" {
			warnings = append(warnings, fmt.Sprintf("skip invalid url %q", rawURL))
			return
		}
		host := strings.ToLower(parsed.Hostname())
		origin := parsed.Scheme + "://" + parsed.Host
		key := origin
		targetMap[key] = probeTarget{
			Name:   strings.TrimSpace(name),
			URL:    rawURL,
			Origin: origin,
			Host:   host,
		}
	}

	if backupPath != "" {
		if data, err := os.ReadFile(backupPath); err == nil {
			var backup backupFile
			if err := json.Unmarshal(data, &backup); err != nil {
				return nil, nil, fmt.Errorf("parse backup %s: %w", backupPath, err)
			}
			for _, account := range backup.Accounts.Accounts {
				addTarget(account.SiteName, account.SiteURL)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, nil, fmt.Errorf("read backup %s: %w", backupPath, err)
		} else {
			warnings = append(warnings, fmt.Sprintf("backup file not found: %s", backupPath))
		}
	}

	if siteURL != "" {
		addTarget("manual", siteURL)
	}

	targets := make([]probeTarget, 0, len(targetMap))
	for _, target := range targetMap {
		targets = append(targets, target)
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Origin < targets[j].Origin
	})

	return targets, warnings, nil
}

func readChromeMasterKey(localStatePath string) ([]byte, error) {
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return nil, err
	}

	var state localStateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	encoded := strings.TrimSpace(state.OSCrypt.EncryptedKey)
	if encoded == "" {
		return nil, errors.New("os_crypt.encrypted_key missing")
	}

	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	if bytes.HasPrefix(raw, []byte("DPAPI")) {
		raw = raw[len("DPAPI"):]
	}

	return decryptDPAPI(raw)
}

func collectCookies(cookiesPath string, masterKey []byte, targets []probeTarget, limit int) ([]cookieHit, []string) {
	if _, err := os.Stat(cookiesPath); err != nil {
		return nil, []string{fmt.Sprintf("cookies db missing: %s", cookiesPath)}
	}

	tmpPath, cleanup, err := copyToTempFile(cookiesPath)
	if err != nil {
		return nil, []string{fmt.Sprintf("copy cookies db failed: %v", err)}
	}
	defer cleanup()

	db, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return nil, []string{fmt.Sprintf("open cookies db failed: %v", err)}
	}
	defer db.Close()

	hits := []cookieHit{}
	warnings := []string{}
	seen := map[string]bool{}

	for _, target := range targets {
		rows, err := db.Query(`
			SELECT host_key, name, path, value, encrypted_value, expires_utc, is_persistent, is_secure, is_httponly
			FROM cookies
			WHERE host_key = ? OR host_key LIKE ? OR host_key LIKE ?
			ORDER BY host_key, name
			LIMIT ?`,
			target.Host, "%."+target.Host, "%"+target.Host, limit)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("query cookies for %s failed: %v", target.Host, err))
			continue
		}

		for rows.Next() {
			var (
				hostKey      string
				name         string
				pathValue    string
				plainValue   string
				encrypted    []byte
				expiresUTC   int64
				isPersistent int64
				isSecure     int64
				isHTTPOnly   int64
			)
			if err := rows.Scan(&hostKey, &name, &pathValue, &plainValue, &encrypted, &expiresUTC, &isPersistent, &isSecure, &isHTTPOnly); err != nil {
				warnings = append(warnings, fmt.Sprintf("scan cookies row failed: %v", err))
				continue
			}

			key := target.Origin + "|" + hostKey + "|" + name + "|" + pathValue
			if seen[key] {
				continue
			}
			seen[key] = true

			source := "value"
			decryptFailed := ""
			value := plainValue
			if value == "" && len(encrypted) > 0 {
				source = "encrypted_value"
				decrypted, err := decryptChromeValue(masterKey, encrypted)
				if err != nil {
					decryptFailed = err.Error()
				} else {
					value = string(decrypted)
				}
			}

			hits = append(hits, cookieHit{
				Target:        target.Origin,
				HostKey:       hostKey,
				Name:          name,
				Path:          pathValue,
				Source:        source,
				ValuePreview:  previewText(value, 120),
				ExpiresUTC:    expiresUTC,
				IsPersistent:  isPersistent != 0,
				IsSecure:      isSecure != 0,
				IsHttpOnly:    isHTTPOnly != 0,
				DecryptFailed: decryptFailed,
			})
		}
		_ = rows.Close()
	}

	return hits, warnings
}

func collectLevelDBHits(dir string, storeName string, targets []probeTarget, limit int) ([]storageHit, []string) {
	if _, err := os.Stat(dir); err != nil {
		return nil, []string{fmt.Sprintf("%s missing: %s", storeName, dir)}
	}

	tmpDir, cleanup, err := copyDirToTemp(dir)
	if err != nil {
		return nil, []string{fmt.Sprintf("copy %s failed: %v", storeName, err)}
	}
	defer cleanup()

	db, err := leveldb.OpenFile(tmpDir, nil)
	if err != nil {
		return nil, []string{fmt.Sprintf("open %s failed: %v", storeName, err)}
	}
	defer db.Close()

	targetMatch := func(text string) string {
		text = strings.ToLower(text)
		for _, target := range targets {
			if strings.Contains(text, strings.ToLower(target.Origin)) || strings.Contains(text, target.Host) {
				return target.Origin
			}
		}
		return ""
	}

	isCandidate := func(text string) bool {
		lower := strings.ToLower(text)
		for _, item := range candidateStorageKeys {
			if strings.Contains(lower, strings.ToLower(item)) {
				return true
			}
		}
		return false
	}

	preferredHits := []storageHit{}
	fallbackHits := []storageHit{}
	warnings := []string{}
	iter := db.NewIterator(nil, nil)
	defer iter.Release()

	for iter.Next() {
		key := append([]byte(nil), iter.Key()...)
		value := append([]byte(nil), iter.Value()...)
		rawKeyText := sanitizedText(key)
		rawValueText, valueFormat := decodeStorageValue(value)

		target := targetMatch(rawKeyText)
		if target == "" {
			target = targetMatch(rawValueText)
		}
		if target == "" && !isCandidate(rawKeyText) && !isCandidate(rawValueText) {
			continue
		}

		origin, storageKey, ok := parseDOMStorageKey(key)
		if target == "" && ok {
			target = targetMatch(origin)
		}
		if target == "" {
			target = "unknown"
		}

		hit := storageHit{
			Store:       storeName,
			Target:      target,
			Parsed:      ok,
			Origin:      origin,
			StorageKey:  storageKey,
			RawKeyHex:   previewHex(key, 64),
			RawKeyText:  previewText(rawKeyText, 160),
			ValueText:   previewText(rawValueText, 220),
			ValueHex:    previewHex(value, 96),
			ValueFormat: valueFormat,
		}

		if ok || isCandidate(storageKey) || isCandidate(rawKeyText) || isCandidate(rawValueText) {
			preferredHits = append(preferredHits, hit)
		} else {
			fallbackHits = append(fallbackHits, hit)
		}
		if len(preferredHits) >= limit {
			break
		}
	}

	if err := iter.Error(); err != nil {
		warnings = append(warnings, fmt.Sprintf("iterate %s failed: %v", storeName, err))
	}

	hits := preferredHits
	for _, hit := range fallbackHits {
		if len(hits) >= limit {
			break
		}
		hits = append(hits, hit)
	}

	return hits, warnings
}

func parseDOMStorageKey(key []byte) (origin string, storageKey string, ok bool) {
	if len(key) < 3 || key[0] != '_' {
		return "", "", false
	}
	body := key[1:]
	sep := bytes.IndexByte(body, 0)
	if sep <= 0 || sep+1 >= len(body) {
		return "", "", false
	}
	origin = sanitizedText(body[:sep])
	storageKey = sanitizedText(body[sep+1:])
	if origin == "" || storageKey == "" {
		return "", "", false
	}
	return origin, storageKey, true
}

func decodeStorageValue(value []byte) (string, string) {
	if len(value) == 0 {
		return "", "empty"
	}

	if value[0] == 1 && len(value) > 1 {
		if txt, ok := tryDecodeUTF16LE(value[1:]); ok {
			return txt, "prefixed-utf16le"
		}
		if isMostlyPrintable(value[1:]) {
			return sanitizedText(value[1:]), "prefixed-utf8-ish"
		}
	}

	if txt, ok := tryDecodeUTF16LE(value); ok {
		return txt, "utf16le"
	}

	if isMostlyPrintable(value) {
		return sanitizedText(value), "utf8-ish"
	}

	return sanitizedText(value), "binary"
}

func tryDecodeUTF16LE(data []byte) (string, bool) {
	if len(data) < 2 || len(data)%2 != 0 {
		return "", false
	}
	u16 := make([]uint16, 0, len(data)/2)
	zeroHi := 0
	for i := 0; i < len(data); i += 2 {
		v := uint16(data[i]) | uint16(data[i+1])<<8
		u16 = append(u16, v)
		if data[i+1] == 0 {
			zeroHi++
		}
	}
	if zeroHi < len(u16)/3 {
		return "", false
	}

	text := string(utf16.Decode(u16))
	text = strings.TrimSpace(strings.ReplaceAll(text, "\x00", ""))
	if text == "" {
		return "", false
	}
	return text, true
}

func isMostlyPrintable(data []byte) bool {
	printable := 0
	for _, b := range data {
		if b == '\r' || b == '\n' || b == '\t' || (b >= 32 && b <= 126) {
			printable++
		}
	}
	return printable*100/len(data) >= 75
}

func sanitizedText(data []byte) string {
	text := string(data)
	text = strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == '\t' || (r >= 32 && r != 127) {
			return r
		}
		return -1
	}, text)
	return strings.TrimSpace(text)
}

func previewText(input string, max int) string {
	input = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(input, "\r", " "), "\n", " "))
	if len(input) <= max {
		return input
	}
	return input[:max] + "..."
}

func previewHex(data []byte, max int) string {
	if len(data) > max {
		data = data[:max]
	}
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(data)*2)
	for i, b := range data {
		out[i*2] = hexdigits[b>>4]
		out[i*2+1] = hexdigits[b&0x0f]
	}
	return string(out)
}

func decryptChromeValue(masterKey []byte, encrypted []byte) ([]byte, error) {
	if len(encrypted) == 0 {
		return nil, errors.New("empty encrypted value")
	}
	if bytes.HasPrefix(encrypted, []byte("v10")) || bytes.HasPrefix(encrypted, []byte("v11")) {
		if len(encrypted) < 3+12+16 {
			return nil, errors.New("encrypted value too short for aes-gcm")
		}
		block, err := aes.NewCipher(masterKey)
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		nonce := encrypted[3 : 3+12]
		ciphertext := encrypted[3+12:]
		return gcm.Open(nil, nonce, ciphertext, nil)
	}
	return decryptDPAPI(encrypted)
}

func copyToTempFile(src string) (string, func(), error) {
	data, err := readWindowsSharedFile(src)
	if err != nil {
		return "", nil, err
	}
	tmpDir, err := os.MkdirTemp("", "chrome-profile-probe-*")
	if err != nil {
		return "", nil, err
	}
	dst := filepath.Join(tmpDir, filepath.Base(src))
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, err
	}
	return dst, func() { _ = os.RemoveAll(tmpDir) }, nil
}

func readWindowsSharedFile(path string) ([]byte, error) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	handle, err := windows.CreateFile(
		ptr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(handle)

	file := os.NewFile(uintptr(handle), path)
	if file == nil {
		return nil, fmt.Errorf("wrap file handle failed: %s", path)
	}
	defer file.Close()

	return io.ReadAll(file)
}

func copyDirToTemp(src string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "chrome-storage-probe-*")
	if err != nil {
		return "", nil, err
	}
	dst := filepath.Join(tmpDir, filepath.Base(src))
	if err := os.MkdirAll(dst, 0o755); err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return "", nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, entry.Name()))
		if err != nil {
			continue
		}
		_ = os.WriteFile(filepath.Join(dst, entry.Name()), data, 0o600)
	}
	return dst, func() { _ = os.RemoveAll(tmpDir) }, nil
}

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32            = syscall.NewLazyDLL("crypt32.dll")
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procCryptUnprotect = crypt32.NewProc("CryptUnprotectData")
	procLocalFree      = kernel32.NewProc("LocalFree")
)

func decryptDPAPI(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty dpapi payload")
	}

	var in dataBlob
	in.cbData = uint32(len(data))
	in.pbData = &data[0]
	var out dataBlob

	r, _, err := procCryptUnprotect.Call(
		uintptr(unsafe.Pointer(&in)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&out)),
	)
	if r == 0 {
		return nil, err
	}
	defer procLocalFree.Call(uintptr(unsafe.Pointer(out.pbData)))

	buf := unsafe.Slice(out.pbData, out.cbData)
	return append([]byte(nil), buf...), nil
}
