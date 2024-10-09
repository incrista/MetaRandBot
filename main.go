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
	/*
		// load env variables from .env file
		err := godotenv.Load()
		if err != nil {
			log.Fatalf("Error loading .env file")
		}
	*/

	// get bot token from env variable
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	// webhookURL := os.Getenv("WEBHOOK_URL") // e.g. "https://yourdomain.com:8080/<TOKEN>"
	if botToken == "" { // if botToken == "" || webhookURL == "" {
		log.Fatalf("Telegram bot token not found in environment variables")
	}

	rand.Seed(time.Now().UnixNano())

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}
	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	/*
		// Set webhook
		webhookConfig, err := tgbotapi.NewWebhook(webhookURL)
		if err != nil {
			log.Fatalf("Failed to create webhook: %s", err)
		}

		_, err = bot.Request(webhookConfig)
		if err != nil {
			log.Fatalf("Error setting webhook: %s", err)
		}

		// Get webhook info to confirm it's set up correctly
		info, err := bot.GetWebhookInfo()
		if err != nil {
			log.Fatalf("Error getting webhook info: %s", err)
		}

		if info.LastErrorDate != 0 {
			log.Printf("Telegram webhook setup error: %s", info.LastErrorMessage)
		}

		// Start the HTTP server to listen for webhook calls
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			update, err := bot.HandleUpdate(r)
			if err != nil {
				log.Println("Failed to parse update: ", err)
				http.Error(w, "Invalid request", http.StatusBadRequest)
				return
			}

			if update.Message != nil { // Check if message is not nil
				if update.Message.Document != nil {
					// Handle document file
					if update.Message.Document.MimeType[:5] == "video" {
						fileID := update.Message.Document.FileID
						file, err := bot.GetFileDirectURL(fileID)
						if err != nil {
							log.Println("Failed to get file: ", err)
							return
						}

						// download file locally
						inputFilePath := fmt.Sprintf("input_%d.mp4", update.Message.MessageID)
						outputFilePath := fmt.Sprintf("output_%d.mp4", update.Message.MessageID)

						err = downloadFile(inputFilePath, file)
						if err != nil {
							log.Println("Failed to download video: ", err)
							return
						}

						err = RandomizeVideoMetadata(inputFilePath, outputFilePath)
						if err != nil {
							log.Println("Error randomizing metadata: ", err)
							return
						}

						// return altered video
						mediaFile := tgbotapi.NewDocument(update.Message.Chat.ID, tgbotapi.FilePath(outputFilePath))
						_, err = bot.Send(mediaFile)
						if err != nil {
							log.Println("Error sending modified file: ", err)
						}

						// clean up
						os.Remove(inputFilePath)
						os.Remove(outputFilePath)
					} else {
						// if the file is not a video
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please send a valid video file (mp4, mkv, avi).")
						bot.Send(msg)
					}
				}
			}

			// Respond to the request
			w.WriteHeader(http.StatusOK)
		})
	*/

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {

			if update.Message.IsCommand() {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

				switch update.Message.Command() {
				case "healthz":
					msg.Text = "https://metarandbot.onrender.com/healthz\nWorking OK :)"
				default:
					msg.Text = "I don't know that command"
				}
				if _, err := bot.Send(msg); err != nil {
					log.Panic(err)
				}
			}

			if update.Message.Document != nil {
				// handle document file
				if update.Message.Document.MimeType[:5] == "video" {
					fileID := update.Message.Document.FileID
					file, err := bot.GetFileDirectURL(fileID)
					if err != nil {
						log.Println("Failed to get file: ", err)
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unable to find file in telegram servers. Please send the file again. [ERR: 01]")
						bot.Send(msg)
						return
					}

					// download file locally
					inputFilePath := fmt.Sprintf("input_%d.mp4", update.Message.MessageID)
					outputFilePath := fmt.Sprintf("output_%d.mp4", update.Message.MessageID)

					err = downloadFile(inputFilePath, file)
					if err != nil {
						log.Println("Failed to download video: ", err)
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unable to download. Please send the file again.[ERR: 02]")
						bot.Send(msg)
						return
					}

					err = RandomizeVideoMetadata(inputFilePath, outputFilePath)
					if err != nil {
						log.Println("Error randomizing metadata: ", err)
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unable to alter metadata. Please send the file again.[ERR: 03]")
						bot.Send(msg)
						return
					}

					// return altered video
					mediaFile := tgbotapi.NewDocument(update.Message.Chat.ID, tgbotapi.FilePath(outputFilePath))
					_, err = bot.Send(mediaFile)
					if err != nil {
						log.Println("Error sending modified file: ", err)
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Unable to send altered file. Please send the file again.[ERR: 04]")
						bot.Send(msg)
					}

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

	// endpoint for health checks
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Working OK :)"))
	})

	// start server on 0.0.0.0:443
	port := os.Getenv("PORT")
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
