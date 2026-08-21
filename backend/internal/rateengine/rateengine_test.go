package rateengine

import (
	"testing"

	"github.com/bruhjeshhh/delivery-tracker/internal/models"
)

func TestVolumetricWeight(t *testing.T) {
	// 30 x 20 x 10 = 6000 / 5000 = 1.2 kg
	got := VolumetricWeight(30, 20, 10)
	if got != 1.2 {
		t.Fatalf("expected 1.2, got %v", got)
	}
}

func TestBillableWeight_ActualHigher(t *testing.T) {
	got := BillableWeight(5, 1.2)
	if got != 5 {
		t.Fatalf("expected actual weight 5 to win, got %v", got)
	}
}

func TestBillableWeight_VolumetricHigher(t *testing.T) {
	got := BillableWeight(0.5, 1.2)
	if got != 1.2 {
		t.Fatalf("expected volumetric weight 1.2 to win, got %v", got)
	}
}

func TestCalculate_Prepaid(t *testing.T) {
	card := models.RateCard{ID: 1, BaseFee: 30, RatePerKg: 20, CODSurcharge: 25}
	in := ChargeInput{
		LengthCM: 30, BreadthCM: 20, HeightCM: 10, // volumetric = 1.2kg
		ActualWeightKg: 2, // actual wins: billable = 2kg
		OrderType:      models.OrderTypeB2C,
		PaymentType:    models.PaymentPrepaid,
	}
	res, err := Calculate(in, card)
	if err != nil {
		t.Fatal(err)
	}
	// base 30 + 2kg * 20/kg = 70, no COD surcharge
	if res.Charge != 70 {
		t.Fatalf("expected charge 70, got %v", res.Charge)
	}
	if res.BillableWeightKg != 2 {
		t.Fatalf("expected billable weight 2, got %v", res.BillableWeightKg)
	}
}

func TestCalculate_COD(t *testing.T) {
	card := models.RateCard{ID: 1, BaseFee: 30, RatePerKg: 20, CODSurcharge: 25}
	in := ChargeInput{
		LengthCM: 30, BreadthCM: 20, HeightCM: 10,
		ActualWeightKg: 2,
		OrderType:      models.OrderTypeB2C,
		PaymentType:    models.PaymentCOD,
	}
	res, err := Calculate(in, card)
	if err != nil {
		t.Fatal(err)
	}
	// 70 + 25 COD surcharge = 95
	if res.Charge != 95 {
		t.Fatalf("expected charge 95, got %v", res.Charge)
	}
}

func TestCalculate_MissingRateCard(t *testing.T) {
	_, err := Calculate(ChargeInput{}, models.RateCard{})
	if err != ErrNoRateCard {
		t.Fatalf("expected ErrNoRateCard, got %v", err)
	}
}
