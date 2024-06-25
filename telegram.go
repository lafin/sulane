package main

import (
	"context"
	"log"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SendTelegramMessage - send telegram message
func SendTelegramMessage(ctx context.Context, message string) {
	bot := GetAnyArgFromContext(ctx, "bot").(*tgbotapi.BotAPI)
	if bot != nil {
		chatID, err := strconv.ParseInt(GetStringArgFromContext(ctx, "chat_id"), 10, 64)
		if err != nil {
			log.Panic(err)
		}
		_, err = bot.Send(tgbotapi.NewMessage(chatID, message))
		if err != nil {
			log.Panic(err)
		}
	}
}
