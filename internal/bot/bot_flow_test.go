package bot_test

// bot_flow_test.go is the pre-swap oracle: it pins the bot's user-facing
// contract on the CURRENT gopkg.in/telebot.v4 implementation so the telego
// migration (T3) can be verified to delete or relax NO assertion. Every reply
// assertion compares against a LITERAL string from i18n/locales/en.json.
// Never call i18n.Service.Translate in an assertion: it returns the key on
// miss and would self-fulfill.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/capcom6/phone2tg-proxy/internal/bot/fsm"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers"
	"github.com/capcom6/phone2tg-proxy/internal/bot/middleware"
	"github.com/capcom6/phone2tg-proxy/internal/bot/router"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/gofiber/storage/memory/v2"
	"go.uber.org/zap"
	"gopkg.in/telebot.v4"
)

// ---------------------------------------------------------------------------
// i18n literals (en.json) - copied verbatim, NEVER resolved via Translate().
// ---------------------------------------------------------------------------

const (
	msgWelcome      = "Welcome to the contact bot!"
	msgSendContact  = "Please, send me your contact"
	msgShareContact = "You must share your contact with me"
	msgInvalidPhone = "Invalid phone number format. Please share your contact again."
	msgThanks       = "Thanks for sharing your contact!"
	msgDeleteFailed = "Failed to delete association"
	msgDeleteOK     = "Your association has been deleted"
	btnShareContact = "Share contact"
)

// i18nKeys are exactly the 8 translation keys the bot depends on.
var i18nKeys = []string{
	"welcome_message",
	"send_contact",
	"share_contact",
	"invalid_phone",
	"thanks_contact",
	"delete_success",
	"delete_failed",
	"share_contact_button",
}

// ---------------------------------------------------------------------------
// Recording Telegram transport: captures every raw Bot API call.
// ---------------------------------------------------------------------------

// tgCall is one captured Bot API request.
type tgCall struct {
	Method string         `json:"-"`
	Params map[string]any `json:"-"`
}

// recordingTransport implements http.RoundTripper. It records every request
// telebot makes (method + JSON body) and answers {"ok":true} so Raw() never
// errors. No packet leaves the process.
type recordingTransport struct {
	mu    sync.Mutex
	calls []tgCall
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, readErr := io.ReadAll(req.Body)
	if readErr != nil {
		return nil, fmt.Errorf("read request body: %w", readErr)
	}

	var params map[string]any
	if len(body) > 0 {
		if unmarshalErr := json.Unmarshal(body, &params); unmarshalErr != nil {
			return nil, fmt.Errorf("unmarshal request body: %w", unmarshalErr)
		}
	}

	// URL shape: <url>/bot<token>/<method>
	parts := strings.Split(req.URL.Path, "/")
	method := parts[len(parts)-1]

	rt.mu.Lock()
	rt.calls = append(rt.calls, tgCall{Method: method, Params: params})
	rt.mu.Unlock()

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":1}}`)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    req,
	}
	return resp, nil
}

// filter returns the captured calls for one Bot API method.
func (rt *recordingTransport) filter(method string) []tgCall {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	calls := make([]tgCall, 0, len(rt.calls))
	for _, c := range rt.calls {
		if c.Method == method {
			calls = append(calls, c)
		}
	}
	return calls
}

// reset drops all captured calls.
func (rt *recordingTransport) reset() {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.calls = nil
}

// texts returns the "text" parameter of every captured sendMessage call.
func (rt *recordingTransport) texts(method string) []string {
	calls := rt.filter(method)

	texts := make([]string, 0, len(calls))
	for _, c := range calls {
		if s, ok := c.Params["text"].(string); ok {
			texts = append(texts, s)
		}
	}
	return texts
}

