package main

import (
	"context"
	"log"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// redactTelegramError renders err with every occurrence of token replaced by
// "***". Telegram library errors embed the request URL, which carries the bot
// token, so no telegram error may be formatted verbatim.
func redactTelegramError(err error, token string) string {
	if err == nil {
		return ""
	}
	if token == "" {
		return err.Error()
	}
	return strings.ReplaceAll(err.Error(), token, "***")
}

// SendTelegramMessage - send telegram message; a no-op when no bot is configured.
// Telegram problems are reported but never abort the caller.
func SendTelegramMessage(ctx context.Context, message string) {
	bot, ok := GetAnyArgFromContext(ctx, keyBot).(*tgbotapi.BotAPI)
	if !ok || bot == nil {
		return
	}
	chatID, err := strconv.ParseInt(GetStringArgFromContext(ctx, keyChatID), 10, 64)
	if err != nil {
		log.Printf("telegram: invalid chat id: %v", err)
		return
	}
	if _, err := bot.Send(tgbotapi.NewMessage(chatID, message)); err != nil {
		log.Printf("telegram: send message: %s", redactTelegramError(err, bot.Token))
	}
}
