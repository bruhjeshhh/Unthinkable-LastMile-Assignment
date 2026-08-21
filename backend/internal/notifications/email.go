package notifications

import (
	"fmt"
	"net/smtp"

	"github.com/bruhjeshhh/delivery-tracker/internal/config"
)

type EmailSender interface {
	Send(toEmail, subject, body string) error
}

type SMTPEmailSender struct {
	cfg config.Config
}

func NewSMTPEmailSender(cfg config.Config) *SMTPEmailSender {
	return &SMTPEmailSender{cfg: cfg}
}

func (s *SMTPEmailSender) Send(toEmail, subject, body string) error {
	if s.cfg.SMTPHost == "" {
		// No SMTP configured (e.g. local dev) — log instead of failing the whole flow.
		fmt.Printf("[email:mock] to=%s subject=%q body=%q\n", toEmail, subject, body)
		return nil
	}
	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n",
		s.cfg.FromEmail, toEmail, subject, body))
	addr := s.cfg.SMTPHost + ":" + s.cfg.SMTPPort
	return smtp.SendMail(addr, auth, s.cfg.FromEmail, []string{toEmail}, msg)
}
