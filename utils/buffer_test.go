package utils

import (
	"chat/globals"
	"testing"
)

type testCharge struct{}

func (testCharge) GetType() string             { return globals.TokenBilling }
func (testCharge) GetModels() []string         { return nil }
func (testCharge) GetInput() float32           { return 0.5 }
func (testCharge) GetOutput() float32          { return 1.25 }
func (testCharge) SupportAnonymous() bool      { return true }
func (testCharge) IsBilling() bool             { return true }
func (testCharge) IsBillingType(t string) bool { return t == globals.TokenBilling }
func (testCharge) GetLimit() float32           { return 0 }

func TestRecordQuotaMatchesInputAndOutputQuota(t *testing.T) {
	buffer := &Buffer{
		InputTokens: 2000,
		Charge:      testCharge{},
		Model:       "gpt-3.5-turbo",
	}
	buffer.Write("hello world")

	inputQuota := buffer.GetInputQuota()
	outputQuota := buffer.GetOutputQuota(false)
	recordQuota := buffer.GetRecordQuota()

	if recordQuota != inputQuota+outputQuota {
		t.Fatalf("record quota = %f, want input + output quota = %f", recordQuota, inputQuota+outputQuota)
	}
	if inputQuota <= 0 || outputQuota <= 0 {
		t.Fatalf("input quota = %f, output quota = %f, both should be positive", inputQuota, outputQuota)
	}
}