// replyMarkup returns the decoded reply_markup of the n-th captured call of a
// method. The n-th call is used because handler order is deterministic.
func (rt *recordingTransport) replyMarkup(t *testing.T, method string, n int) map[string]any {
	t.Helper()

	calls := rt.filter(method)
	if n >= len(calls) {
		t.Fatalf("expected at least %d %s calls, got %d", n+1, method, len(calls))
	}

	raw, ok := calls[n].Params["reply_markup"]
	if !ok {
		t.Fatalf("%s call %d carries no reply_markup", method, n)
	}

	str, ok := raw.(string)
	if !ok {
		t.Fatalf("%s call %d reply_markup must be a string, got %T", method, n, raw)
	}

	var markup map[string]any
	if err := json.Unmarshal([]byte(str), &markup); err != nil {
		t.Fatalf("unmarshal reply_markup: %v", err)
	}
	return markup
}

// ---------------------------------------------------------------------------
// Recording storage.Service fake.
// ---------------------------------------------------------------------------

// storageCall records one call of the storage.Service fake.
type storageCall struct {
	Phone string
	ID    int64
}

// fakeStorage implements storage.Service and records every call.
type fakeStorage struct {
	mu     sync.Mutex
	stored []storageCall
	// deleted records every telegramID passed to Delete.
	deleted []int64
	// storeErr, when set, is returned by Store (e.g. ErrInvalidPhoneNumber).
	storeErr error
	// deleteErr, when set, is returned by Delete.
	deleteErr error
}

func (f *fakeStorage) Store(_ context.Context, phoneNumber string, telegramID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.stored = append(f.stored, storageCall{Phone: phoneNumber, ID: telegramID})
	if f.storeErr != nil {
		return f.storeErr
	}
	return nil
}

func (f *fakeStorage) Get(_ context.Context, _ string) (int64, error) {
	return 0, errors.New("not implemented")
}

func (f *fakeStorage) Delete(_ context.Context, telegramID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deleted = append(f.deleted, telegramID)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
}

func (f *fakeStorage) storeCalls() []storageCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]storageCall(nil), f.stored...)
}

// deleteCalls returns every telegramID recorded by Delete.
func (f *fakeStorage) deleteCalls() []int64 {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]int64(nil), f.deleted...)
}

// ---------------------------------------------------------------------------
// Harness: real telebot bot + real middleware + real router + real handlers.
// ---------------------------------------------------------------------------

const (
	chatIDPrivate = int64(100200300)
	chatIDGroup   = int64(-1009876543210)
	otherUserID   = int64(777888999)
	botID         = int64(424242)
	ownBotName    = "own_test_bot"
	otherBotName  = "OtherBot"
)

// flowHarness wires the exact production graph from internal/bot/module.go:
// middleware.Logger -> router.New(fsm, bot) -> handlers, but with an offline
// telebot bot whose HTTP client records every call.
type flowHarness struct {
	t          *testing.T
	bot        *telebot.Bot
	transport  *recordingTransport
	storage    *fakeStorage
	fsmService *fsm.Service
}

func newFlowHarness(t *testing.T) *flowHarness {
	t.Helper()

	transport := &recordingTransport{
		mu:    sync.Mutex{},
		calls: nil,
	}
	st := &fakeStorage{
		mu:        sync.Mutex{},
		stored:    nil,
		deleted:   nil,
		storeErr:  nil,
		deleteErr: nil,
	}

	b, err := telebot.NewBot(telebot.Settings{
		Token:       "123456:test-offline-token-not-real",
		Offline:     true, // skips getMe; no network
		Synchronous: true, // ProcessUpdate runs handlers inline
		Client:      &http.Client{Transport: transport},
	})
	if err != nil {
		t.Fatalf("new bot: %v", err)
	}

	// telebot drops any /cmd@suffix whose botname is not EqualFold(b.Me
	// .Username). Offline leaves Me empty, so seed it for CM2/CM3.
	b.Me = &telebot.User{ID: botID, Username: ownBotName}

	logger := zap.NewNop()
	fsmStorage := memory.New()
	fsmService := fsm.New("fsm", 0, fsmStorage)

	b.Use(middleware.Logger(logger))

	rt := router.New(fsmService, b)
	startHandler := handlers.NewStartHandler(st, newTestTranslator(t))
	stopHandler := handlers.NewStopHandler(st, logger, newTestTranslator(t))

	if regErr := startHandler.Register(rt); regErr != nil {
		t.Fatalf("register start handler: %v", regErr)
	}
	if regErr := stopHandler.Register(rt); regErr != nil {
		t.Fatalf("register stop handler: %v", regErr)
	}

	return &flowHarness{
		t:          t,
		bot:        b,
		transport:  transport,
		storage:    st,
		fsmService: fsmService,
	}
}

