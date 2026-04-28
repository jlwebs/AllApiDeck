package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	appUpdateGithubOwner         = "jlwebs"
	appUpdateGithubRepo          = "AllApiDeck"
	appUpdateReleaseAPIURL       = "https://api.github.com/repos/jlwebs/AllApiDeck/releases/latest"
	appUpdateDownloadEventName   = "app:update-download-progress"
	appUpdateStageIdle           = "idle"
	appUpdateStagePreparing      = "preparing"
	appUpdateStageDownloading    = "downloading"
	appUpdateStageCompleted      = "completed"
	appUpdateStageError          = "error"
	appUpdateDownloadsDirName    = "updates"
	appUpdateDownloadBufferBytes = 256 * 1024
)

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
	ContentType        string `json:"content_type"`
}

type githubReleasePayload struct {
	TagName string               `json:"tag_name"`
	HTMLURL string               `json:"html_url"`
	Body    string               `json:"body"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type AppUpdateAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browserDownloadUrl"`
	Size               int64  `json:"size"`
	ContentType        string `json:"contentType"`
}

type AppUpdateReleaseInfo struct {
	LatestTag     string          `json:"latestTag"`
	LatestVersion string          `json:"latestVersion"`
	HTMLURL       string          `json:"htmlUrl"`
	Body          string          `json:"body"`
	TargetOS      string          `json:"targetOs"`
	TargetArch    string          `json:"targetArch"`
	Asset         *AppUpdateAsset `json:"asset,omitempty"`
}

type AppUpdateDownloadSnapshot struct {
	Active        bool    `json:"active"`
	Stage         string  `json:"stage"`
	LatestTag     string  `json:"latestTag"`
	FileName      string  `json:"fileName"`
	DownloadURL   string  `json:"downloadUrl"`
	SavedPath     string  `json:"savedPath"`
	TotalBytes    int64   `json:"totalBytes"`
	ReceivedBytes int64   `json:"receivedBytes"`
	Percent       float64 `json:"percent"`
	Message       string  `json:"message"`
	Error         string  `json:"error"`
	StartedAt     int64   `json:"startedAt"`
	UpdatedAt     int64   `json:"updatedAt"`
}

func (a *App) GetLatestAppReleaseInfo() (*AppUpdateReleaseInfo, error) {
	payload, err := fetchLatestAppReleasePayload(context.Background())
	if err != nil {
		return nil, err
	}

	asset := selectBestReleaseAsset(payload.Assets)
	info := &AppUpdateReleaseInfo{
		LatestTag:     strings.TrimSpace(payload.TagName),
		LatestVersion: normalizeUpdateVersion(payload.TagName),
		HTMLURL:       strings.TrimSpace(payload.HTMLURL),
		Body:          payload.Body,
		TargetOS:      runtime.GOOS,
		TargetArch:    runtime.GOARCH,
	}
	if asset != nil {
		info.Asset = &AppUpdateAsset{
			Name:               asset.Name,
			BrowserDownloadURL: asset.BrowserDownloadURL,
			Size:               asset.Size,
			ContentType:        asset.ContentType,
		}
	}
	return info, nil
}

func (a *App) GetAppUpdateDownloadSnapshot() AppUpdateDownloadSnapshot {
	a.updateDownloadMu.Lock()
	defer a.updateDownloadMu.Unlock()
	return a.updateDownload
}

func (a *App) StartLatestAppReleaseDownload() (AppUpdateDownloadSnapshot, error) {
	a.updateDownloadMu.Lock()
	if a.updateDownload.Active && (a.updateDownload.Stage == appUpdateStagePreparing || a.updateDownload.Stage == appUpdateStageDownloading) {
		snapshot := a.updateDownload
		a.updateDownloadMu.Unlock()
		return snapshot, nil
	}
	a.updateDownloadMu.Unlock()

	info, err := a.GetLatestAppReleaseInfo()
	if err != nil {
		debugLogf("app update metadata fetch failed before download: %v", err)
		a.setAppUpdateDownloadSnapshot(AppUpdateDownloadSnapshot{
			Active:    false,
			Stage:     appUpdateStageError,
			Error:     err.Error(),
			Message:   "获取最新版本信息失败",
			UpdatedAt: time.Now().UnixMilli(),
		})
		return a.GetAppUpdateDownloadSnapshot(), err
	}
	if info.Asset == nil || strings.TrimSpace(info.Asset.BrowserDownloadURL) == "" {
		err := fmt.Errorf("no suitable release asset for %s/%s", runtime.GOOS, runtime.GOARCH)
		debugLogf("app update asset select failed: %v", err)
		a.setAppUpdateDownloadSnapshot(AppUpdateDownloadSnapshot{
			Active:    false,
			Stage:     appUpdateStageError,
			LatestTag: info.LatestTag,
			Error:     err.Error(),
			Message:   "当前系统没有匹配的安装包",
			UpdatedAt: time.Now().UnixMilli(),
		})
		return a.GetAppUpdateDownloadSnapshot(), err
	}

	snapshot := AppUpdateDownloadSnapshot{
		Active:      true,
		Stage:       appUpdateStagePreparing,
		LatestTag:   info.LatestTag,
		FileName:    info.Asset.Name,
		DownloadURL: info.Asset.BrowserDownloadURL,
		TotalBytes:  info.Asset.Size,
		Percent:     0,
		Message:     "准备下载更新包",
		StartedAt:   time.Now().UnixMilli(),
		UpdatedAt:   time.Now().UnixMilli(),
	}
	a.setAppUpdateDownloadSnapshot(snapshot)

	go a.runLatestAppReleaseDownload(snapshot)
	return snapshot, nil
}

