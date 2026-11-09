package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type CatalogItem struct {
	ID         string `json:"id"`
	AppName    string `json:"app_name"`
	Version    string `json:"version"`
	MinIOS     string `json:"min_ios"`
	IsPatched  bool   `json:"is_patched"`
	Modified   string `json:"modified"`
	ModifiedBy string `json:"modified_by"`
	Tweaker    string `json:"tweaker"`
	Note       string `json:"note"`
	Thumbnail  string `json:"thumbnail"`
	UpdatedAt  string `json:"updated_at"`
}

var catalogMutex sync.Mutex

func SaveAndSyncCatalog(meta AppMetadata, iconPath string) error {
	catalogMutex.Lock()
	defer catalogMutex.Unlock()

	// Pastikan folder public/data dan public/thumbnails ada
	_ = os.MkdirAll("./public/thumbnails", os.ModePerm)
	_ = os.MkdirAll("./public/data", os.ModePerm)

	jsonFilePath := "./public/data/apps.json"

	// 1. Simpan Icon ke folder public/thumbnails
	thumbWebPath := ""
	if iconPath != "" {
		fileName := meta.BundleID + ".jpg"
		dstPath := filepath.Join("./public/thumbnails", fileName)
		if err := copyFile(iconPath, dstPath); err == nil {
			thumbWebPath = "/thumbnails/" + fileName
		}
	}

	// 2. Baca data JSON yang sudah ada di public/data/apps.json
	var items []CatalogItem
	fileData, err := os.ReadFile(jsonFilePath)
	if err == nil {
		_ = json.Unmarshal(fileData, &items)
	}

	newItem := CatalogItem{
		ID:         meta.BundleID,
		AppName:    meta.AppName,
		Version:    meta.Version,
		MinIOS:     meta.MinIOS,
		IsPatched:  meta.IsPatched,
		Modified:   meta.Modifier,
		ModifiedBy: meta.ModifiedBy,
		Tweaker:    meta.Tweaker,
		Note:       meta.Note,
		Thumbnail:  thumbWebPath,
		UpdatedAt:  time.Now().Format("2006-01-02"),
	}

	// Update jika app sudah ada, atau tambahkan di paling atas jika baru
	updated := false
	for i, item := range items {
		if item.ID == newItem.ID {
			items[i] = newItem
			updated = true
			break
		}
	}

	if !updated {
		items = append([]CatalogItem{newItem}, items...)
	}

	// 3. Tulis ulang file public/data/apps.json
	output, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return fmt.Errorf("gagal marshal JSON catalog: %v", err)
	}

	if err := os.WriteFile(jsonFilePath, output, 0644); err != nil {
		return fmt.Errorf("gagal menyimpan file JSON: %v", err)
	}

	log.Printf("📦 Katalog lokal diperbarui: %s (v%s)", meta.AppName, meta.Version)

	// 4. Trigger Git Push Async ke GitHub ➔ Vercel Auto Deploy
	go syncToGitHub(meta.AppName, meta.Version)

	return nil
}

func syncToGitHub(appName, version string) {
	commitMsg := fmt.Sprintf("auto-update: release %s v%s", appName, version)

	_ = exec.Command("git", "add", "public/data/apps.json", "public/thumbnails/").Run()
	_ = exec.Command("git", "commit", "-m", commitMsg).Run()

	cmdPush := exec.Command("git", "push", "origin", "main")
	if err := cmdPush.Run(); err != nil {
		_ = exec.Command("git", "push", "origin", "master").Run()
	}

	log.Printf("🚀 Auto-Sync ke GitHub & Vercel selesai! (%s)", commitMsg)
}