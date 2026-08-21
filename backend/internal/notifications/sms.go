package notifications

import "fmt"

type SMSSender interface {
	Send(toPhone, body string) error
}

// MockSMSSender logs instead of dispatching — swap for a real Twilio/MSG91
// client by implementing SMSSender; the worker only depends on the interface.
type MockSMSSender struct{}

func NewMockSMSSender() *MockSMSSender { return &MockSMSSender{} }

func (m *MockSMSSender) Send(toPhone, body string) error {
	fmt.Printf("[sms:mock] to=%s body=%q\n", toPhone, body)
	return nil
}
