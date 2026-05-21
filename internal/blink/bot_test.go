package blink

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBotDeploy(t *testing.T) {
	bm := NewBotManager()

	bot, err := bm.Deploy(BotConfig{
		Name:     "test-bot",
		Platform: PlatformSlack,
		Token:    "xoxb-test",
		Handler:  "echo",
	})
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}
	if bot.State != BotDeployed {
		t.Errorf("expected deployed, got %s", bot.State)
	}
	if bot.Config.Name != "test-bot" {
		t.Errorf("expected test-bot, got %s", bot.Config.Name)
	}
}

func TestBotDeployMissingName(t *testing.T) {
	bm := NewBotManager()
	_, err := bm.Deploy(BotConfig{Platform: PlatformSlack})
	if err == nil {
		t.Error("expected error for missing name")
	}
}

func TestBotDeployMissingPlatform(t *testing.T) {
	bm := NewBotManager()
	_, err := bm.Deploy(BotConfig{Name: "test"})
	if err == nil {
		t.Error("expected error for missing platform")
	}
}

func TestBotStartStop(t *testing.T) {
	bm := NewBotManager()
	bot, _ := bm.Deploy(BotConfig{
		Name:     "test",
		Platform: PlatformDiscord,
		Handler:  "echo",
	})

	started, err := bm.Start(context.Background(), bot.ID)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if started.State != BotRunning {
		t.Errorf("expected running, got %s", started.State)
	}

	if err := bm.Stop(bot.ID); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	stopped, _ := bm.Get(bot.ID)
	if stopped.State != BotStopped {
		t.Errorf("expected stopped, got %s", stopped.State)
	}
}

func TestBotHandleMessage(t *testing.T) {
	bm := NewBotManager()
	bot, _ := bm.Deploy(BotConfig{
		Name:     "echo-bot",
		Platform: PlatformTelegram,
		Handler:  "echo",
	})
	bm.Start(context.Background(), bot.ID)

	resp, err := bm.HandleMessage(context.Background(), bot.ID, IncomingMessage{
		From:    "user-1",
		Channel: "general",
		Content: "hello",
	})
	if err != nil {
		t.Fatalf("HandleMessage failed: %v", err)
	}
	if !strings.Contains(resp.Content, "echo-bot") {
		t.Errorf("expected bot name in response, got %s", resp.Content)
	}
	if !strings.Contains(resp.Content, "hello") {
		t.Errorf("expected echo of 'hello', got %s", resp.Content)
	}
}

func TestBotHandleMessageStopped(t *testing.T) {
	bm := NewBotManager()
	bot, _ := bm.Deploy(BotConfig{
		Name:     "stopped",
		Platform: PlatformSlack,
		Handler:  "echo",
	})

	_, err := bm.HandleMessage(context.Background(), bot.ID, IncomingMessage{
		Content: "test",
	})
	if err == nil {
		t.Error("expected error for stopped bot")
	}
}

func TestBotMessageCount(t *testing.T) {
	bm := NewBotManager()
	bot, _ := bm.Deploy(BotConfig{
		Name:     "counter",
		Platform: PlatformDiscord,
		Handler:  "echo",
	})
	bm.Start(context.Background(), bot.ID)

	for i := 0; i < 5; i++ {
		bm.HandleMessage(context.Background(), bot.ID, IncomingMessage{Content: "msg"})
	}

	updated, _ := bm.Get(bot.ID)
	if updated.MessageCount != 5 {
		t.Errorf("expected 5 messages, got %d", updated.MessageCount)
	}
}

func TestBotList(t *testing.T) {
	bm := NewBotManager()
	bm.Deploy(BotConfig{Name: "a", Platform: PlatformSlack, Handler: "echo"})
	bm.Deploy(BotConfig{Name: "b", Platform: PlatformDiscord, Handler: "echo"})
	bm.Deploy(BotConfig{Name: "c", Platform: PlatformTelegram, Handler: "echo"})

	bots := bm.List()
	if len(bots) != 3 {
		t.Errorf("expected 3 bots, got %d", len(bots))
	}
}

func TestBotListByPlatform(t *testing.T) {
	bm := NewBotManager()
	bm.Deploy(BotConfig{Name: "a", Platform: PlatformSlack, Handler: "echo"})
	bm.Deploy(BotConfig{Name: "b", Platform: PlatformSlack, Handler: "echo"})
	bm.Deploy(BotConfig{Name: "c", Platform: PlatformDiscord, Handler: "echo"})

	slackBots := bm.ListByPlatform(PlatformSlack)
	if len(slackBots) != 2 {
		t.Errorf("expected 2 slack bots, got %d", len(slackBots))
	}
}

func TestBotRemove(t *testing.T) {
	bm := NewBotManager()
	bot, _ := bm.Deploy(BotConfig{Name: "rm", Platform: PlatformSlack, Handler: "echo"})
	bm.Remove(bot.ID)
	if len(bm.List()) != 0 {
		t.Error("expected 0 after remove")
	}
}

func TestBotCustomHandler(t *testing.T) {
	bm := NewBotManager()
	bm.RegisterHandler("upper", func(ctx context.Context, bot *Bot, msg IncomingMessage) (OutgoingMessage, error) {
		return OutgoingMessage{
			Content: strings.ToUpper(msg.Content),
			Channel: msg.Channel,
		}, nil
	})

	bot, _ := bm.Deploy(BotConfig{
		Name:     "upper-bot",
		Platform: PlatformSlack,
		Handler:  "upper",
	})
	bm.Start(context.Background(), bot.ID)

	resp, _ := bm.HandleMessage(context.Background(), bot.ID, IncomingMessage{
		Content: "hello world",
	})
	if resp.Content != "HELLO WORLD" {
		t.Errorf("expected HELLO WORLD, got %s", resp.Content)
	}
}

func TestBotServeHTTP(t *testing.T) {
	bm := NewBotManager()
	bot, _ := bm.Deploy(BotConfig{
		Name:     "http-bot",
		Platform: PlatformWebhook,
		Handler:  "echo",
	})
	bm.Start(context.Background(), bot.ID)

	body := `{"from":"user","channel":"test","content":"hi"}`
	req := httptest.NewRequest("POST", "/"+bot.ID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	bm.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp OutgoingMessage
	json.NewDecoder(w.Body).Decode(&resp)
	if !strings.Contains(resp.Content, "hi") {
		t.Errorf("expected 'hi' in response, got %s", resp.Content)
	}
}

func TestBotServeHTTPWrongMethod(t *testing.T) {
	bm := NewBotManager()
	req := httptest.NewRequest("GET", "/some-bot", nil)
	w := httptest.NewRecorder()
	bm.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestBotSerialization(t *testing.T) {
	bot := &Bot{
		ID:      "bot-1",
		Config:  BotConfig{Name: "test", Platform: PlatformSlack, Token: "secret"},
		State:   BotRunning,
		MessageCount: 42,
	}
	data, err := json.Marshal(bot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "slack") {
		t.Error("expected platform in JSON")
	}
}