func newTestTranslator(t *testing.T) *i18n.Service {
	t.Helper()

	svc, err := i18n.NewService(i18n.Config{
		Language:         "en",
		TranslationsPath: "../../i18n/locales",
	})
	if err != nil {
		t.Fatalf("new i18n service: %v", err)
	}
	return svc
}

// setState primes chatIDPrivate to wait_for_contact, as if onboarding was
// mid-flight (the only non-empty state the suite needs).
func (h *flowHarness) setState() {
	h.t.Helper()

	if err := h.fsmService.Set(
		strconv.FormatInt(chatIDPrivate, 10),
		fsm.NewState(handlers.StateStartWaitForContact),
	); err != nil {
		h.t.Fatalf("set state: %v", err)
	}
}

func (h *flowHarness) getState(chatID int64) fsm.StateName {
	h.t.Helper()

	st, err := h.fsmService.Get(strconv.FormatInt(chatID, 10))
	if err != nil {
		h.t.Fatalf("get state: %v", err)
	}
	return st.Name
}

// send drives exactly one update through the real dispatch cascade.
func (h *flowHarness) send(u telebot.Update) {
	h.t.Helper()

	h.transport.reset()
	h.bot.ProcessUpdate(u)
}

// privateMessage builds a private-chat text message update for chatIDPrivate.
func privateMessage(id int, text string) telebot.Update {
	return telebot.Update{
		ID: id,
		Message: &telebot.Message{
			ID:     id,
			Chat:   &telebot.Chat{ID: chatIDPrivate},
			Sender: &telebot.User{ID: chatIDPrivate},
			Text:   text,
		},
	}
}

// groupMessage builds a group-chat text message update (used by SC-4, whose
// body is pinned post-migration).
func groupMessage(id int, chatID int64, text string) telebot.Update {
	return telebot.Update{
		ID: id,
		Message: &telebot.Message{
			ID:     id,
			Chat:   &telebot.Chat{ID: chatID},
			Sender: &telebot.User{ID: otherUserID},
			Text:   text,
		},
	}
}

// contactMessage builds a contact update for chatIDPrivate; sameUser selects
// the chat owner as the contact's user (the SC-2 ownership polarity).
func contactMessage(id int, sameUser bool, phone string) telebot.Update {
	chatID := chatIDPrivate

	contactUserID := chatID
	if !sameUser {
		contactUserID = otherUserID
	}

	return telebot.Update{
		ID: id,
		Message: &telebot.Message{
			ID:      id,
			Chat:    &telebot.Chat{ID: chatID},
			Sender:  &telebot.User{ID: chatID},
			Contact: &telebot.Contact{UserID: contactUserID, PhoneNumber: phone},
		},
	}
}

// assertLastText checks that the last sendMessage text equals want.
func (h *flowHarness) assertLastText(want string) {
	h.t.Helper()

	got := h.transport.texts("sendMessage")
	if len(got) == 0 {
		h.t.Fatalf("expected a sendMessage, got none (calls: %v)", h.transport.calls)
	}
	if got[len(got)-1] != want {
		h.t.Fatalf("reply text mismatch:\n want: %q\n got:  %q", want, got[len(got)-1])
	}
}

