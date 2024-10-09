package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/joho/godotenv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func RandomizeVideoMetadata(inputFilePath, outputFilePath string) error {

	randomTitle := fmt.Sprintf("RandomTitle%d", rand.Intn(100))
	randomGenre := fmt.Sprintf("Genre%d", rand.Intn(100))
	randomDate := time.Now().AddDate(-rand.Intn(5), -rand.Intn(12), -rand.Intn(28)).Format("2006-01-02")

	cmd := exec.Command("ffmpeg", "-i", inputFilePath, "-metadata", fmt.Sprintf("title=%s", randomTitle),
		"-metadata", fmt.Sprintf("genre=%s", randomGenre),
		"-metadata", fmt.Sprintf("date=%s", randomDate), "-codec", "copy", outputFilePath)

	return cmd.Run()
}

func main() {
	// load env variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	// get bot token from env variable
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if botToken == "" {
		log.Fatalf("Telegram bot token not found in environment variables")
	}

	rand.Seed(time.Now().UnixNano())

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			if update.Message.Document != nil {

				if update.Message.Document.MimeType[:5] == "video" {
					fileID := update.Message.Document.FileID
					file, err := bot.GetFileDirectURL(fileID)
					if err != nil {
						log.Println("Failed to get file: ", err)
						continue
					}

					// download original file
					inputFilePath := fmt.Sprintf("input_%d.mp4", update.Message.MessageID)
					outputFilePath := fmt.Sprintf("output_%d.mp4", update.Message.MessageID)

					err = downloadFile(inputFilePath, file)
					if err != nil {
						log.Println("Failed to download video: ", err)
						continue
					}

					err = RandomizeVideoMetadata(inputFilePath, outputFilePath)
					if err != nil {
						log.Println("Error randomizing metadata: ", err)
						continue
					}

					// send altered file to user
					mediaFile := tgbotapi.NewDocument(update.Message.Chat.ID, tgbotapi.FilePath(outputFilePath))
					_, err = bot.Send(mediaFile)
					if err != nil {
						log.Println("Error sending modified file: ", err)
					}

					// clean up after sending
					os.Remove(inputFilePath)
					os.Remove(outputFilePath)
				} else {
					// if file not mp4 video
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please send a valid video file (.mp4).")
					bot.Send(msg)
				}
			}
		}
	}
}

// download and save file locally
func downloadFile(filepath string, url string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// fetch
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// write to file
	_, err = io.Copy(out, resp.Body)
	return err
}
