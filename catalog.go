package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type CatalogItem struct {
	ID         string `json:"id"`
	AppName    string `json:"app_name"`
	Version    string `json:"version"`
	MinIOS     string `json:"min_ios"`
	Category   string `json:"category"`
	IsPatched  bool   `json:"is_patched"`
	Modified   string `json:"modified"`
	ModifiedBy string `json:"modified_by"`
	Tweaker    string `json:"tweaker"`
	Note       string `json:"note"`
	Thumbnail  string `json:"thumbnail"`
	UpdatedAt  string `json:"updated_at"`
}

var catalogMutex sync.Mutex

// Menambahkan aplikasi baru ke Web Catalog tanpa menimpa build lama
func SaveAndSyncCatalog(meta AppMetadata, iconPath string) error {
	catalogMutex.Lock()
	defer catalogMutex.Unlock()

	_ = os.MkdirAll("./public/thumbnails", os.ModePerm)
	_ = os.MkdirAll("./public/data", os.ModePerm)

	jsonFilePath := "./public/data/apps.json"

	// Membuat Unique ID berbasis timestamp agar setiap build .ipa tersimpan terpisah
	uniqueID := fmt.Sprintf("%s_%d", meta.BundleID, time.Now().Unix())

	// 1. Simpan Icon ke folder public/thumbnails dengan Unique ID
	thumbWebPath := ""
	if iconPath != "" {
		fileName := uniqueID + ".jpg"
		dstPath := filepath.Join("./public/thumbnails", fileName)
		if err := copyFile(iconPath, dstPath); err == nil {
			thumbWebPath = "/thumbnails/" + fileName
		}
	}

	// 2. Baca data JSON yang sudah ada
	var items []CatalogItem
	fileData, err := os.ReadFile(jsonFilePath)
	if err == nil {
		_ = json.Unmarshal(fileData, &items)
	}

	newItem := CatalogItem{
		ID:         uniqueID,
		AppName:    meta.AppName,
		Version:    meta.Version,
		MinIOS:     meta.MinIOS,
		Category:   meta.Category,
		IsPatched:  meta.IsPatched,
		Modified:   meta.Modifier,
		ModifiedBy: meta.ModifiedBy,
		Tweaker:    meta.Tweaker,
		Note:       meta.Note,
		Thumbnail:  thumbWebPath,
		UpdatedAt:  time.Now().Format("2006-01-02"),
	}

	// Selalu tambahkan build baru di posisi paling atas catalog
	items = append([]CatalogItem{newItem}, items...)

	// 3. Tulis ulang file public/data/apps.json
	output, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("gagal marshal JSON catalog: %v", err)
	}

	if err := os.WriteFile(jsonFilePath, output, 0644); err != nil {
		return fmt.Errorf("gagal menyimpan file JSON: %v", err)
	}

	log.Printf("📦 Katalog lokal diperbarui: %s (v%s - %s)", meta.AppName, meta.Version, meta.Category)

	// 4. Trigger Git Push Async ke GitHub ➔ Vercel Auto Deploy
	go syncToGitHub(meta.AppName, meta.Version)

	return nil
}

// Menghapus aplikasi dari Web Catalog berdasarkan Nama, Bundle ID, atau Unique ID
func DeleteFromCatalog(target string) (string, error) {
	catalogMutex.Lock()
	defer catalogMutex.Unlock()

	jsonFilePath := "./public/data/apps.json"
	var items []CatalogItem
	fileData, err := os.ReadFile(jsonFilePath)
	if err != nil {
		return "", fmt.Errorf("gagal membaca file catalog: %v", err)
	}

	_ = json.Unmarshal(fileData, &items)

	newItems := []CatalogItem{}
	deletedAppName := ""

	targetLower := strings.ToLower(strings.TrimSpace(target))

	for _, item := range items {
		isMatch := strings.EqualFold(item.ID, targetLower) ||
			strings.HasPrefix(strings.ToLower(item.ID), targetLower+"_") ||
			strings.Contains(strings.ToLower(item.AppName), targetLower)

		if isMatch {
			deletedAppName = item.AppName
			if item.Thumbnail != "" {
				thumbFile := filepath.Base(item.Thumbnail)
				_ = os.Remove(filepath.Join("./public/thumbnails", thumbFile))
			}
			continue
		}
		newItems = append(newItems, item)
	}

	if deletedAppName == "" {
		return "", fmt.Errorf("aplikasi '%s' tidak ditemukan di katalog", target)
	}

	output, err := json.MarshalIndent(newItems, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(jsonFilePath, output, 0644); err != nil {
		return "", fmt.Errorf("gagal memperbarui file catalog: %v", err)
	}

	log.Printf("🗑️ Katalog lokal diperbarui: Menghapus %s", deletedAppName)

	go syncToGitHub(deletedAppName, "deleted")

	return deletedAppName, nil
}

// Mengunggah perubahan catalog secara otomatis ke GitHub
func syncToGitHub(appName, action string) {
	var commitMsg string
	if action == "deleted" {
		commitMsg = fmt.Sprintf("auto-update: remove %s from catalog", appName)
	} else {
		commitMsg = fmt.Sprintf("auto-update: release %s v%s", appName, action)
	}

	_ = exec.Command("git", "add", "public/data/apps.json", "public/thumbnails/").Run()
	_ = exec.Command("git", "commit", "-m", commitMsg).Run()

	cmdPush := exec.Command("git", "push", "origin", "main")
	if err := cmdPush.Run(); err != nil {
		_ = exec.Command("git", "push", "origin", "master").Run()
	}

	log.Printf("🚀 Auto-Sync ke GitHub & Vercel selesai! (%s)", commitMsg)
}
