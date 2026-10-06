package mailer

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderGivesBothBodies(t *testing.T) {
	link := "https://vpn.example.com/password/set?code=abc123"
	for _, name := range []string{WelcomeTemplate, PasswordTemplate, ForgotTemplate} {
		content, err := Render(name, LinkData{
			Email: "ada@example.com", Organization: "Acme", Link: link, Lasts: "3 days",
			URL: "https://vpn.example.com/",
		})
		if err != nil {
			t.Fatalf("Render(%s): %v", name, err)
		}
		if !strings.Contains(content.HTML, `href="`+link+`"`) {
			t.Errorf("%s HTML lacks the link", name)
		}
		if strings.Contains(content.Text, "<") {
			t.Errorf("%s plain text holds markup:\n%s", name, content.Text)
		}
		for _, want := range []string{"ada@example.com", link, "3 days"} {
			if !strings.Contains(content.Text, want) {
				t.Errorf("%s plain text lacks %q", name, want)
			}
		}
	}
}

func TestMessageQueuedBeforePlainTextStillDecodes(t *testing.T) {
	// A job written by the previous version, with no "text" field.
	var msg Message
	err := json.Unmarshal([]byte(`{"recipient":"a@example.com","subject":"s","body":"<p>hi</p>","sender":"b@example.com"}`), &msg)
	if err != nil || msg.Body != "<p>hi</p>" || msg.Text != "" {
		t.Errorf("decoded %+v, %v", msg, err)
	}
}
