// Deprecated: Use [services.CalculatePricing] from internal/services/pricing.go instead.
// Package calc is retained temporarily for backward compatibility and tests.
// Package calc provides pure business logic for shipping pricing and storage (PRD 8.9, 8.6).
// No database or HTTP; safe for unit testing.
package calc

import (
	"math"

	"github.com/Qcsinc23/qcs-cargo/internal/services"
)

const (
	DimWeightDivisor    = 166.0 // volumetric divisor (lb/in³)
	MinCharge           = 10.0  // minimum shipping charge USD
	VolumeDiscountLb    = 100.0 // weight (lb) above which volume discount applies
	VolumeDiscountPct   = 0.05  // 5% off
	DoorToDoorFee       = 25.0  // flat fee for door_to_door
	ExpressSurchargePct = 0.25  // 25% for express
)

// Destination rates USD per lb (PRD 8.2).
var destRates = services.Rates

// DimensionalWeight returns (L*W*H)/166. Returns 0 if any dimension is 0.
func DimensionalWeight(l, w, h float64) float64 {
	if l <= 0 || w <= 0 || h <= 0 {
		return 0
	}
	return (l * w * h) / DimWeightDivisor
}

// BillableWeight returns the greater of actual weight and dimensional weight.
func BillableWeight(actualWeight, dimWeight float64) float64 {
	if dimWeight > actualWeight {
		return dimWeight
	}
	return actualWeight
}

// RateForDestination returns rate per lb and true if destination is known.
func RateForDestination(destID string) (float64, bool) {
	r, ok := destRates[destID]
	return r, ok
}

// ShippingInput is the input to CalculateShipping (PRD 8.9).
type ShippingInput struct {
	Destination           string
	Service               string // standard, express, door_to_door
	ActualWeight          float64
	Length, Width, Height float64
	DeclaredValue         float64
	Insurance             bool
}

// ShippingResult is the result of CalculateShipping.
type ShippingResult struct {
	DestinationID  string
	Service        string
	ActualWeight   float64
	DimWeight      float64
	BillableWeight float64
	RatePerLb      float64
	BaseCost       float64
	Surcharge      float64
	DoorToDoorFee  float64
	Insurance      float64
	VolumeDiscount float64
	Total          float64
	MinimumApplied bool
}

// CalculateShipping computes shipping cost from input. Returns zero-value result and false for unknown destination or invalid weight.
func CalculateShipping(in ShippingInput) (ShippingResult, bool) {
	if _, ok := RateForDestination(in.Destination); !ok || in.ActualWeight <= 0 {
		return ShippingResult{}, false
	}
	pricing := services.CalculatePricing(services.PricingInput{
		DestinationID: in.Destination,
		WeightLbs:     in.ActualWeight,
		LengthIn:      in.Length,
		WidthIn:       in.Width,
		HeightIn:      in.Height,
		ServiceType:   in.Service,
		ValueUSD:      in.DeclaredValue,
		AddInsurance:  in.Insurance,
	})

	surcharge := 0.0
	doorToDoor := 0.0
	switch in.Service {
	case "express":
		surcharge = pricing.ServiceFees
	case "door_to_door":
		doorToDoor = pricing.ServiceFees
	}

	return ShippingResult{
		DestinationID:  pricing.DestinationID,
		Service:        pricing.Service,
		ActualWeight:   pricing.ActualWeight,
		DimWeight:      pricing.DimWeight,
		BillableWeight: math.Round(pricing.BillableWeight*100) / 100,
		RatePerLb:      pricing.RatePerLb,
		BaseCost:       pricing.Subtotal,
		Surcharge:      math.Round(surcharge*100) / 100,
		DoorToDoorFee:  doorToDoor,
		Insurance:      pricing.Insurance,
		VolumeDiscount: pricing.Discount,
		Total:          pricing.Total,
		MinimumApplied: pricing.MinimumApplied,
	}, true
}
