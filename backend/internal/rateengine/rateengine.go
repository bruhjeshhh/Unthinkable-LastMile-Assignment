package rateengine

import (
	"errors"
	"math"

	"github.com/bruhjeshhh/delivery-tracker/internal/models"
)

var ErrNoRateCard = errors.New("no rate card configured for this order type / zone pair")

const volumetricDivisor = 5000.0 // industry-standard volumetric divisor (cm^3 / 5000 = kg)

// Dimensions in cm, weight in kg.
type ChargeInput struct {
	LengthCM       float64
	BreadthCM      float64
	HeightCM       float64
	ActualWeightKg float64
	OrderType      models.OrderType
	PaymentType    models.PaymentType
}

type ChargeResult struct {
	VolumetricWeightKg float64
	BillableWeightKg   float64
	Charge             float64
	RateCardID         int64
}

// VolumetricWeight computes (L x B x H) / 5000, the standard courier-industry formula.
func VolumetricWeight(lengthCM, breadthCM, heightCM float64) float64 {
	return (lengthCM * breadthCM * heightCM) / volumetricDivisor
}

// BillableWeight is the greater of actual and volumetric weight — couriers
// always bill on whichever is higher, since a light-but-bulky package still
// occupies vehicle space proportional to its volume.
func BillableWeight(actualKg, volumetricKg float64) float64 {
	return math.Max(actualKg, volumetricKg)
}

// Calculate is a pure function: given the physical inputs and a resolved rate
// card, it returns the final charge. It has no DB/side effects, which is what
// makes it independently unit-testable — the rate card lookup (zone + order
// type -> rate_cards row) happens one layer up, in the orders service.
func Calculate(in ChargeInput, card models.RateCard) (ChargeResult, error) {
	if card.ID == 0 {
		return ChargeResult{}, ErrNoRateCard
	}
	volumetric := VolumetricWeight(in.LengthCM, in.BreadthCM, in.HeightCM)
	billable := BillableWeight(in.ActualWeightKg, volumetric)

	charge := card.BaseFee + billable*card.RatePerKg
	if in.PaymentType == models.PaymentCOD {
		charge += card.CODSurcharge
	}
	// round to 2 decimal places
	charge = math.Round(charge*100) / 100

	return ChargeResult{
		VolumetricWeightKg: round3(volumetric),
		BillableWeightKg:   round3(billable),
		Charge:             charge,
		RateCardID:         card.ID,
	}, nil
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}
