package calc_test

import (
	"testing"

	"github.com/Qcsinc23/qcs-cargo/internal/calc"
	"github.com/Qcsinc23/qcs-cargo/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDimensionalWeight(t *testing.T) {
	tests := []struct {
		name    string
		l, w, h float64
		want    float64
	}{
		{"standard box", 12, 12, 12, 10.41},
		{"flat envelope", 15, 12, 1, 1.08},
		{"zero dimension", 0, 12, 12, 0},
		{"large box", 24, 24, 24, 83.28},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.DimensionalWeight(tt.l, tt.w, tt.h)
			assert.InDelta(t, tt.want, got, 0.01)
		})
	}
}

func TestBillableWeight(t *testing.T) {
	assert.Equal(t, 10.0, calc.BillableWeight(5.0, 10.0))
	assert.Equal(t, 5.0, calc.BillableWeight(5.0, 3.0))
	assert.Equal(t, 5.0, calc.BillableWeight(5.0, 0))
}

func TestRateForDestination(t *testing.T) {
	r, ok := calc.RateForDestination("guyana")
	require.True(t, ok)
	assert.Equal(t, 3.50, r)
	_, ok = calc.RateForDestination("unknown")
	assert.False(t, ok)
}

func TestCalculateShipping(t *testing.T) {
	tests := []struct {
		name      string
		input     calc.ShippingInput
		wantTotal float64
	}{
		{
			name: "standard 5lb to Guyana",
			input: calc.ShippingInput{
				Destination:   "guyana",
				Service:       "standard",
				ActualWeight:  5.0,
				DeclaredValue: 100,
				Insurance:     false,
			},
			wantTotal: 17.50,
		},
		{
			name: "express 5lb to Guyana",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "express",
				ActualWeight: 5.0,
			},
			wantTotal: 21.88,
		},
		{
			name: "door-to-door 5lb to Guyana",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "door_to_door",
				ActualWeight: 5.0,
			},
			wantTotal: 42.50,
		},
		{
			name: "dimensional weight exceeds actual",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "standard",
				ActualWeight: 5.0,
				Length:       24, Width: 24, Height: 24,
			},
			wantTotal: 291.48,
		},
		{
			name: "volume discount 150lb",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "standard",
				ActualWeight: 150.0,
			},
			wantTotal: 498.75,
		},
		{
			name: "minimum charge",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "standard",
				ActualWeight: 0.5,
			},
			wantTotal: 10.00,
		},
		{
			name: "insurance",
			input: calc.ShippingInput{
				Destination:   "guyana",
				Service:       "standard",
				ActualWeight:  5.0,
				DeclaredValue: 500,
				Insurance:     true,
			},
			wantTotal: 22.50,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := calc.CalculateShipping(tt.input)
			require.True(t, ok)
			assert.InDelta(t, tt.wantTotal, result.Total, 0.01)
		})
	}
}

func TestCalculateShipping_Invalid(t *testing.T) {
	_, ok := calc.CalculateShipping(calc.ShippingInput{Destination: "unknown", ActualWeight: 5})
	assert.False(t, ok)
	_, ok = calc.CalculateShipping(calc.ShippingInput{Destination: "guyana", ActualWeight: 0})
	assert.False(t, ok)
}

func TestCalculateShipping_ParityWithServices(t *testing.T) {
	tests := []struct {
		name  string
		input calc.ShippingInput
	}{
		{
			name: "standard with insurance",
			input: calc.ShippingInput{
				Destination:   "jamaica",
				Service:       "standard",
				ActualWeight:  8.5,
				DeclaredValue: 300,
				Insurance:     true,
			},
		},
		{
			name: "dimensional over actual",
			input: calc.ShippingInput{
				Destination:  "barbados",
				Service:      "standard",
				ActualWeight: 10,
				Length:       24,
				Width:        24,
				Height:       24,
			},
		},
		{
			name: "express with volume discount",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "express",
				ActualWeight: 150,
			},
		},
		{
			name: "door to door with volume discount",
			input: calc.ShippingInput{
				Destination:  "guyana",
				Service:      "door_to_door",
				ActualWeight: 150,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calcResult, ok := calc.CalculateShipping(tt.input)
			require.True(t, ok)

			pricingResult := services.CalculatePricing(services.PricingInput{
				DestinationID: tt.input.Destination,
				WeightLbs:     tt.input.ActualWeight,
				LengthIn:      tt.input.Length,
				WidthIn:       tt.input.Width,
				HeightIn:      tt.input.Height,
				ServiceType:   tt.input.Service,
				ValueUSD:      tt.input.DeclaredValue,
				AddInsurance:  tt.input.Insurance,
			})

			assert.Equal(t, pricingResult.DestinationID, calcResult.DestinationID)
			assert.Equal(t, pricingResult.Service, calcResult.Service)
			assert.InDelta(t, pricingResult.ActualWeight, calcResult.ActualWeight, 0.01)
			assert.InDelta(t, pricingResult.DimWeight, calcResult.DimWeight, 0.01)
			assert.InDelta(t, pricingResult.BillableWeight, calcResult.BillableWeight, 0.01)
			assert.InDelta(t, pricingResult.RatePerLb, calcResult.RatePerLb, 0.01)
			assert.InDelta(t, pricingResult.Subtotal, calcResult.BaseCost, 0.01)
			assert.InDelta(t, pricingResult.Insurance, calcResult.Insurance, 0.01)
			assert.InDelta(t, pricingResult.Discount, calcResult.VolumeDiscount, 0.01)
			assert.InDelta(t, pricingResult.Total, calcResult.Total, 0.01)
			assert.Equal(t, pricingResult.MinimumApplied, calcResult.MinimumApplied)

			switch tt.input.Service {
			case "express":
				assert.InDelta(t, pricingResult.ServiceFees, calcResult.Surcharge, 0.01)
				assert.InDelta(t, 0.0, calcResult.DoorToDoorFee, 0.01)
			case "door_to_door":
				assert.InDelta(t, pricingResult.ServiceFees, calcResult.DoorToDoorFee, 0.01)
				assert.InDelta(t, 0.0, calcResult.Surcharge, 0.01)
			default:
				assert.InDelta(t, 0.0, calcResult.Surcharge, 0.01)
				assert.InDelta(t, 0.0, calcResult.DoorToDoorFee, 0.01)
			}
		})
	}
}

func TestCalculateShipping_DoorToDoorDiscountOrderParity(t *testing.T) {
	in := calc.ShippingInput{
		Destination:  "guyana",
		Service:      "door_to_door",
		ActualWeight: 150,
	}

	result, ok := calc.CalculateShipping(in)
	require.True(t, ok)

	assert.InDelta(t, 27.50, result.VolumeDiscount, 0.01)
	assert.InDelta(t, 522.50, result.Total, 0.01)
}
