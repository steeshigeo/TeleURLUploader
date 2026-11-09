package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"howett.net/plist"
)

// Fungsi pembantu untuk mengunduh file dari URL
func downloadFile(url string, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server merespons dengan status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	return err
}

// Struct data dari Info.plist
type InfoPlistData struct {
	BundleName       string `plist:"CFBundleDisplayName"`
	FallbackName     string `plist:"CFBundleName"`
	BundleID         string `plist:"CFBundleIdentifier"`
	Version          string `plist:"CFBundleShortVersionString"`
	MinimumOSVersion string `plist:"MinimumOSVersion"`
}

// Struct penampung respon iTunes API (Termasuk Genre / Kategori)
type ITunesResponse struct {
	Results []struct {
		TrackName        string   `json:"trackName"`
		TrackViewURL     string   `json:"trackViewUrl"`
		ArtworkURL       string   `json:"artworkUrl100"`
		PrimaryGenreName string   `json:"primaryGenreName"`
		Genres           []string `json:"genres"`
	} `json:"results"`
}

// Struct data akhir untuk caption, catalog & thumbnail
type AppMetadata struct {
	AppName       string
	BundleID      string
	Version       string
	UpdatedTo     string
	MinIOS        string
	Category      string
	IsPatched     bool
	Modifier      string
	ModifiedBy    string
	Tweaker       string
	Note          string
	AppStoreURL   string
	ArtworkURL    string
	CustomName    string
	FinalFileName string
}

// Membersihkan string agar aman untuk nama file
func sanitizeFilename(s string) string {
	var builder strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('_')
		}
	}
	result := builder.String()
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	return strings.Trim(result, "_")
}

// Menyusun nama file IPA otomatis
func generateDefaultFileName(appName, version, modifier, customName string) string {
	var parts []string

	cleanApp := sanitizeFilename(appName)
	if cleanApp != "" {
		parts = append(parts, cleanApp)
	}

	cleanVer := sanitizeFilename(version)
	if cleanVer != "" {
		parts = append(parts, cleanVer)
	}

	cleanMod := sanitizeFilename(modifier)
	if cleanMod != "" {
		parts = append(parts, cleanMod)
	}

	cleanName := sanitizeFilename(customName)
	if cleanName != "" {
		parts = append(parts, cleanName)
	}

	if len(parts) == 0 {
		return "downloaded_app.ipa"
	}

	return strings.Join(parts, "_") + ".ipa"
}

// Mencari Link App Store, Icon, Nama Resmi, dan Kategori/Genre via iTunes API
func FetchAppStoreData(bundleID string) (appStoreURL string, artworkURL string, trackName string, category string) {
	if strings.TrimSpace(bundleID) == "" {
		return "", "", "", ""
	}

	client := http.Client{Timeout: 5 * time.Second}
	lookupURL := fmt.Sprintf("https://itunes.apple.com/lookup?bundleId=%s", bundleID)
	if resp, err := client.Get(lookupURL); err == nil {
		var result ITunesResponse
		if json.NewDecoder(resp.Body).Decode(&result) == nil && len(result.Results) > 0 {
			resp.Body.Close()
			res := result.Results[0]

			cat := res.PrimaryGenreName
			if cat == "" && len(res.Genres) > 0 {
				cat = res.Genres[0]
			}

			return res.TrackViewURL, res.ArtworkURL, res.TrackName, cat
		}
		resp.Body.Close()
	}

	return "", "", "", ""
}

