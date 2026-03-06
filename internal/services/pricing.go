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

type ShipmentPackageInput struct {
	WeightLbs float64
	LengthIn  float64
	WidthIn   float64
	HeightIn  float64
}

func CalculatePricing(in PricingInput) PricingResult {
	dimWeight := 0.0
	if in.LengthIn > 0 && in.WidthIn > 0 && in.HeightIn > 0 {
		dimWeight = (in.LengthIn * in.WidthIn * in.HeightIn) / DimWeightDivisor
	}

	return calculatePricingTotals(in.DestinationID, in.ServiceType, in.WeightLbs, dimWeight, in.ValueUSD, in.AddInsurance)
}

func CalculateShipmentPricing(destinationID, serviceType string, packages []ShipmentPackageInput) PricingResult {
	totalActualWeight := 0.0
	totalDimWeight := 0.0
	for _, pkg := range packages {
		totalActualWeight += pkg.WeightLbs
		if pkg.LengthIn > 0 && pkg.WidthIn > 0 && pkg.HeightIn > 0 {
			totalDimWeight += (pkg.LengthIn * pkg.WidthIn * pkg.HeightIn) / DimWeightDivisor
		}
	}

	return calculatePricingTotals(destinationID, serviceType, totalActualWeight, totalDimWeight, 0, false)
}

func calculatePricingTotals(destinationID, serviceType string, actualWeight, dimWeight, valueUSD float64, addInsurance bool) PricingResult {
	rate, ok := Rates[destinationID]
	if !ok {
		rate = DefaultRatePerLb
	}

	billableWeight := math.Max(actualWeight, dimWeight)
	baseCost := billableWeight * rate

	serviceFees := 0.0
	if serviceType == "express" {
		serviceFees = baseCost * ExpressSurchargePct
	} else if serviceType == "door_to_door" {
		serviceFees = DoorToDoorFee
	}

	insurance := 0.0
	if addInsurance {
		insurance = valueUSD * InsuranceRate
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
		DestinationID:  destinationID,
		Service:        serviceType,
		ActualWeight:   actualWeight,
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
