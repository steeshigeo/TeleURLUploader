package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

var activeCaptions = make(map[int64]string)

func main() {
	errEnv := godotenv.Load(".env")
	if errEnv != nil {
		log.Printf("⚠️ WARNING: Gagal memuat file .env! Error: %v", errEnv)
	} else {
		log.Println("✅ File .env berhasil dimuat.")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		botToken = os.Getenv("TOKEN")
	}

	if botToken == "" {
		log.Fatal("BOT_TOKEN tidak ditemukan di file .env")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatalf("Gagal membuat bot: %v", err)
	}

	localAPI := os.Getenv("LOCAL_API_URL")
	if localAPI != "" {
		bot.SetAPIEndpoint(localAPI)
		log.Printf("Menggunakan Local Telegram Bot API: %s", localAPI)
	}

	allowedUsername := strings.TrimPrefix(strings.TrimSpace(os.Getenv("ALLOWED_USERNAME")), "@")
	targetGroupStr := strings.TrimSpace(os.Getenv("TARGET_GROUP_ID"))
	topicStr := strings.TrimSpace(os.Getenv("TOPIC_ID"))

	log.Printf("Bot aktif: @%s", bot.Self.UserName)
	log.Printf("📌 Config Target Group ID: %s", targetGroupStr)
	log.Printf("📌 Config Topic ID     : %s", topicStr)
	log.Printf("🔒 Allowed Username    : @%s", allowedUsername)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if !update.Message.Chat.IsPrivate() {
			continue
		}

		chatID := update.Message.Chat.ID
		senderUsername := update.Message.From.UserName

		if allowedUsername != "" && !strings.EqualFold(senderUsername, allowedUsername) {
			log.Printf("Akses DM ditolak untuk user: @%s (ID: %d)", senderUsername, update.Message.From.ID)
			msg := tgbotapi.NewMessage(chatID, "⛔ <b>Akses Ditolak.</b>\nBot ini dikunci khusus untuk Owner.")
			msg.ParseMode = "HTML"
			bot.Send(msg)
			continue
		}

		if update.Message.Document != nil {
			doc := update.Message.Document

			if strings.HasSuffix(strings.ToLower(doc.FileName), ".ipa") {
				statusMsg, _ := bot.Send(tgbotapi.NewMessage(chatID, "⏳ Menerima file IPA, mengekstrak Metadata & Icon..."))

				os.MkdirAll("./downloads", os.ModePerm)
				localPath := filepath.Join("./downloads", doc.FileName)

				err := fetchTelegramFile(bot, doc.FileID, localPath)
				if err != nil {
					log.Printf("Error fetch file: %v", err)
					bot.Send(tgbotapi.NewMessage(chatID, "❌ Gagal mengunduh file IPA dari Telegram. Pastikan Local Bot API Server aktif."))
					continue
				}

				captionToUse := update.Message.Caption
				if strings.TrimSpace(captionToUse) != "" {
					activeCaptions[chatID] = captionToUse
				} else {
					captionToUse = activeCaptions[chatID]
				}

				processAndSendIPA(bot, chatID, localPath, captionToUse, statusMsg.MessageID)
			}
			continue
		}

		if update.Message.Text != "" {
			text := update.Message.Text

			if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
				lines := strings.Split(text, "\n")
				fileURL := strings.TrimSpace(lines[0])

				userCaption := ""
				if len(lines) > 1 {
					userCaption = strings.Join(lines[1:], "\n")
				}

				if strings.Contains(strings.ToLower(fileURL), ".ipa") || strings.Contains(strings.ToLower(fileURL), "download") {
					statusMsg, _ := bot.Send(tgbotapi.NewMessage(chatID, "⏳ Mengunduh IPA dari Direct URL & mengekstrak Metadata..."))

					os.MkdirAll("./downloads", os.ModePerm)
					fileName := filepath.Base(fileURL)
					if idx := strings.Index(fileName, "?"); idx != -1 {
						fileName = fileName[:idx]
					}
					if !strings.HasSuffix(strings.ToLower(fileName), ".ipa") {
						fileName = "downloaded_app.ipa"
					}

					localPath := filepath.Join("./downloads", fileName)
					err := downloadFile(fileURL, localPath)

					if err != nil {
						bot.Send(tgbotapi.NewMessage(chatID, "❌ Gagal mengunduh file IPA dari URL tersebut."))
						continue
					}

					processAndSendIPA(bot, chatID, localPath, userCaption, statusMsg.MessageID)
					continue
				}
			}

			if text == "/start" {
				welcomeText := fmt.Sprintf(
					"👋 Halo <b>%s</b>!\n\n"+
						"Bot aktif dalam mode <b>Owner Only</b> & siap mengirim ke Topic Grup.\n\n"+
						"💡 <b>Tips Deskripsi/Caption:</b>\n"+
						"• Kirim deskripsi sekali, lalu kirim/forward file <code>.ipa</code> (bisa bulk/banyak sekaligus).\n"+
						"• <code>Name: NamaTambahan</code> ➔ Menambah teks nama di akhir file IPA\n"+
						"• <code>Modified: Fitur Mod</code> ➔ Menampilkan baris Modified & disisipkan ke nama file\n"+
						"• <code>Modified by: NamaKamu</code> ➔ Menampilkan baris Modified by\n"+
						"• Ketik <code>patched</code> ➔ Menampilkan baris <b>Patched</b>\n"+
						"• Ketik <code>Tweaked by: @username</code> ➔ Menampilkan <b>Tweaked by</b>\n"+
						"• Ketik <code>Note: Catatan kamu</code> ➔ Menampilkan blok <b>Note</b>\n\n"+
						"🗑️ <b>Hapus App Catalog:</b>\n"+
						"• <code>/delete Elevate</code> atau <code>/delete com.symptom.tracker</code>",
					update.Message.From.FirstName,
				)
				msg := tgbotapi.NewMessage(chatID, welcomeText)
				msg.ParseMode = "HTML"
				bot.Send(msg)
				continue
			}

			if strings.HasPrefix(text, "/delete") || strings.HasPrefix(text, "/hapus") {
				parts := strings.SplitN(text, " ", 2)
				if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
					targetApp := strings.TrimSpace(parts[1])
					deletedName, err := DeleteFromCatalog(targetApp)
					if err != nil {
						msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("❌ %v", err))
						bot.Send(msg)
					} else {
						msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("🗑️ <b>%s</b> berhasil dihapus dari Web Catalog & GitHub!", deletedName))
						msg.ParseMode = "HTML"
						bot.Send(msg)
					}
				} else {
					msg := tgbotapi.NewMessage(chatID, "⚠️ <b>Format salah.</b>\nGunakan: <code>/delete Nama Aplikasi</code> atau <code>/delete BundleID</code>")
					msg.ParseMode = "HTML"
					bot.Send(msg)
				}
				continue
			}

			activeCaptions[chatID] = text
			msg := tgbotapi.NewMessage(chatID, "📝 <b>Deskripsi diperbarui & aktif!</b>\nSilakan kirim file <code>.ipa</code> kamu.")
			msg.ParseMode = "HTML"
			bot.Send(msg)
		}
	}
}

