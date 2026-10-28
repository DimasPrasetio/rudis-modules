package infrastructure

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"__GO_MODULE__/internal/modules/mailer/application"
)

func TestBuildMessage(t *testing.T) {
	msg, err := BuildMessage(application.Envelope{ID: 3, From: "noreply@app.test", FromName: "Toko Maju", To: "ana@example.com", ToName: "Ana Ságara",
		Subject: "Atur ulang kata sandi — Toko Maju", Text: "Baris satu\nBaris dua", HTML: "<p>Halo</p>"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	s := string(msg)
	for _, want := range []string{"From: \"Toko Maju\" <noreply@app.test>\r\n", "Subject: =?utf-8?q?", "multipart/alternative", "Content-Type: text/html", "Message-ID: <3."} {
		if !strings.Contains(s, want) {
			t.Errorf("message lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\n") {
		t.Error("a bare LF: every line must end in CRLF")
	}
}

// fakeServer is the smallest SMTP server that can take a message, rejecting RCPT with code
// reject when it is set.
func fakeServer(t *testing.T, reject string) (addr string, got chan string) {
	t.Helper()
	return fakeServerWith(t, serverAnswers{rcpt: reject})
}

// serverAnswers changes what fakeServerWith replies: a refusal at MAIL FROM or RCPT, and what it
// does at QUIT ("drop": hang up without answering).
type serverAnswers struct {
	mail, rcpt, quit string
}

func fakeServerWith(t *testing.T, ans serverAnswers) (addr string, got chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	got = make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		defer ln.Close()
		r := bufio.NewReader(conn)
		say := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
		say("220 fake")
		var data strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				say("250 fake")
			case strings.HasPrefix(cmd, "MAIL"):
				if ans.mail != "" {
					say(ans.mail)
				} else {
					say("250 ok")
				}
			case strings.HasPrefix(cmd, "RCPT"):
				if ans.rcpt != "" {
					say(ans.rcpt)
				} else {
					say("250 ok")
				}
			case cmd == "DATA":
				say("354 go")
				for {
					l, err := r.ReadString('\n')
					if err != nil || l == ".\r\n" {
						break
					}
					data.WriteString(l)
				}
				say("250 queued")
				got <- data.String()
			case cmd == "QUIT":
				switch ans.quit {
				case "drop":
				case "":
					say("221 bye")
				default:
					say(ans.quit)
				}
				return
			default:
				say("250 ok")
			}
		}
	}()
	return ln.Addr().String(), got
}

func hostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, port, _ := net.SplitHostPort(addr)
	p, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	return host, p
}

func TestSMTPDelivers(t *testing.T) {
	addr, got := fakeServer(t, "")
	host, p := hostPort(t, addr)
	err := SMTP{Host: host, Port: p, TLS: "none"}.Send(context.Background(), application.Envelope{From: "noreply@app.test", To: "ana@example.com", Subject: "Halo", Text: "Isi pesan teks"})
	if err != nil {
		t.Fatal(err)
	}
	if body := <-got; !strings.Contains(body, "Subject: Halo") || !strings.Contains(body, "\r\n\r\nIsi pesan teks") {
		t.Fatalf("a plain-text message must arrive with its body; server got:\n%s", body)
	}
}

func TestShutdownCutsASlowServerShort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			defer conn.Close()
			_, _ = conn.Write([]byte("220 fake\r\n"))
			time.Sleep(5 * time.Second) // never answers EHLO
		}
	}()
	host, p := hostPort(t, ln.Addr().String())
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)
	start := time.Now()
	_ = SMTP{Host: host, Port: p, TLS: "none"}.Send(ctx, application.Envelope{From: "noreply@app.test", To: "a@example.com", Subject: "s", Text: "t"})
	if waited := time.Since(start); waited > 2*time.Second {
		t.Fatalf("Send ignored the cancellation for %v", waited)
	}
}

func TestSMTPRefusalIsPermanent(t *testing.T) {
	addr, _ := fakeServer(t, "550 no such user")
	host, p := hostPort(t, addr)
	err := SMTP{Host: host, Port: p, TLS: "none"}.Send(context.Background(), application.Envelope{From: "noreply@app.test", To: "nobody@example.com", Subject: "s", Text: "t"})
	if !errors.As(err, new(application.PermanentError)) {
		t.Fatalf("want a permanent error for 550, got %v", err)
	}
}

func TestStartTLSIsRequiredWhenAsked(t *testing.T) {
	addr, _ := fakeServer(t, "")
	host, p := hostPort(t, addr)
	err := SMTP{Host: host, Port: p, TLS: "starttls"}.Send(context.Background(), application.Envelope{From: "noreply@app.test", To: "a@example.com", Subject: "s", Text: "t"})
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("a server without STARTTLS must be refused, got %v", err)
	}
}

func TestMessageIDIsTheSameOnEveryAttempt(t *testing.T) {
	e := application.Envelope{ID: 9, CreatedAt: time.Date(2026, 10, 28, 1, 2, 3, 4, time.UTC), From: "noreply@app.test", To: "a@example.com", Subject: "s", Text: "t",
		ReplyTo: "halo@tokomaju.test"}
	id := func() string {
		msg, err := BuildMessage(e, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		s := string(msg)
		if !strings.Contains(s, "Reply-To: <halo@tokomaju.test>\r\n") {
			t.Fatalf("no Reply-To:\n%s", s)
		}
		i := strings.Index(s, "Message-ID: ")
		return s[i : i+strings.Index(s[i:], "\r\n")]
	}
	if a, b := id(), id(); a != b {
		t.Fatalf("Message-ID changed between attempts: %s / %s", a, b)
	}
}

func TestAnAcceptedMessageIsDeliveredWhateverQuitSays(t *testing.T) {
	for _, quit := range []string{"drop", "421 closing"} {
		addr, got := fakeServerWith(t, serverAnswers{quit: quit})
		host, p := hostPort(t, addr)
		err := SMTP{Host: host, Port: p, TLS: "none"}.Send(context.Background(), application.Envelope{From: "noreply@app.test", To: "ana@example.com", Subject: "s", Text: "t"})
		if err != nil {
			t.Fatalf("quit %q: the server took the message, yet Send failed (it would be sent again): %v", quit, err)
		}
		<-got
	}
}

func TestASenderRefusalIsConfigurationNotPermanent(t *testing.T) {
	addr, _ := fakeServerWith(t, serverAnswers{mail: "553 5.7.1 sender not allowed"})
	host, p := hostPort(t, addr)
	err := SMTP{Host: host, Port: p, TLS: "none"}.Send(context.Background(), application.Envelope{From: "noreply@app.test", To: "ana@example.com", Subject: "s", Text: "t"})
	if !errors.As(err, new(application.ConfigError)) || errors.As(err, new(application.PermanentError)) {
		t.Fatalf("a sender refused at MAIL FROM must be a ConfigError, retried — got %T %v", err, err)
	}
}