func (a *App) OpenDownloadedAppUpdate() error {
	snapshot := a.GetAppUpdateDownloadSnapshot()
	targetPath := strings.TrimSpace(snapshot.SavedPath)
	if targetPath == "" {
		return fmt.Errorf("downloaded update package not found")
	}
	if _, err := os.Stat(targetPath); err != nil {
		return fmt.Errorf("downloaded update package missing: %w", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", targetPath)
		configureBackgroundCmd(cmd)
	case "darwin":
		cmd = exec.Command("open", targetPath)
	default:
		cmd = exec.Command("xdg-open", targetPath)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}

func (a *App) runLatestAppReleaseDownload(snapshot AppUpdateDownloadSnapshot) {
	targetDir := filepath.Join(resolveRuntimeRootDir(), appUpdateDownloadsDirName)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		a.setAppUpdateDownloadError(snapshot, "创建更新目录失败", err)
		return
	}

	finalPath := filepath.Join(targetDir, sanitizeUpdateFilename(snapshot.FileName))
	tempPath := finalPath + ".download"

	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, snapshot.DownloadURL, nil)
	if err != nil {
		a.setAppUpdateDownloadError(snapshot, "创建下载请求失败", err)
		return
	}
	request.Header.Set("Accept", "application/octet-stream")
	request.Header.Set("User-Agent", "AllApiDeck-Updater")

	client, err := newOutboundHTTPClient(0)
	if err != nil {
		a.setAppUpdateDownloadError(snapshot, "创建下载客户端失败", err)
		return
	}
	response, err := client.Do(request)
	if err != nil {
		debugLogf("app update download request failed: %v", err)
		a.setAppUpdateDownloadError(snapshot, "下载更新包失败", err)
		return
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		err = fmt.Errorf("github_asset_http_%d", response.StatusCode)
		debugLogf("app update download http error: %v", err)
		a.setAppUpdateDownloadError(snapshot, "下载更新包失败", err)
		return
	}

	totalBytes := response.ContentLength
	if totalBytes <= 0 {
		totalBytes = snapshot.TotalBytes
	}

	file, err := os.Create(tempPath)
	if err != nil {
		a.setAppUpdateDownloadError(snapshot, "创建更新包文件失败", err)
		return
	}

	next := snapshot
	next.Stage = appUpdateStageDownloading
	next.TotalBytes = totalBytes
	next.Message = "正在下载更新包"
	next.UpdatedAt = time.Now().UnixMilli()
	a.setAppUpdateDownloadSnapshot(next)

	buffer := make([]byte, appUpdateDownloadBufferBytes)
	var received int64
	lastEmitAt := time.Now().Add(-time.Second)

	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
				_ = file.Close()
				_ = os.Remove(tempPath)
				a.setAppUpdateDownloadError(snapshot, "写入更新包失败", writeErr)
				return
			}
			received += int64(n)
			if time.Since(lastEmitAt) >= 150*time.Millisecond {
				current := next
				current.ReceivedBytes = received
				current.Percent = computeUpdateDownloadPercent(received, totalBytes)
				current.UpdatedAt = time.Now().UnixMilli()
				a.setAppUpdateDownloadSnapshot(current)
				lastEmitAt = time.Now()
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			_ = file.Close()
			_ = os.Remove(tempPath)
			a.setAppUpdateDownloadError(snapshot, "下载更新包失败", readErr)
			return
		}
	}

	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		a.setAppUpdateDownloadError(snapshot, "关闭更新包文件失败", err)
		return
	}

	if err := os.Remove(finalPath); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tempPath)
		a.setAppUpdateDownloadError(snapshot, "覆盖已有更新包失败", err)
		return
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		_ = os.Remove(tempPath)
		a.setAppUpdateDownloadError(snapshot, "保存更新包失败", err)
		return
	}

	finished := next
	finished.Active = false
	finished.Stage = appUpdateStageCompleted
	finished.ReceivedBytes = received
	finished.TotalBytes = maxUpdateInt64(totalBytes, received)
	finished.Percent = 100
	finished.SavedPath = finalPath
	finished.Message = "下载完成，可以打开安装包"
	finished.Error = ""
	finished.UpdatedAt = time.Now().UnixMilli()
	a.setAppUpdateDownloadSnapshot(finished)
}