func fetchTelegramFile(bot *tgbotapi.BotAPI, fileID string, dst string) error {
	fileConfig := tgbotapi.FileConfig{FileID: fileID}
	file, err := bot.GetFile(fileConfig)
	if err != nil {
		return err
	}

	if _, err := os.Stat(file.FilePath); err == nil {
		return copyFile(file.FilePath, dst)
	}

	fileURL := file.Link(bot.Token)
	return downloadFile(fileURL, dst)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func processAndSendIPA(bot *tgbotapi.BotAPI, userChatID int64, localPath string, userCaption string, statusMsgID int) {
	meta, _ := ExtractIPAMetadata(localPath, userCaption)
	captionText := BuildIPACaption(meta)
	iconPath := GetAppIconThumbnail(meta, localPath, "./downloads")

	finalFilePath := localPath
	if meta.FinalFileName != "" {
		renamedPath := filepath.Join("./downloads", meta.FinalFileName)
		if err := os.Rename(localPath, renamedPath); err == nil {
			finalFilePath = renamedPath
		}
	}

	targetChatID := userChatID
	targetGroupStr := strings.TrimSpace(os.Getenv("TARGET_GROUP_ID"))
	if targetGroupStr != "" {
		if gID, err := strconv.ParseInt(targetGroupStr, 10, 64); err == nil && gID != 0 {
			targetChatID = gID
			log.Printf("[SEND] Menargetkan Group ID: %d", targetChatID)
		}
	}

	topicID := 0
	topicStr := strings.TrimSpace(os.Getenv("TOPIC_ID"))
	if topicStr != "" {
		if tID, err := strconv.Atoi(topicStr); err == nil && tID != 0 {
			topicID = tID
			log.Printf("[SEND] Menargetkan Topic ID: %d", topicID)
		}
	}

	sendErr := sendDocumentDirectly(bot.Token, targetChatID, topicID, finalFilePath, captionText, iconPath)
	if sendErr == nil {
		go SaveAndSyncCatalog(meta, iconPath)

		if statusMsgID != 0 {
			bot.Send(tgbotapi.NewDeleteMessage(userChatID, statusMsgID))
		}
		if targetChatID != userChatID {
			bot.Send(tgbotapi.NewMessage(userChatID, fmt.Sprintf("✅ <b>%s</b> berhasil dirilis ke Topic Grup & Web Catalog!", meta.AppName)))
		}
	} else {
		log.Printf("❌ ERROR pengiriman: %v", sendErr)
		errMsg := fmt.Sprintf("❌ <b>Gagal mengirim ke grup!</b>\n\n<code>Error Telegram API: %v</code>", sendErr)
		errSendMsg := tgbotapi.NewMessage(userChatID, errMsg)
		errSendMsg.ParseMode = "HTML"
		bot.Send(errSendMsg)
	}

	os.Remove(finalFilePath)
	if iconPath != "" {
		os.Remove(iconPath)
	}
}

func sendDocumentDirectly(botToken string, chatID int64, topicID int, filePath string, caption string, thumbPath string) error {
	localAPI := strings.TrimRight(os.Getenv("LOCAL_API_URL"), "/")
	var apiURL string
	if localAPI != "" {
		if strings.Contains(localAPI, "%s") {
			apiURL = fmt.Sprintf(localAPI, botToken, "sendDocument")
		} else {
			apiURL = fmt.Sprintf("%s/bot%s/sendDocument", localAPI, botToken)
		}
	} else {
		apiURL = fmt.Sprintf("https://api.telegram.org/bot%s/sendDocument", botToken)
	}

	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)

	go func() {
		defer pipeWriter.Close()
		defer writer.Close()

		_ = writer.WriteField("chat_id", strconv.FormatInt(chatID, 10))
		if topicID != 0 {
			_ = writer.WriteField("message_thread_id", strconv.Itoa(topicID))
		}
		if caption != "" {
			_ = writer.WriteField("caption", caption)
			_ = writer.WriteField("parse_mode", "HTML")
		}

		docFile, err := os.Open(filePath)
		if err != nil {
			return
		}
		defer docFile.Close()

		part, err := writer.CreateFormFile("document", filepath.Base(filePath))
		if err != nil {
			return
		}
		_, _ = io.Copy(part, docFile)

		if thumbPath != "" {
			if thumbFile, err := os.Open(thumbPath); err == nil {
				if thumbPart, err := writer.CreateFormFile("thumbnail", filepath.Base(thumbPath)); err == nil {
					_, _ = io.Copy(thumbPart, thumbFile)
				}
				thumbFile.Close()
			}
		}
	}()

	req, err := http.NewRequest("POST", apiURL, pipeReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{
		Timeout: 30 * time.Minute,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result struct {
		Ok          bool   `json:"ok"`
		Description string `json:"description"`
	}
	_ = json.Unmarshal(respBody, &result)

	if !result.Ok {
		return fmt.Errorf(result.Description)
	}

	return nil
}
