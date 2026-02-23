package services

import (
	"math"
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
	Subtotal       float64
	ServiceFees    float64
	Insurance      float64
	Discount       float64
	Total          float64
	BillableWeight float64
}

func CalculatePricing(in PricingInput) PricingResult {
	rate, ok := Rates[in.DestinationID]
	if !ok {
		rate = 4.50 // Default/Fallback rate
	}

	dimWeight := 0.0
	if in.LengthIn > 0 && in.WidthIn > 0 && in.HeightIn > 0 {
		dimWeight = (in.LengthIn * in.WidthIn * in.HeightIn) / 166.0
	}

	billableWeight := math.Max(in.WeightLbs, dimWeight)
	baseCost := billableWeight * rate

	serviceFees := 0.0
	if in.ServiceType == "express" {
		serviceFees = baseCost * 0.25
	} else if in.ServiceType == "door_to_door" {
		serviceFees = 25.0
	}

	insurance := 0.0
	if in.AddInsurance {
		insurance = in.ValueUSD / 100.0
	}

	// Volume Discounts (PRD 5.1/8.2 and Pricing page)
	discountPercent := 0.0
	if billableWeight >= 1000 {
		discountPercent = 0.20
	} else if billableWeight >= 500 {
		discountPercent = 0.15
	} else if billableWeight >= 250 {
		discountPercent = 0.10
	} else if billableWeight >= 100 {
		discountPercent = 0.05
	}

	discount := (baseCost + serviceFees) * discountPercent

	subtotal := baseCost
	total := subtotal + serviceFees + insurance - discount

	// Minimum charge $10
	if total < 10.0 {
		total = 10.0
	}

	return PricingResult{
		Subtotal:       math.Round(subtotal*100) / 100,
		ServiceFees:    math.Round(serviceFees*100) / 100,
		Insurance:      math.Round(insurance*100) / 100,
		Discount:       math.Round(discount*100) / 100,
		Total:          math.Round(total*100) / 100,
		BillableWeight: math.Round(billableWeight*100) / 100,
	}
}
