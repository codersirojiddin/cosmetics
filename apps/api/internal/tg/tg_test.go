package tg

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientRequestsAndErrors(t *testing.T) {
	var lastPath string
	var lastBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		lastBody = nil
		_ = json.Unmarshal(b, &lastBody)
		switch {
		case strings.HasSuffix(lastPath, "/getMe"):
			_, _ = io.WriteString(w, `{"ok":true,"result":{"id":42,"is_bot":true,"first_name":"Shop","username":"shop_bot"}}`)
		case strings.HasSuffix(lastPath, "/sendMessage"):
			if lastBody["chat_id"].(float64) == 13 {
				w.WriteHeader(403)
				_, _ = io.WriteString(w, `{"ok":false,"error_code":403,"description":"Forbidden: bot was blocked by the user"}`)
				return
			}
			_, _ = io.WriteString(w, `{"ok":true,"result":{"message_id":77,"chat":{"id":5,"type":"private"}}}`)
		case strings.HasSuffix(lastPath, "/getUpdates"):
			_, _ = io.WriteString(w, `{"ok":true,"result":[{"update_id":9,"message":{"message_id":1,"from":{"id":5,"first_name":"Ali"},"chat":{"id":5,"type":"private"},"text":"/start login_x"}}]}`)
		default:
			_, _ = io.WriteString(w, `{"ok":true,"result":true}`)
		}
	}))
	defer srv.Close()
	c := NewClient("123:SECRET").WithBaseURL(srv.URL)
	ctx := context.Background()

	me, err := c.GetMe(ctx)
	require.NoError(t, err)
	require.Equal(t, "shop_bot", me.Username)
	require.Equal(t, "/bot123:SECRET/getMe", lastPath)

	id, err := c.SendMessage(ctx, 5, "<b>hi</b>", &SendOpts{Keyboard: Keyboard{{{Text: "Go", CallbackData: "claim:1"}}}})
	require.NoError(t, err)
	require.EqualValues(t, 77, id)
	require.Equal(t, "HTML", lastBody["parse_mode"])
	require.NotNil(t, lastBody["reply_markup"])

	_, err = c.SendMessage(ctx, 13, "x", nil)
	var ae *APIError
	require.ErrorAs(t, err, &ae)
	require.True(t, ae.Blocked())

	ups, err := c.GetUpdates(ctx, 9, 1)
	require.NoError(t, err)
	require.Len(t, ups, 1)
	require.Equal(t, "/start login_x", ups[0].Message.Text)
	require.EqualValues(t, 9, lastBody["offset"])

	require.NoError(t, c.EditMessage(ctx, 5, 77, "edited", nil))
	require.Contains(t, lastBody, "reply_markup", "edits send an explicit (possibly empty) keyboard so buttons can be removed")
	require.NoError(t, c.SetWebhook(ctx, "https://x/y", "sec"))
	require.Equal(t, "sec", lastBody["secret_token"])
}

func TestTokenIsRedactedFromTransportErrors(t *testing.T) {
	c := NewClient("123:SECRET").WithBaseURL("http://127.0.0.1:1") // nothing listens here
	_, err := c.GetMe(context.Background())
	require.Error(t, err)
	require.NotContains(t, err.Error(), "SECRET")
}
