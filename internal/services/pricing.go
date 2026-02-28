package services

import (
	"math"
)

// Pricing constants
const (
	DimWeightDivisor    = 166.0
	MinCharge           = 10.0
	DoorToDoorFee       = 25.0
	ExpressSurchargePct = 0.25
	DefaultRatePerLb    = 4.50
	InsuranceRate       = 0.01
	DiscountAt100Lbs    = 0.05
	DiscountAt250Lbs    = 0.10
	DiscountAt500Lbs    = 0.15
	DiscountAt1000Lbs   = 0.20
)

var Rates = map[string]float64{
	"guyana":   3.50,
	"jamaica":  3.75,
	"trinidad": 3.50,
	"barbados": 4.00,
	"suriname": 4.25,
}

type PricingInput struct {
	DestinationID string
	WeightLbs     float64
	LengthIn      float64
	WidthIn       float64
	HeightIn      float64
	ServiceType   string // standard, express, door_to_door
	ValueUSD      float64
	AddInsurance  bool
}

type PricingResult struct {
	DestinationID  string
	Service        string
	ActualWeight   float64
	DimWeight      float64
	BillableWeight float64
	RatePerLb      float64
	Subtotal       float64
	ServiceFees    float64
	Insurance      float64
	Discount       float64
	Total          float64
	MinimumApplied bool
}

func CalculatePricing(in PricingInput) PricingResult {
	rate, ok := Rates[in.DestinationID]
	if !ok {
		rate = DefaultRatePerLb
	}

	dimWeight := 0.0
	if in.LengthIn > 0 && in.WidthIn > 0 && in.HeightIn > 0 {
		dimWeight = (in.LengthIn * in.WidthIn * in.HeightIn) / DimWeightDivisor
	}

	billableWeight := math.Max(in.WeightLbs, dimWeight)
	baseCost := billableWeight * rate

	serviceFees := 0.0
	if in.ServiceType == "express" {
		serviceFees = baseCost * ExpressSurchargePct
	} else if in.ServiceType == "door_to_door" {
		serviceFees = DoorToDoorFee
	}

	insurance := 0.0
	if in.AddInsurance {
		insurance = in.ValueUSD * InsuranceRate
	}

	// Volume Discounts (PRD 5.1/8.2 and Pricing page)
	discountPercent := 0.0
	if billableWeight >= 1000 {
		discountPercent = DiscountAt1000Lbs
	} else if billableWeight >= 500 {
		discountPercent = DiscountAt500Lbs
	} else if billableWeight >= 250 {
		discountPercent = DiscountAt250Lbs
	} else if billableWeight >= 100 {
		discountPercent = DiscountAt100Lbs
	}

	discount := (baseCost + serviceFees) * discountPercent

	subtotal := baseCost
	total := subtotal + serviceFees + insurance - discount

	// Minimum charge $10
	minimumApplied := false
	if total < MinCharge {
		total = MinCharge
		minimumApplied = true
	}

	return PricingResult{
		DestinationID:  in.DestinationID,
		Service:        in.ServiceType,
		ActualWeight:   in.WeightLbs,
		DimWeight:      math.Round(dimWeight*100) / 100,
		BillableWeight: math.Round(billableWeight*100) / 100,
		RatePerLb:      rate,
		Subtotal:       math.Round(subtotal*100) / 100,
		ServiceFees:    math.Round(serviceFees*100) / 100,
		Insurance:      math.Round(insurance*100) / 100,
		Discount:       math.Round(discount*100) / 100,
		Total:          math.Round(total*100) / 100,
		MinimumApplied: minimumApplied,
	}
}
