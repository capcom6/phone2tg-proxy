package bot_test

// bot_flow_test.go is the pre-swap oracle: it pins the bot's user-facing
// contract so the binder migration (T3) can be verified to delete or relax
// NO assertion. Every reply assertion compares against a LITERAL string from
// i18n/locales/en.json. Never call i18n.Service.Translate in an assertion:
// it returns the key on miss and would self-fulfill.
//
// T3 re-points the drive side from the old ProcessUpdate cascade to
// telegohandler's synchronous HandleUpdate: one update runs inline through
// the real middleware, predicates and dispatcher, with no Start()/channel
// wait anywhere. The 12 assertion IDs and their literals are unchanged.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers/start"
	"github.com/capcom6/phone2tg-proxy/internal/bot/handlers/stop"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/state"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/go-core-fx/telegofx"
	"github.com/go-core-fx/telegofx/fsm"
	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	"go.uber.org/zap"
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

// recordingTransport implements [http.RoundTripper]. It records every request
// the bot makes (method + JSON body) and answers {"ok":true} so Raw() never
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
// telego posts the markup as a JSON object in the request body, so the raw
// parameter arrives already decoded from the transport's JSON unmarshal.
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

	switch markup := raw.(type) {
	case map[string]any:
		return markup
	case string:
		var decoded map[string]any
		if err := json.Unmarshal([]byte(markup), &decoded); err != nil {
			t.Fatalf("unmarshal reply_markup: %v", err)
		}
		return decoded
	default:
		t.Fatalf("%s call %d reply_markup must be a JSON object, got %T", method, n, raw)
		return nil
	}
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
// Harness: real telego bot + real middleware + real router + real handlers.
// ---------------------------------------------------------------------------

const (
	chatIDPrivate = int64(100200300)
	chatIDGroup   = int64(-1009876543210)
	otherUserID   = int64(777888999)
	ownBotName    = "own_test_bot"
	otherBotName  = "OtherBot"

	// offlineToken satisfies telego's ^\d+:[\w-]{35}$ gate. telego performs
	// no network call at construction under OD-C=a, so the token only has to
	// be format-valid; the HTTP client below answers everything locally.
	offlineToken = "123456:AAE-characterization-token-not-real"
)

