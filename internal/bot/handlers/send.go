package handlers

import (
	"fmt"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

// Send replies to the chat the update arrived in. A nil markup leaves the
// reply_markup field off the request entirely. The th.Context is the
// handler's context: it is alive for the whole handler and canceled once
// the update finishes, so the API call is bounded by the update's lifetime.
// It is exported because the start and stop subpackages both reply through it.
func Send(ctx *th.Context, chatID int64, text string, markup telego.ReplyMarkup) error {
	_, err := ctx.Bot().SendMessage(ctx.Context(), &telego.SendMessageParams{
		ChatID:      telego.ChatID{ID: chatID},
		Text:        text,
		ReplyMarkup: markup,
	})
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}

	return nil
}