func (a *App) setAppUpdateDownloadError(snapshot AppUpdateDownloadSnapshot, message string, err error) {
	next := snapshot
	next.Active = false
	next.Stage = appUpdateStageError
	next.Error = strings.TrimSpace(err.Error())
	next.Message = strings.TrimSpace(message)
	next.UpdatedAt = time.Now().UnixMilli()
	a.setAppUpdateDownloadSnapshot(next)
}

func (a *App) setAppUpdateDownloadSnapshot(snapshot AppUpdateDownloadSnapshot) {
	a.updateDownloadMu.Lock()
	a.updateDownload = snapshot
	a.updateDownloadMu.Unlock()

	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, appUpdateDownloadEventName, snapshot)
	}
}

func fetchLatestAppReleasePayload(ctx context.Context) (*githubReleasePayload, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, appUpdateReleaseAPIURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "AllApiDeck-Updater")

	client, err := newOutboundHTTPClient(12 * time.Second)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		debugLogf("app update release fetch failed: %v", err)
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		err = fmt.Errorf("github_release_http_%d", response.StatusCode)
		debugLogf("app update release http error: %v", err)
		return nil, err
	}

	var payload githubReleasePayload
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		debugLogf("app update release decode failed: %v", err)
		return nil, err
	}
	return &payload, nil
}

func selectBestReleaseAsset(assets []githubReleaseAsset) *githubReleaseAsset {
	if len(assets) == 0 {
		return nil
	}

	bestIndex := -1
	bestScore := -1
	for index, asset := range assets {
		score := scoreReleaseAsset(asset)
		if score > bestScore {
			bestIndex = index
			bestScore = score
		}
	}
	if bestIndex < 0 || bestScore < 0 {
		return nil
	}
	asset := assets[bestIndex]
	return &asset
}

func scoreReleaseAsset(asset githubReleaseAsset) int {
	name := strings.ToLower(strings.TrimSpace(asset.Name))
	if name == "" {
		return -1
	}
	if !strings.Contains(name, "allapideck") && !strings.Contains(name, "all-api-deck") {
		return -1
	}

	score := 0
	switch runtime.GOOS {
	case "windows":
		if !strings.Contains(name, "windows") || !strings.HasSuffix(name, ".exe") {
			return -1
		}
		score += 100
		if strings.Contains(name, runtime.GOARCH) {
			score += 20
		}
		if runtime.GOARCH == "amd64" && strings.Contains(name, "amd64") {
			score += 20
		}
	case "darwin":
		if !strings.Contains(name, "macos") || !strings.HasSuffix(name, ".dmg") {
			return -1
		}
		score += 100
		if strings.Contains(name, "universal") {
			score += 30
		}
		if strings.Contains(name, runtime.GOARCH) {
			score += 20
		}
	case "linux":
		if !strings.Contains(name, "linux") {
			return -1
		}
		score += 80
		switch {
		case strings.HasSuffix(name, ".appimage"):
			score += 30
		case strings.HasSuffix(name, ".deb"):
			score += 20
		case strings.HasSuffix(name, ".tar.gz"):
			score += 10
		default:
			return -1
		}
		if strings.Contains(name, runtime.GOARCH) {
			score += 20
		}
	default:
		return -1
	}
	return score
}

func sanitizeUpdateFilename(name string) string {
	cleaned := strings.TrimSpace(filepath.Base(name))
	if cleaned == "" || cleaned == "." || cleaned == string(filepath.Separator) {
		return "AllApiDeck-update.bin"
	}
	return cleaned
}

func computeUpdateDownloadPercent(received int64, total int64) float64 {
	if total <= 0 || received <= 0 {
		return 0
	}
	if received >= total {
		return 100
	}
	return (float64(received) / float64(total)) * 100
}

func normalizeUpdateVersion(value string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(value, "v"), "V"))
}

func maxUpdateInt64(left int64, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
