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

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

/*
func RandomizeVideoMetadata(inputFilePath, outputFilePath string) error {

	randomTitle := fmt.Sprintf("RandomTitle%d", rand.Intn(100))
	randomGenre := fmt.Sprintf("Genre%d", rand.Intn(100))
	randomDate := time.Now().AddDate(-rand.Intn(5), -rand.Intn(12), -rand.Intn(28)).Format("2006-01-02")

	cmd := exec.Command("ffmpeg", "-i", inputFilePath, "-metadata", fmt.Sprintf("title=%s", randomTitle),
		"-metadata", fmt.Sprintf("genre=%s", randomGenre),
		"-metadata", fmt.Sprintf("date=%s", randomDate), "-codec", "copy", outputFilePath)

	return cmd.Run()
}
*/

func RandomizeVideoMetadata(inputFilePath, outputFilePath string) error {

	randomCreationTime := time.Now().AddDate(-rand.Intn(5), -rand.Intn(12), -rand.Intn(28)).Format("2006-01-02T15:04:05")
	randomModifyTime := time.Now().Add(time.Duration(rand.Intn(86400)) * time.Second).Format("2006-01-02T15:04:05")

	cmd := exec.Command("ffmpeg", "-i", inputFilePath,
		"-metadata", fmt.Sprintf("creation_time=%s", randomCreationTime),
		"-metadata", fmt.Sprintf("modification_time=%s", randomModifyTime),
		"-codec", "copy", outputFilePath)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to randomize media timestamps: %v", err)
	}

	log.Printf("Media timestamps randomized successfully: creation_time=%s, modification_time=%s", randomCreationTime, randomModifyTime)
	return nil
}

func main() {
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

	go func() {
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60

		updates := bot.GetUpdatesChan(u)

		for update := range updates {
			if update.Message != nil {

				if update.Message.IsCommand() {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

					switch update.Message.Command() {
					case "healthz":
						msg.Text = "https://metarandbot.onrender.com\nWorking OK :)"
					default:
						msg.Text = "I don't know that command"
					}
					if _, err := bot.Send(msg); err != nil {
						log.Panic(err)
					}
				}

				if update.Message.Document != nil {
					// handle video file
					if update.Message.Document.MimeType[:5] == "video" {
						fileID := update.Message.Document.FileID
						file, err := bot.GetFileDirectURL(fileID)
						if err != nil {
							log.Println("Failed to get file: ", err)
							msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unable to find file in telegram servers. Please send the file again. [ERR: 01]")
							bot.Send(msg)
							continue
						}

						message := fmt.Sprintf("Received File: %s", fileID)
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, message)
						bot.Send(msg)

						// download file locally
						inputFilePath := fmt.Sprintf("input_%d.mp4", update.Message.MessageID)
						outputFilePath := fmt.Sprintf("output_%d.mp4", update.Message.MessageID)

						err = downloadFile(inputFilePath, file)
						if err != nil {
							log.Println("Failed to download video: ", err)
							msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unable to download. Please send the file again. [ERR: 02]")
							bot.Send(msg)
							continue
						}

						err = RandomizeVideoMetadata(inputFilePath, outputFilePath)
						if err != nil {
							log.Println("Error randomizing metadata: ", err)
							msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unable to alter metadata. Please send the file again. [ERR: 03]")
							bot.Send(msg)
							continue
						}

						// return altered video
						retryLimit := 3
						retryCount := 0
						for retryCount < retryLimit {
							mediaFile := tgbotapi.NewDocument(update.Message.Chat.ID, tgbotapi.FilePath(outputFilePath))
							_, err = bot.Send(mediaFile)
							if err != nil {
								retryCount++
								log.Printf("Error sending modified file (attempt %d): %s", retryCount, err)

								time.Sleep(2 * time.Second)

								if retryCount == retryLimit {
									msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unable to send altered file after multiple attempts. Please send the file again. [ERR: 04]")
									bot.Send(msg)
								}
							} else {
								log.Println("Successfully sent the modified file.")
								break
							}
						}

						/*mediaFile := tgbotapi.NewDocument(update.Message.Chat.ID, tgbotapi.FilePath(outputFilePath))
						_, err = bot.Send(mediaFile)
						if err != nil {
							log.Println("Error sending modified file: ", err)
							msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unable to send altered file. Please send the file again. [ERR: 04]")
							bot.Send(msg)
						}
						*/

						// clean up
						os.Remove(inputFilePath)
						os.Remove(outputFilePath)
					} else {
						// if the file is not a video
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please send a valid .mp4 video file.")
						bot.Send(msg)
					}
				}
			}
		}
	}()

	// health check
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Working OK :)"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "443"
	}

	log.Printf("Starting server on port %s", port)

	err = http.ListenAndServe("0.0.0.0:"+port, nil)
	if err != nil {
		log.Fatalf("Failed to start server: %s", err)
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