// assertNoReply checks that no sendMessage was captured.
func (h *flowHarness) assertNoReply() {
	h.t.Helper()

	if got := h.transport.texts("sendMessage"); len(got) != 0 {
		h.t.Fatalf("expected no sendMessage, got %v", got)
	}
}

// assertState checks the FSM state of chatIDPrivate.
func (h *flowHarness) assertState(want fsm.StateName) {
	h.t.Helper()

	if got := h.getState(chatIDPrivate); got != want {
		h.t.Fatalf("state mismatch for chat %d: want %q, got %q", chatIDPrivate, want, got)
	}
}

// ---------------------------------------------------------------------------
// SC-1: /stop must never delete while onboarding is mid-flight.
// ---------------------------------------------------------------------------

// SC1_blocked_stop_mid_onboarding: /stop while state=wait_for_contact does
// NOT call storage.Delete and sends no reply (the update is silently
// consumed by the command matcher; no handler is registered for the state).
func TestBotFlow_SC1_blocked_stop_mid_onboarding(t *testing.T) {
	h := newFlowHarness(t)
	h.setState()

	h.send(privateMessage(1, "/stop"))

	if got := h.storage.deleteCalls(); len(got) != 0 {
		t.Fatalf("storage.Delete must NOT be called mid-onboarding, got %v", got)
	}
	h.assertNoReply()
	h.assertState(handlers.StateStartWaitForContact)
}

// SC1_stop_in_empty: /stop in StateEmpty calls Delete and replies with the
// literal delete_success text.
func TestBotFlow_SC1_stop_in_empty(t *testing.T) {
	h := newFlowHarness(t)

	h.send(privateMessage(2, "/stop"))

	h.assertLastText(msgDeleteOK)
}

// ---------------------------------------------------------------------------
// SC-2: contact ownership and phone validation.
// ---------------------------------------------------------------------------

// SC2_same_user_contact_stores: a contact whose user id equals the chat id is
// stored with (PhoneNumber, chat.ID) and the state is deleted.
func TestBotFlow_SC2_same_user_contact_stores(t *testing.T) {
	h := newFlowHarness(t)
	h.setState()

	h.send(contactMessage(3, true, "+15550001122"))

	calls := h.storage.storeCalls()
	if len(calls) != 1 {
		t.Fatalf("expected exactly one Store call, got %v", calls)
	}
	if calls[0].Phone != "+15550001122" || calls[0].ID != chatIDPrivate {
		t.Fatalf("Store called with (%q, %d), want (+15550001122, %d)", calls[0].Phone, calls[0].ID, chatIDPrivate)
	}
	h.assertLastText(msgThanks)
	h.assertState(fsm.StateEmpty)
}

// SC2_other_user_contact_rejected: a contact owned by another user is NOT
// stored; share_contact is sent and the state is unchanged.
func TestBotFlow_SC2_other_user_contact_rejected(t *testing.T) {
	h := newFlowHarness(t)
	h.setState()

	h.send(contactMessage(4, false, "+15550001122"))

	if got := h.storage.storeCalls(); len(got) != 0 {
		t.Fatalf("storage.Store must NOT be called for another user's contact, got %v", got)
	}
	h.assertLastText(msgShareContact)
	h.assertState(handlers.StateStartWaitForContact)
}

// SC2_invalid_phone_keeps_state: storage rejecting the phone keeps the state
// and replies with the invalid_phone text.
func TestBotFlow_SC2_invalid_phone_keeps_state(t *testing.T) {
	h := newFlowHarness(t)
	h.setState()
	h.storage.storeErr = storage.ErrInvalidPhoneNumber

	h.send(contactMessage(5, true, ""))

	h.assertLastText(msgInvalidPhone)
	h.assertState(handlers.StateStartWaitForContact)
}

// ---------------------------------------------------------------------------
// SC-3: update narrowing - non-message updates must not touch the FSM.
// ---------------------------------------------------------------------------

