// Package start implements the /start onboarding handler.
package start

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/capcom6/phone2tg-proxy/internal/bot/handler"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/go-core-fx/telegofx"
	"github.com/go-core-fx/telegofx/fsm"
	tgpred "github.com/go-core-fx/telegofx/predicates"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

// Handler is the /start flow: welcome message, contact request, contact
// storage.
type Handler struct {
	storage    storage.Service
	translator *i18n.Service
	fsm        *fsm.Store
}

// New constructs the start handler. It returns handler.Handler - not the
// concrete type - because dig keys value groups by the EXACT result type: a
// concrete return type would make the []handler.Handler slice resolve empty
// with no error.
func New(
	store *fsm.Store,
	storage storage.Service,
	translator *i18n.Service,
) handler.Handler {
	return &Handler{
		storage:    storage,
		translator: translator,
		fsm:        store,
	}
}

// Register mounts the start routes DIRECTLY on the router (the owner's
// reference pattern - no central registrar). telegohandler dispatch is
// first-match-wins over registration order, so the /start command route and
// the contact route must precede the plain-text fall-through. Every chain
// starts with the cheap private-chat guard (OD-SC4(a)) because each
// predicate-bearing route match pays an update.Clone() before evaluating
// predicates; fsm.StateEqual then reads the FSM state inside the predicate.
func (h *Handler) Register(router *telegofx.Router) {
	guard := tgpred.MessageWithChatType(telego.ChatTypePrivate)

	router.Handle(
		h.handleStart,
		guard,
		fsm.StateEqual(h.fsm, fsm.StateEmpty),
		th.CommandEqual("start"),
	)
	router.Handle(
		h.handleContact,
		guard,
		fsm.StateEqual(h.fsm, handlers.StateStartWaitForContact),
		tgpred.MessageWithContact(),
	)
	router.Handle(
		h.handleText,
		guard,
		fsm.StateEqual(h.fsm, handlers.StateStartWaitForContact),
		th.Not(th.AnyCommand()),
	)
}

// handleStart enters the wait-for-contact state and sends the welcome with
// the share-contact keyboard. The route predicates guarantee a message with
// a sender.
func (h *Handler) handleStart(ctx *th.Context, update telego.Update) error {
	msg := update.Message

	if err := h.fsm.Set(ctx, msg.Chat.ID, msg.From.ID, handlers.StateStartWaitForContact); err != nil {
		return fmt.Errorf("set fsm state: %w", err)
	}

	if sendErr := handlers.Send(
		ctx,
		msg.Chat.ID,
		h.translator.Translate("welcome_message"),
		h.makeShareContactKeyboard(),
	); sendErr != nil {
		return fmt.Errorf("send welcome: %w", sendErr)
	}
	return nil
}

// handleContact stores the shared contact and clears the FSM state.
func (h *Handler) handleContact(ctx *th.Context, update telego.Update) error {
	contact := update.Message.Contact
	if contact == nil {
		if sendErr := handlers.Send(
			ctx,
			update.Message.Chat.ID,
			h.translator.Translate("send_contact"),
			h.makeShareContactKeyboard(),
		); sendErr != nil {
			return fmt.Errorf("send contact prompt: %w", sendErr)
		}
		return nil
	}

	// SC-2: the contact-ownership check stays in the handler BODY on
	// purpose. As a Predicate it would degrade to a silent false and the
	// user would get no reply at all; here the other user's contact is
	// actively rejected with the share_contact text.
	if contact.UserID != update.Message.From.ID {
		if sendErr := handlers.Send(
			ctx,
			update.Message.Chat.ID,
			h.translator.Translate("share_contact"),
			h.makeShareContactKeyboard(),
		); sendErr != nil {
			return fmt.Errorf("send share contact rejection: %w", sendErr)
		}
		return nil
	}

	// OD-B-style context: the handler's context must not reach
	// fsm/storage - storage ops keep the fixed background+1s budget.
	storeCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := h.storage.Store(storeCtx, contact.PhoneNumber, update.Message.Chat.ID); err != nil {
		if errors.Is(err, storage.ErrInvalidPhoneNumber) {
			if sendErr := handlers.Send(
				ctx, update.Message.Chat.ID, h.translator.Translate("invalid_phone"), nil,
			); sendErr != nil {
				return fmt.Errorf("send invalid phone: %w", sendErr)
			}
			return nil
		}
		return fmt.Errorf("set phone number: %w", err)
	}

	if err := h.fsm.Clear(ctx, update.Message.Chat.ID, update.Message.From.ID); err != nil {
		return fmt.Errorf("clear fsm state: %w", err)
	}

	if sendErr := handlers.Send(
		ctx,
		update.Message.Chat.ID,
		h.translator.Translate("thanks_contact"),
		&telego.ReplyKeyboardRemove{RemoveKeyboard: true},
	); sendErr != nil {
		return fmt.Errorf("send contact thanks: %w", sendErr)
	}
	return nil
}

// handleText replies "send contact" to any non-command text while waiting
// for the contact. Commands never reach it: th.Not(th.AnyCommand()) rejects
// them.
func (h *Handler) handleText(ctx *th.Context, update telego.Update) error {
	if sendErr := handlers.Send(
		ctx,
		update.Message.Chat.ID,
		h.translator.Translate("send_contact"),
		h.makeShareContactKeyboard(),
	); sendErr != nil {
		return fmt.Errorf("send contact prompt: %w", sendErr)
	}
	return nil
}

func (h *Handler) makeShareContactKeyboard() *telego.ReplyKeyboardMarkup {
	return &telego.ReplyKeyboardMarkup{
		Keyboard: [][]telego.KeyboardButton{
			{{Text: h.translator.Translate("share_contact_button"), RequestContact: true}},
		},
		ResizeKeyboard:  true,
		OneTimeKeyboard: true,
	}
}