// Membaca deskripsi/caption dari Telegram (Dengan Dukungan Multi-Line Note)
func parseCaptionOptions(userCaption string) (modified string, modifiedBy string, isPatched bool, note string, tweaker string, customName string) {
	if strings.TrimSpace(userCaption) == "" {
		return "", "", false, "", "", ""
	}

	lines := strings.Split(userCaption, "\n")
	var modifiedLines []string
	var noteLines []string
	inNote := false

	for _, line := range lines {
		if inNote {
			noteLines = append(noteLines, line)
			continue
		}

		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)

		if lower == "patched" || lower == "- patched" || lower == "• patched" {
			isPatched = true
		} else if strings.HasPrefix(lower, "name:") || strings.HasPrefix(lower, "name :") {
			idx := strings.Index(line, ":")
			if idx != -1 {
				customName = strings.TrimSpace(line[idx+1:])
			}
		} else if strings.HasPrefix(lower, "note:") || strings.HasPrefix(lower, "note :") {
			inNote = true
			idx := strings.Index(line, ":")
			if idx != -1 {
				firstLine := strings.TrimSpace(line[idx+1:])
				if firstLine != "" {
					noteLines = append(noteLines, firstLine)
				}
			}
		} else if strings.HasPrefix(lower, "tweaked by:") || strings.HasPrefix(lower, "tweaked by :") || strings.HasPrefix(lower, "tweaker:") {
			idx := strings.Index(line, ":")
			if idx != -1 {
				tweaker = strings.TrimSpace(line[idx+1:])
			}
		} else if strings.HasPrefix(lower, "modified by:") || strings.HasPrefix(lower, "modified by :") || strings.HasPrefix(lower, "modded by:") || strings.HasPrefix(lower, "modded by :") {
			idx := strings.Index(line, ":")
			if idx != -1 {
				modifiedBy = strings.TrimSpace(line[idx+1:])
			}
		} else if strings.HasPrefix(lower, "modified:") || strings.HasPrefix(lower, "modifier:") {
			idx := strings.Index(line, ":")
			if idx != -1 {
				modifiedLines = append(modifiedLines, strings.TrimSpace(line[idx+1:]))
			}
		} else {
			if trimmed != "" {
				modifiedLines = append(modifiedLines, trimmed)
			}
		}
	}

	modified = strings.Join(modifiedLines, ", ")
	note = strings.Join(noteLines, "\n")
	return modified, modifiedBy, isPatched, note, tweaker, customName
}

// Membaca Info.plist dari dalam file .ipa
func ExtractIPAMetadata(ipaPath string, userCaption string) (AppMetadata, error) {
	r, err := zip.OpenReader(ipaPath)
	if err != nil {
		return AppMetadata{}, err
	}
	defer r.Close()

	var plistFile *zip.File
	for _, f := range r.File {
		name := strings.ToLower(f.Name)
		if strings.HasPrefix(name, "payload/") && strings.HasSuffix(name, ".app/info.plist") {
			plistFile = f
			break
		}
	}

	if plistFile == nil {
		return AppMetadata{}, fmt.Errorf("Info.plist tidak ditemukan")
	}

	rc, err := plistFile.Open()
	if err != nil {
		return AppMetadata{}, err
	}
	defer rc.Close()

	buf, err := io.ReadAll(rc)
	if err != nil {
		return AppMetadata{}, err
	}

	var parsedPlist InfoPlistData
	decoder := plist.NewDecoder(bytes.NewReader(buf))
	if err := decoder.Decode(&parsedPlist); err != nil {
		return AppMetadata{}, err
	}

	// Fetch data App Store (Termasuk Category)
	appStoreURL, artworkURL, storeAppName, category := FetchAppStoreData(parsedPlist.BundleID)

	appName := storeAppName
	if appName == "" {
		appName = parsedPlist.BundleName
	}
	if appName == "" {
		appName = parsedPlist.FallbackName
	}
	if appName == "" {
		appName = strings.TrimSuffix(filepath.Base(ipaPath), filepath.Ext(ipaPath))
	}

	if category == "" {
		category = "Utilities" // Fallback default jika aplikasi private/tweak khusus
	}

	modifiedText, modifiedByText, isPatched, noteText, tweakerText, customName := parseCaptionOptions(userCaption)

	finalFileName := generateDefaultFileName(appName, parsedPlist.Version, modifiedText, customName)

	meta := AppMetadata{
		AppName:       appName,
		BundleID:      parsedPlist.BundleID,
		Version:       parsedPlist.Version,
		UpdatedTo:     parsedPlist.Version,
		MinIOS:        parsedPlist.MinimumOSVersion,
		Category:      category,
		IsPatched:     isPatched,
		Modifier:      modifiedText,
		ModifiedBy:    modifiedByText,
		Tweaker:       tweakerText,
		Note:          noteText,
		AppStoreURL:   appStoreURL,
		ArtworkURL:    artworkURL,
		CustomName:    customName,
		FinalFileName: finalFileName,
	}

	return meta, nil
}

