// Package calc provides pure business logic for shipping pricing and storage (PRD 8.9, 8.6).
// No database or HTTP; safe for unit testing.
package calc

import "math"

const (
	DimWeightDivisor = 166.0   // volumetric divisor (lb/in³)
	MinCharge        = 10.0    // minimum shipping charge USD
	VolumeDiscountLb = 100.0   // weight (lb) above which volume discount applies
	VolumeDiscountPct = 0.05  // 5% off
	DoorToDoorFee    = 25.0    // flat fee for door_to_door
	ExpressSurchargePct = 0.25 // 25% for express
)

// Destination rates USD per lb (PRD 8.2).
var destRates = map[string]float64{
	"guyana": 3.50, "jamaica": 3.75, "trinidad": 3.50, "barbados": 4.00, "suriname": 4.25,
}

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
	Destination   string
	Service       string  // standard, express, door_to_door
	ActualWeight  float64
	Length, Width, Height float64
	DeclaredValue float64
	Insurance     bool
}

// ShippingResult is the result of CalculateShipping.
type ShippingResult struct {
	DestinationID   string
	Service         string
	ActualWeight    float64
	DimWeight       float64
	BillableWeight  float64
	RatePerLb       float64
	BaseCost        float64
	Surcharge       float64
	DoorToDoorFee   float64
	Insurance       float64
	VolumeDiscount  float64
	Total           float64
	MinimumApplied  bool
}

// CalculateShipping computes shipping cost from input. Returns zero-value result and false for unknown destination or invalid weight.
func CalculateShipping(in ShippingInput) (ShippingResult, bool) {
	rate, ok := RateForDestination(in.Destination)
	if !ok || in.ActualWeight <= 0 {
		return ShippingResult{}, false
	}
	dim := DimensionalWeight(in.Length, in.Width, in.Height)
	billable := BillableWeight(in.ActualWeight, dim)
	base := billable * rate

	// Volume discount: 5% off for 100+ lb (PRD 8.6)
	volumeDiscount := 0.0
	if billable >= VolumeDiscountLb {
		volumeDiscount = base * VolumeDiscountPct
		base -= volumeDiscount
	}

	surcharge := 0.0
	doorToDoor := 0.0
	switch in.Service {
	case "express":
		surcharge = base * ExpressSurchargePct
	case "door_to_door":
		doorToDoor = DoorToDoorFee
	}

	insurance := 0.0
	if in.Insurance && in.DeclaredValue > 0 {
		insurance = in.DeclaredValue / 100.0
	}

	total := base + surcharge + doorToDoor + insurance
	minApplied := false
	if total < MinCharge {
		total = MinCharge
		minApplied = true
	}

	return ShippingResult{
		DestinationID:  in.Destination,
		Service:        in.Service,
		ActualWeight:    in.ActualWeight,
		DimWeight:       math.Round(dim*100) / 100,
		BillableWeight:  billable,
		RatePerLb:       rate,
		BaseCost:        math.Round(base*100) / 100,
		Surcharge:       math.Round(surcharge*100) / 100,
		DoorToDoorFee:   doorToDoor,
		Insurance:       math.Round(insurance*100) / 100,
		VolumeDiscount:  math.Round(volumeDiscount*100) / 100,
		Total:           math.Round(total*100) / 100,
		MinimumApplied:  minApplied,
	}, true
}