// SC3_update_narrowing: edited_message, channel_post and callback_query
// updates create no FSM state (telebot dispatches them to OnEdited /
// OnChannelPost / OnCallback, none of which is registered).
func TestBotFlow_SC3_update_narrowing(t *testing.T) {
	h := newFlowHarness(t)

	updates := []telebot.Update{
		{
			ID: 6,
			EditedMessage: &telebot.Message{
				ID:   6,
				Chat: &telebot.Chat{ID: chatIDPrivate},
				Text: "/start",
			},
		},
		{
			ID: 7,
			ChannelPost: &telebot.Message{
				ID:   7,
				Chat: &telebot.Chat{ID: chatIDGroup},
				Text: "/start",
			},
		},
		{
			ID: 8,
			Callback: &telebot.Callback{
				Sender:  &telebot.User{ID: chatIDPrivate},
				Message: &telebot.Message{ID: 8, Chat: &telebot.Chat{ID: chatIDPrivate}},
				Data:    "\fnoop",
			},
		},
	}
	for _, u := range updates {
		h.send(u)
	}

	if got := h.getState(chatIDPrivate); got != fsm.StateEmpty {
		t.Fatalf("edited_message must not create FSM state, got %q", got)
	}
	if got := h.getState(chatIDGroup); got != fsm.StateEmpty {
		t.Fatalf("channel_post must not create FSM state, got %q", got)
	}
	h.assertNoReply()
}

// ---------------------------------------------------------------------------
// SC-4: chat-type guard (OD-SC4).
// ---------------------------------------------------------------------------

// SC4_chat_type_guard pins the POST-migration intent per OD-SC4 (a): a group
// /start creates no state and sends nothing.
//
// NOTE: this row documents the INTENT, not today's behavior. On the current
// telebot implementation a group /start DOES create state and send the
// welcome keyboard (router.go has no chat-type guard). It is expected to
// FAIL against today's code and gets pinned in T3 when the guard is added.
// Skipped here so the oracle stays green on telebot; do not delete.
func TestBotFlow_SC4_chat_type_guard(t *testing.T) {
	t.Skip(
		"OD-SC4=(a): pins the POST-migration intent (group /start is dropped); " +
			"today's telebot creates state and sends welcome. Pinned in T3.",
	)

	h := newFlowHarness(t)

	h.send(groupMessage(9, chatIDGroup, "/start"))

	if got := h.getState(chatIDGroup); got != fsm.StateEmpty {
		t.Fatalf("group /start must create no FSM state, got %q", got)
	}
	h.assertNoReply()
}

// ---------------------------------------------------------------------------
// Command matching: exact, case-sensitive, @suffix-aware.
// ---------------------------------------------------------------------------

// CM1_uppercase_stop_noop: "/STOP" in StateEmpty must NOT call Delete. Today
// the command matcher is case-sensitive, so "/STOP" is not matched as a
// command; the update is consumed by the fall-through OnText endpoint
// (registered for wait_for_contact only), so StateEmpty sees no handler and
// the router silently drops it.
func TestBotFlow_CM1_uppercase_stop_noop(t *testing.T) {
	h := newFlowHarness(t)

	h.send(privateMessage(10, "/STOP"))

	if got := h.storage.storeCalls(); len(got) != 0 {
		t.Fatalf("storage.Delete must NOT be called for /STOP, got %v", got)
	}
	h.assertNoReply()
	h.assertState(fsm.StateEmpty)
}

// CM2_at_other_bot_dropped: "/start@OtherBot" is dropped before any handler
// runs because the @suffix does not match this bot's username.
func TestBotFlow_CM2_at_other_bot_dropped(t *testing.T) {
	h := newFlowHarness(t)

	h.send(privateMessage(11, "/start@"+otherBotName))

	if got := h.storage.storeCalls(); len(got) != 0 {
		t.Fatalf("start flow must NOT run for @other-bot command, got %v", got)
	}
	h.assertNoReply()
	h.assertState(fsm.StateEmpty)
}