// Ekstrak AppIcon dari IPA & Konversi CgBI PNG ke JPEG standar via macOS 'sips'
func ExtractIPAIcon(ipaPath, outputDir string) (string, error) {
	r, err := zip.OpenReader(ipaPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var iconFile *zip.File

	for _, f := range r.File {
		name := strings.ToLower(f.Name)
		if strings.HasPrefix(name, "payload/") && strings.HasSuffix(name, ".png") {
			if strings.Contains(name, "appicon") {
				iconFile = f
				if strings.Contains(name, "60x60@2x") || strings.Contains(name, "60x60@3x") || strings.Contains(name, "76x76@2x") {
					break
				}
			}
		}
	}

	if iconFile == nil {
		for _, f := range r.File {
			name := strings.ToLower(f.Name)
			if strings.HasPrefix(name, "payload/") && strings.HasSuffix(name, ".png") && strings.Contains(name, "icon") {
				iconFile = f
				break
			}
		}
	}

	if iconFile == nil {
		return "", fmt.Errorf("AppIcon tidak ditemukan di dalam IPA")
	}

	rc, err := iconFile.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	rawIconPath := filepath.Join(outputDir, "raw_"+filepath.Base(iconFile.Name))
	outFile, err := os.Create(rawIconPath)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(outFile, rc)
	outFile.Close()
	if copyErr != nil {
		os.Remove(rawIconPath)
		return "", copyErr
	}

	jpgOutputPath := filepath.Join(outputDir, "thumb_"+strings.TrimSuffix(filepath.Base(iconFile.Name), filepath.Ext(iconFile.Name))+".jpg")
	cmd := exec.Command("sips", "-s", "format", "jpeg", rawIconPath, "--out", jpgOutputPath)
	if err := cmd.Run(); err == nil {
		os.Remove(rawIconPath)
		return jpgOutputPath, nil
	}

	return rawIconPath, nil
}

// ALUR PRIORITAS THUMBNAIL (1: AppIcon dari IPA -> 2: Artwork dari App Store)
func GetAppIconThumbnail(meta AppMetadata, ipaPath, outputDir string) string {
	iconPath, err := ExtractIPAIcon(ipaPath, outputDir)
	if err == nil && iconPath != "" {
		return iconPath
	}

	if meta.ArtworkURL != "" {
		appStoreIconPath := filepath.Join(outputDir, "thumb_appstore.jpg")
		if err := downloadFile(meta.ArtworkURL, appStoreIconPath); err == nil {
			return appStoreIconPath
		}
	}

	return ""
}

// Menyusun caption berformat Telegram Blockquote
func BuildIPACaption(meta AppMetadata) string {
	titleText := fmt.Sprintf("<b>%s</b>", meta.AppName)
	if meta.AppStoreURL != "" {
		titleText = fmt.Sprintf("<a href=\"%s\"><b>%s</b></a>", meta.AppStoreURL, meta.AppName)
	}

	var blockLines []string
	blockLines = append(blockLines, fmt.Sprintf("• Bundle ID: %s", meta.BundleID))
	blockLines = append(blockLines, fmt.Sprintf("• Category: %s", meta.Category))
	blockLines = append(blockLines, fmt.Sprintf("• Updated to: %s", meta.UpdatedTo))
	blockLines = append(blockLines, fmt.Sprintf("• Minimum iOS: %s", meta.MinIOS))

	if meta.IsPatched {
		blockLines = append(blockLines, "• Patched")
	}

	if meta.Modifier != "" {
		blockLines = append(blockLines, fmt.Sprintf("• Modified: %s", meta.Modifier))
	}

	if meta.ModifiedBy != "" {
		blockLines = append(blockLines, fmt.Sprintf("• Modified by: %s", meta.ModifiedBy))
	}

	if meta.Tweaker != "" {
		blockLines = append(blockLines, fmt.Sprintf("• Tweaked by %s", meta.Tweaker))
	}

	caption := fmt.Sprintf(
		"%s\n\n"+
			"<blockquote>%s</blockquote>",
		titleText,
		strings.Join(blockLines, "\n"),
	)

	if strings.TrimSpace(meta.Note) != "" {
		safeNote := html.EscapeString(meta.Note)
		caption += fmt.Sprintf("\n\n<blockquote>• Note:\n%s</blockquote>", safeNote)
	}

	return caption
}