// flowHarness wires the exact production graph from internal/bot/module.go:
// handlers registering their ordered predicate chains DIRECTLY on the shared
// telegofx router (the owner's reference pattern), the telegofx/fsm Store
// over the
// in-memory Storage - but with an offline telego bot whose HTTP client
// records every call.
type flowHarness struct {
	t         *testing.T
	bot       *telegofx.Bot
	router    *telegofx.Router
	transport *recordingTransport
	storage   *fakeStorage
	fsmStore  *fsm.Store
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

	cfg := telegofx.Config{
		Token: offlineToken,
		Mode:  telegofx.ModePolling,
	}

	// telegofx.New validates the token format only: no getMe, no polling
	// (OD-C=a). Every API call the suite observes goes through the recording
	// transport, which rides the graph-provided *http.Client the signature now
	// takes.
	b, err := telegofx.New(
		cfg,
		&http.Client{Transport: transport},
		nil,
		zap.NewNop(),
	)
	if err != nil {
		t.Fatalf("new bot: %v", err)
	}

	rt, err := telegofx.NewRouter(cfg, b)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}

	logger := zap.NewNop()
	fsmStore := fsm.NewStore(state.NewStorage(), logger)

	// Registration mirrors internal/bot/module.go: start registers its three
	// routes first, then stop's /stop, both directly on the router.

	startHandler := start.New(fsmStore, st, newTestTranslator(t))
	stopHandler := stop.New(fsmStore, st, logger, newTestTranslator(t))

	startHandler.Register(rt)
	stopHandler.Register(rt)

	return &flowHarness{
		t:         t,
		bot:       b,
		router:    rt,
		transport: transport,
		storage:   st,
		fsmStore:  fsmStore,
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
// mid-flight (the only non-empty state the suite needs). State is keyed by
// chat+user; the private fixtures set Chat.ID == From.ID == chatIDPrivate.
func (h *flowHarness) setState() {
	h.t.Helper()

	if err := h.fsmStore.Set(
		context.Background(),
		chatIDPrivate,
		chatIDPrivate,
		handlers.StateStartWaitForContact,
	); err != nil {
		h.t.Fatalf("set state: %v", err)
	}
}

// getState returns the FSM state recorded for chatID. Under chat+user
// keying a state may live under (chatID, chatID) - the private fixtures -
// or (chatID, otherUserID) - the group fixture's sender - so BOTH candidate
// keys are probed: otherwise a guard regression writing
// (chatIDGroup, otherUserID) would be invisible to SC-4. An empty result
// means no state was ever written for the chat.
func (h *flowHarness) getState(chatID int64) string {
	h.t.Helper()

	for _, userID := range []int64{chatID, otherUserID} {
		state, err := h.fsmStore.Get(context.Background(), chatID, userID)
		if err != nil {
			h.t.Fatalf("get state: %v", err)
		}
		if state != "" {
			return state
		}
	}

	return fsm.StateEmpty
}

// send drives exactly one update through the real dispatch cascade,
// synchronously: HandleUpdate runs middleware, predicates and the state groups
// inline before returning.
func (h *flowHarness) send(u telego.Update) {
	h.t.Helper()

	h.transport.reset()
	if err := h.router.BaseGroup().HandleUpdate(context.Background(), h.bot.Bot, u); err != nil {
		h.t.Fatalf("handle update: %v", err)
	}
}

// privateMessage builds a private-chat text message update for chatIDPrivate.
func privateMessage(id int, text string) telego.Update {
	return telego.Update{
		UpdateID: id,
		Message: &telego.Message{
			MessageID: id,
			Chat:      telego.Chat{ID: chatIDPrivate, Type: telego.ChatTypePrivate},
			From:      &telego.User{ID: chatIDPrivate},
			Text:      text,
		},
	}
}

// groupMessage builds a group-chat text message update (used by SC-4, whose
// body is pinned post-migration).
func groupMessage(id int, chatID int64, text string) telego.Update {
	return telego.Update{
		UpdateID: id,
		Message: &telego.Message{
			MessageID: id,
			Chat:      telego.Chat{ID: chatID, Type: telego.ChatTypeGroup},
			From:      &telego.User{ID: otherUserID},
			Text:      text,
		},
	}
}

// contactMessage builds a contact update for chatIDPrivate; sameUser selects
// the chat owner as the contact's user (the SC-2 ownership polarity).
func contactMessage(id int, sameUser bool, phone string) telego.Update {
	chatID := chatIDPrivate

	contactUserID := chatID
	if !sameUser {
		contactUserID = otherUserID
	}

	return telego.Update{
		UpdateID: id,
		Message: &telego.Message{
			MessageID: id,
			Chat:      telego.Chat{ID: chatID, Type: telego.ChatTypePrivate},
			From:      &telego.User{ID: chatID},
			Contact:   &telego.Contact{UserID: contactUserID, PhoneNumber: phone},
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
func (h *flowHarness) assertState(want string) {
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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
// updates create no FSM state (the dispatcher only routes updates whose
// update.Message is set; nothing keys FSM state off the other fields).
func TestBotFlow_SC3_update_narrowing(t *testing.T) {
	t.Parallel()

	h := newFlowHarness(t)

	updates := []telego.Update{
		{
			UpdateID: 6,
			EditedMessage: &telego.Message{
				MessageID: 6,
				Chat:      telego.Chat{ID: chatIDPrivate, Type: telego.ChatTypePrivate},
				Text:      "/start",
			},
		},
		{
			UpdateID: 7,
			ChannelPost: &telego.Message{
				MessageID: 7,
				Chat:      telego.Chat{ID: chatIDGroup, Type: telego.ChatTypeGroup},
				Text:      "/start",
			},
		},
		{
			UpdateID: 8,
			CallbackQuery: &telego.CallbackQuery{
				From: telego.User{ID: chatIDPrivate},
				Message: &telego.Message{
					MessageID: 8,
					Chat:      telego.Chat{ID: chatIDPrivate, Type: telego.ChatTypePrivate},
				},
				Data: "\fnoop",
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

// SC4_chat_type_guard enforces OD-SC4 (a): a group /start creates no FSM
// state and sends nothing.
//
// NOTE: this row was skipped until T3 because the pre-migration dispatcher
// had NO chat-type guard - a group /start DID create state and sent the
// welcome keyboard. T3 added predicates.MessageWithChatType(telego
// .ChatTypePrivate) to every route, so the row is un-skipped and now pins
// the post-migration contract. The skipped-era text is kept below in this
// comment; do not re-skip.
func TestBotFlow_SC4_chat_type_guard(t *testing.T) {
	t.Parallel()

	h := newFlowHarness(t)

	h.send(groupMessage(9, chatIDGroup, "/start"))

	if got := h.getState(chatIDGroup); got != fsm.StateEmpty {
		t.Fatalf("group /start must create no FSM state, got %q", got)
	}
	h.assertNoReply()
}

// ---------------------------------------------------------------------------
// Command matching: th.CommandEqual - case-insensitive, @suffix-agnostic.
// ---------------------------------------------------------------------------

// CM1_uppercase_stop_unlinks: "/STOP" in StateEmpty IS matched as /stop
// (th.CommandEqual compares group 1 EqualFold, owner decision A): Delete is
// called, the delete_success literal is replied, and the state stays
// StateEmpty. Re-pinned from CM1_uppercase_stop_noop, whose case-sensitive
// no-op expectation became false with th.CommandEqual.
func TestBotFlow_CM1_uppercase_stop_unlinks(t *testing.T) {
	t.Parallel()

	h := newFlowHarness(t)

	h.send(privateMessage(10, "/STOP"))

	if got := h.storage.deleteCalls(); len(got) != 1 || got[0] != chatIDPrivate {
		t.Fatalf("storage.Delete must be called once for chat %d, got %v", chatIDPrivate, got)
	}
	h.assertLastText(msgDeleteOK)
	h.assertState(fsm.StateEmpty)
}

// CM2_at_other_bot_runs_start: "/start@OtherBot" IS matched as /start
// (th.CommandEqual ignores the @suffix, owner decision A): the start flow
// runs - welcome reply sent, state enters wait_for_contact. Re-pinned from
// CM2_at_other_bot_dropped, whose suffix-rejection expectation became false
// with th.CommandEqual. The start handler never touches the phone storage
// (that happens on CONTACT), so the run is proven by reply + state.
func TestBotFlow_CM2_at_other_bot_runs_start(t *testing.T) {
	t.Parallel()

	h := newFlowHarness(t)

	h.send(privateMessage(11, "/start@"+otherBotName))

	if got := h.storage.storeCalls(); len(got) != 0 {
		t.Fatalf("phone storage must stay untouched until CONTACT, got %v", got)
	}
	h.assertLastText(msgWelcome)
	h.assertState(handlers.StateStartWaitForContact)
}

// CM3_at_own_bot_accepted: "/start@<own username>" runs the start flow.
func TestBotFlow_CM3_at_own_bot_accepted(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

// ---------------------------------------------------------------------------
// Panic recovery: a panicking route must not kill the process.
// ---------------------------------------------------------------------------

// TestFlow_PanicInLastRouteIsRecovered pins the telegofx default panic
// recovery through the APP's registration order: the panic route is appended
// AFTER start and stop, so a state-free private text falls through every app
// route into it (first-match-wins). The first dispatch panics and comes back
// as an error carrying the panic value; the second reaches the same route,
// and neither dispatch leaked a reply or an FSM state change.
func TestFlow_PanicInLastRouteIsRecovered(t *testing.T) {
	t.Parallel()

	h := newFlowHarness(t)

	var calls atomic.Int32
	h.router.Handle(func(_ *th.Context, _ telego.Update) error {
		if calls.Add(1) == 1 {
			panic("boom in app route")
		}

		return nil
	})

	err := h.router.BaseGroup().
		HandleUpdate(context.Background(), h.bot.Bot, privateMessage(9901, "plain text nobody handles"))
	if err == nil {
		t.Fatal("first dispatch error = nil, want the recovered panic")
	}
	if !strings.Contains(err.Error(), "boom in app route") {
		t.Errorf("first dispatch error = %v, want it to carry the panic value", err)
	}

	err = h.router.BaseGroup().
		HandleUpdate(context.Background(), h.bot.Bot, privateMessage(9902, "plain text nobody handles"))
	if err != nil {
		t.Fatalf("second dispatch error = %v, want nil after recovery", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("panic route calls = %d, want 2", got)
	}

	h.assertNoReply()
	h.assertState(fsm.StateEmpty)
}