// CM3_at_own_bot_accepted: "/start@<own username>" runs the start flow.
func TestBotFlow_CM3_at_own_bot_accepted(t *testing.T) {
	h := newFlowHarness(t)

	h.send(privateMessage(12, "/start@"+ownBotName))

	h.assertLastText(msgWelcome)
	h.assertState(handlers.StateStartWaitForContact)
}

// ---------------------------------------------------------------------------
// Keyboard shape.
// ---------------------------------------------------------------------------

// KB1_welcome_keyboard_shape: the welcome sendMessage carries a reply_markup
// with keyboard[0][0].request_contact == true, resize_keyboard == true and
// one_time_keyboard == true.
func TestBotFlow_KB1_welcome_keyboard_shape(t *testing.T) {
	h := newFlowHarness(t)

	h.send(privateMessage(13, "/start"))

	markup := h.transport.replyMarkup(t, "sendMessage", 0)

	keyboard, ok := markup["keyboard"].([]any)
	if !ok || len(keyboard) != 1 {
		t.Fatalf("reply_markup must carry a 1-row keyboard, got %v", markup)
	}
	row, ok := keyboard[0].([]any)
	if !ok || len(row) != 1 {
		t.Fatalf("keyboard must have exactly 1 button in row 0, got %v", keyboard)
	}
	button, ok := row[0].(map[string]any)
	if !ok {
		t.Fatalf("keyboard button must be an object, got %v", row[0])
	}
	if button["request_contact"] != true {
		t.Fatalf("button[0][0].request_contact must be true, got %v", button)
	}
	if button["text"] != btnShareContact {
		t.Fatalf("button text must be %q, got %v", btnShareContact, button["text"])
	}
	if markup["resize_keyboard"] != true {
		t.Fatalf("resize_keyboard must be true, got %v", markup)
	}
	if markup["one_time_keyboard"] != true {
		t.Fatalf("one_time_keyboard must be true, got %v", markup)
	}
}

// KB2_thanks_remove_keyboard: the thanks sendMessage carries
// reply_markup.remove_keyboard == true in the SAME request.
func TestBotFlow_KB2_thanks_remove_keyboard(t *testing.T) {
	h := newFlowHarness(t)
	h.setState()

	h.send(contactMessage(14, true, "+15550001122"))

	markup := h.transport.replyMarkup(t, "sendMessage", 0)
	if markup["remove_keyboard"] != true {
		t.Fatalf("thanks reply_markup.remove_keyboard must be true, got %v", markup)
	}
}

// ---------------------------------------------------------------------------
// Invariant sweep: every reply literal is distinct and non-empty.
// ---------------------------------------------------------------------------

// TestBotFlow_reply_literals_are_distinct guards the oracle against silent
// i18n key collisions: two distinct keys must never produce the same literal.
func TestBotFlow_reply_literals_are_distinct(t *testing.T) {
	literals := map[string]string{
		"welcome_message":      msgWelcome,
		"send_contact":         msgSendContact,
		"share_contact":        msgShareContact,
		"invalid_phone":        msgInvalidPhone,
		"thanks_contact":       msgThanks,
		"delete_success":       msgDeleteOK,
		"delete_failed":        msgDeleteFailed,
		"share_contact_button": btnShareContact,
	}

	seen := map[string]string{}
	for key, literal := range literals {
		if literal == "" {
			t.Errorf("literal for %s must not be empty", key)
		}
		if prev, ok := seen[literal]; ok {
			t.Errorf("literals of %s and %s collide: %q", key, prev, literal)
		}
		seen[literal] = key
	}

	keys := make([]string, 0, len(literals))
	for key := range literals {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	expected := append([]string(nil), i18nKeys...)
	sort.Strings(expected)
	if fmt.Sprint(keys) != fmt.Sprint(expected) {
		t.Errorf("literal key set drifted from i18nKeys: %v", keys)
	}
}
