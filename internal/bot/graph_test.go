package bot_test

// graph_test.go drives the PRODUCTION bot module through fx: the
// handler.Handler value group, the invoke registration loop and the
// dispatcher seam, with construction only and zero external network. It is
// the regression gate for the group's load-bearing typing: dig keys value
// groups by EXACT type, so a handler ctor returning the concrete type would
// make []handler.Handler resolve EMPTY with no error and register no route -
// a failure mode the oracle cannot see (it registers handlers directly) and
// go build/vet cannot see either.
//
// The drive rows also pin the predicates the handler registration MUST keep:
// the exact-match case-sensitive command matcher (CM1/CM2) and the private
// chat-type guard (OD-SC4(a)). The proxy points at loopback port 1, so the
// reply attempt of a REGISTERED handler fails fast on a refused local dial -
// no packet leaves the machine, and the failure itself is the observable
// proof that the route ran.

import (
	"context"
	"strings"
	"testing"

	"github.com/capcom6/phone2tg-proxy/internal/bot"
	"github.com/capcom6/phone2tg-proxy/internal/i18n"
	"github.com/capcom6/phone2tg-proxy/internal/storage"
	"github.com/go-core-fx/logger"
	"github.com/go-core-fx/telegofx"
	"github.com/mymmrac/telego"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

// loopbackRefusedProxy is an accepted SOCKS5 URL whose port nobody listens
// on: every reply attempt is refused locally instead of reaching the
// network.
const loopbackRefusedProxy = "socks5://127.0.0.1:1"

// graphStubStorage satisfies storage.Service without Redis; the drive rows
// only exercise the /start command path, which stores nothing.
type graphStubStorage struct{}

// Store implements storage.Service.
func (graphStubStorage) Store(_ context.Context, _ string, _ int64) error {
	return nil
}

// Get implements storage.Service.
func (graphStubStorage) Get(_ context.Context, _ string) (int64, error) {
	return 0, storage.ErrPhoneNumberNotFound
}

// Delete implements storage.Service.
func (graphStubStorage) Delete(_ context.Context, _ int64) error {
	return nil
}

// newBotGraph constructs the production bot graph: logger + telegofx with the
// router + the bot module itself (group provides, invoke loop, seam). No
// lifecycle hook ever runs, so no polling starts.
func newBotGraph(t *testing.T) (*telegofx.Bot, *telegofx.Router) {
	t.Helper()

	var (
		b *telegofx.Bot
		r *telegofx.Router
	)

	app := fx.New(
		logger.Module(),
		fx.Supply(telegofx.Config{
			Token:    offlineToken,
			Mode:     telegofx.ModePolling,
			ProxyURL: loopbackRefusedProxy,
		}),
		telegofx.Module(telegofx.WithRouter()),
		fx.Provide(func() storage.Service { return graphStubStorage{} }),
		fx.Provide(func() (*i18n.Service, error) {
			return i18n.NewService(i18n.Config{
				Language:         "en",
				TranslationsPath: "../../i18n/locales",
			})
		}),
		bot.Module(),
		fx.Populate(&b, &r),
		fx.WithLogger(func() fxevent.Logger { return fxevent.NopLogger }),
	)
	if err := app.Err(); err != nil {
		t.Fatalf("construct the production bot graph: %v", err)
	}

	return b, r
}

// graphMessage builds a message update for the given chat and text.
func graphMessage(updateID int, chat telego.Chat, text string) telego.Update {
	return telego.Update{
		UpdateID: updateID,
		Message: &telego.Message{
			MessageID: updateID,
			Chat:      chat,
			From:      &telego.User{ID: chat.ID},
			Text:      text,
		},
	}
}

// TestBotGraph_handlerGroupRegistersRoutes drives one update per row through
// the constructed production graph. A non-empty error means the route
// REGISTERED and ran to its (offline-failed) reply; a silent nil means the
// update was dropped before any handler body executed.
func TestBotGraph_handlerGroupRegistersRoutes(t *testing.T) {
	private := telego.Chat{ID: 100, Type: telego.ChatTypePrivate}

	cases := []struct {
		name    string
		update  telego.Update
		wantErr string
	}{
		{
			name:    "private /start reaches the registered handler",
			update:  graphMessage(1, private, "/start"),
			wantErr: "send message",
		},
		{
			name:    "uppercase /STOP stays unmatched CM1",
			update:  graphMessage(2, telego.Chat{ID: 101, Type: telego.ChatTypePrivate}, "/STOP"),
			wantErr: "",
		},
		{
			name:    "/start@OtherBot is dropped CM2",
			update:  graphMessage(3, telego.Chat{ID: 102, Type: telego.ChatTypePrivate}, "/start@OtherBot"),
			wantErr: "",
		},
		{
			name:    "group /start is dropped OD-SC4a",
			update:  graphMessage(4, telego.Chat{ID: 103, Type: telego.ChatTypeGroup}, "/start"),
			wantErr: "",
		},
	}

	b, r := newBotGraph(t)

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := r.BaseGroup().HandleUpdate(context.Background(), b.Bot, tt.update)

			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("update %q must be dropped before any handler, got %v", tt.update.Message.Text, err)
				}

				return
			}

			if err == nil {
				t.Fatalf("update %q must reach the registered handler whose reply fails offline, got nil",
					tt.update.Message.Text)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("update %q error %q, want it to contain %q", tt.update.Message.Text, err.Error(), tt.wantErr)
			}
		})
	}
}
