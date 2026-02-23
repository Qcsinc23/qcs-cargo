//go:build js && wasm

// This file is reference only: it is not compiled with the current WASM build.
// The app currently uses a simple syscall/js wrapper in main.go, not go-app.
// When you add go-app and create page components (Home, HowItWorks, Services, etc.),
// add the dependency and either remove this build tag so this file compiles,
// or copy the app.Img() snippets from here into your components.
//
// Images are under frontend/static/images/ and served at /web/images/ when the
// server is run (see cmd/server and web/images).
//
// IDE may show "could not import go-app": this file is reference-only (go:build ignore).
// Add require github.com/maxence-charriere/go-app/v10 to the root go.mod when you use go-app.
package main

import "github.com/maxence-charriere/go-app/v10/pkg/app"

// Image placeholder components for generated images (see frontend/static/images/MANIFEST.md).
// Use these in go-app page components once they exist.

// ==========================================
// 1. Home Page (/)
// ==========================================

// IMAGE: frontend/static/images/heroes/hero-home.png
func HomeHeroImg() app.UI {
	return app.Img().
		Src("/web/images/heroes/hero-home.png").
		Alt("Packages flowing from the US to the Caribbean via air freight").
		Class("w-full h-auto object-cover rounded-xl shadow-lg")
}

// IMAGE: frontend/static/images/steps/step-get-address.png
func StepGetAddressImg() app.UI {
	return app.Img().
		Src("/web/images/steps/step-get-address.png").
		Alt("Get Your Address - Mailbox with a suite code label").
		Class("w-full max-w-[480px] h-auto mx-auto drop-shadow-md")
}

// IMAGE: frontend/static/images/steps/step-shop-anywhere.png
func StepShopAnywhereImg() app.UI {
	return app.Img().
		Src("/web/images/steps/step-shop-anywhere.png").
		Alt("Shop Anywhere - Laptop or phone screen showing shopping bags").
		Class("w-full max-w-[480px] h-auto mx-auto drop-shadow-md")
}

// IMAGE: frontend/static/images/steps/step-receive-store.png
func StepReceiveStoreImg() app.UI {
	return app.Img().
		Src("/web/images/steps/step-receive-store.png").
		Alt("We Receive & Store - Warehouse shelf with packages, camera, and scale").
		Class("w-full max-w-[480px] h-auto mx-auto drop-shadow-md")
}

// IMAGE: frontend/static/images/steps/step-ship-ready.png
func StepShipReadyImg() app.UI {
	return app.Img().
		Src("/web/images/steps/step-ship-ready.png").
		Alt("Ship When Ready - Airplane carrying a package towards Caribbean islands").
		Class("w-full max-w-[480px] h-auto mx-auto drop-shadow-md")
}

// Destinations (Icons / Spot Illustrations for Home Page)

// IMAGE: frontend/static/images/destinations/dest-guyana.png
func DestGuyanaImg() app.UI {
	return app.Img().
		Src("/web/images/destinations/dest-guyana.png").
		Alt("Guyana - Stabroek Market or Kaieteur Falls").
		Class("w-full max-w-[400px] h-auto mx-auto drop-shadow-sm")
}

// IMAGE: frontend/static/images/destinations/dest-jamaica.png
func DestJamaicaImg() app.UI {
	return app.Img().
		Src("/web/images/destinations/dest-jamaica.png").
		Alt("Jamaica - Blue Mountains or Jamaican theme").
		Class("w-full max-w-[400px] h-auto mx-auto drop-shadow-sm")
}

// IMAGE: frontend/static/images/destinations/dest-trinidad.png
func DestTrinidadImg() app.UI {
	return app.Img().
		Src("/web/images/destinations/dest-trinidad.png").
		Alt("Trinidad & Tobago - Twin Towers or Carnival colors").
		Class("w-full max-w-[400px] h-auto mx-auto drop-shadow-sm")
}

// IMAGE: frontend/static/images/destinations/dest-barbados.png
func DestBarbadosImg() app.UI {
	return app.Img().
		Src("/web/images/destinations/dest-barbados.png").
		Alt("Barbados - Beach or lighthouse").
		Class("w-full max-w-[400px] h-auto mx-auto drop-shadow-sm")
}

// IMAGE: frontend/static/images/destinations/dest-suriname.png
func DestSurinameImg() app.UI {
	return app.Img().
		Src("/web/images/destinations/dest-suriname.png").
		Alt("Suriname - River or colonial architecture").
		Class("w-full max-w-[400px] h-auto mx-auto drop-shadow-sm")
}

// ==========================================
// 2. How It Works Page (/how-it-works)
// ==========================================

// IMAGE: frontend/static/images/heroes/hero-how-it-works.png
func HowItWorksHeroImg() app.UI {
	return app.Img().
		Src("/web/images/heroes/hero-how-it-works.png").
		Alt("Visual timeline of the shipping journey to the Caribbean").
		Class("w-full h-auto object-cover rounded-xl shadow-lg")
}

// ==========================================
// 3. Services Page (/services)
// ==========================================

// IMAGE: frontend/static/images/services/svc-air-freight.png
func SvcAirFreightImg() app.UI {
	return app.Img().
		Src("/web/images/services/svc-air-freight.png").
		Alt("Standard Air Freight - Airplane with package").
		Class("w-full max-w-[400px] h-auto mx-auto")
}

// IMAGE: frontend/static/images/services/svc-express.png
func SvcExpressImg() app.UI {
	return app.Img().
		Src("/web/images/services/svc-express.png").
		Alt("Express Delivery - Airplane with speed lines").
		Class("w-full max-w-[400px] h-auto mx-auto")
}

// IMAGE: frontend/static/images/services/svc-door-to-door.png
func SvcDoorToDoorImg() app.UI {
	return app.Img().
		Src("/web/images/services/svc-door-to-door.png").
		Alt("Door-to-Door - Package on a doorstep").
		Class("w-full max-w-[400px] h-auto mx-auto")
}

// IMAGE: frontend/static/images/services/svc-consolidation.png
func SvcConsolidationImg() app.UI {
	return app.Img().
		Src("/web/images/services/svc-consolidation.png").
		Alt("Package Consolidation - Multiple small boxes merging").
		Class("w-full max-w-[400px] h-auto mx-auto")
}

// IMAGE: frontend/static/images/services/svc-inspection.png
func SvcInspectionImg() app.UI {
	return app.Img().
		Src("/web/images/services/svc-inspection.png").
		Alt("Content Inspection - Open box with magnifying glass and camera").
		Class("w-full max-w-[400px] h-auto mx-auto")
}

// IMAGE: frontend/static/images/services/svc-repackage.png
func SvcRepackageImg() app.UI {
	return app.Img().
		Src("/web/images/services/svc-repackage.png").
		Alt("Repackaging - Box being unwrapped and rewrapped").
		Class("w-full max-w-[400px] h-auto mx-auto")
}

// IMAGE: frontend/static/images/services/svc-storage.png
func SvcStorageImg() app.UI {
	return app.Img().
		Src("/web/images/services/svc-storage.png").
		Alt("Free Storage - Calendar next to warehouse shelf").
		Class("w-full max-w-[400px] h-auto mx-auto")
}

// IMAGE: frontend/static/images/services/svc-customs.png
func SvcCustomsImg() app.UI {
	return app.Img().
		Src("/web/images/services/svc-customs.png").
		Alt("Customs Clearance - Clipboard with checkmarks and customs stamp").
		Class("w-full max-w-[400px] h-auto mx-auto")
}

// ==========================================
// 4. Pricing Page (/pricing)
// ==========================================

// IMAGE: frontend/static/images/heroes/hero-pricing.png
func PricingHeroImg() app.UI {
	return app.Img().
		Src("/web/images/heroes/hero-pricing.png").
		Alt("Pricing - Calculator and receipt with package and price tags").
		Class("w-full h-auto object-cover rounded-xl shadow-lg")
}

// ==========================================
// 5. About Page (/about)
// ==========================================

// IMAGE: frontend/static/images/heroes/hero-about.png
func AboutHeroImg() app.UI {
	return app.Img().
		Src("/web/images/heroes/hero-about.png").
		Alt("About Us - Warehouse staff organizing packages").
		Class("w-full h-auto object-cover rounded-xl shadow-lg")
}

// IMAGE: frontend/static/images/about/about-mission.png
func AboutMissionImg() app.UI {
	return app.Img().
		Src("/web/images/about/about-mission.png").
		Alt("Our Mission - Globe with US and Caribbean highlighted").
		Class("w-full max-w-[600px] h-auto mx-auto")
}

// IMAGE: frontend/static/images/about/about-team.png
func AboutTeamImg() app.UI {
	return app.Img().
		Src("/web/images/about/about-team.png").
		Alt("Our Team - Diverse people working together").
		Class("w-full max-w-[800px] h-auto mx-auto rounded-xl")
}

// ==========================================
// 6. Contact Page (/contact)
// ==========================================

// IMAGE: frontend/static/images/heroes/hero-contact.png
func ContactHeroImg() app.UI {
	return app.Img().
		Src("/web/images/heroes/hero-contact.png").
		Alt("Contact Us - Chat bubble, envelope, and phone").
		Class("w-full h-auto object-cover rounded-xl shadow-lg")
}

// IMAGE: frontend/static/images/contact/contact-map.png
func ContactMapImg() app.UI {
	return app.Img().
		Src("/web/images/contact/contact-map.png").
		Alt("Location Map - Stylized map pin on street grid").
		Class("w-full max-w-[600px] h-auto mx-auto rounded-xl shadow-md")
}

// ==========================================
// 7. FAQ Page (/faq)
// ==========================================

// IMAGE: frontend/static/images/heroes/hero-faq.png
func FAQHeroImg() app.UI {
	return app.Img().
		Src("/web/images/heroes/hero-faq.png").
		Alt("Frequently Asked Questions - Person with question mark and answer bubbles").
		Class("w-full h-auto object-cover rounded-xl shadow-lg")
}

// ==========================================
// 8. Track Shipment Page (/track)
// ==========================================

// IMAGE: frontend/static/images/heroes/hero-track.png
func TrackHeroImg() app.UI {
	return app.Img().
		Src("/web/images/heroes/hero-track.png").
		Alt("Track Shipment - Package with location pin and dotted path").
		Class("w-full h-auto object-cover rounded-xl shadow-lg")
}

// IMAGE: frontend/static/images/track/track-empty.png
func TrackEmptyImg() app.UI {
	return app.Img().
		Src("/web/images/track/track-empty.png").
		Alt("No packages being tracked - Friendly magnifying glass").
		Class("w-full max-w-[400px] h-auto mx-auto")
}

// ==========================================
// 9. Shipping Calculator Page (/shipping-calculator)
// ==========================================

// IMAGE: frontend/static/images/heroes/hero-calculator.png
func CalculatorHeroImg() app.UI {
	return app.Img().
		Src("/web/images/heroes/hero-calculator.png").
		Alt("Shipping Calculator - Scale, ruler, and calculator").
		Class("w-full h-auto object-cover rounded-xl shadow-lg")
}

// ==========================================
// 10. Destinations Hub (/destinations)
// ==========================================

// IMAGE: frontend/static/images/heroes/hero-destinations.png
func DestinationsHeroImg() app.UI {
	return app.Img().
		Src("/web/images/heroes/hero-destinations.png").
		Alt("Destinations - 5 Caribbean destinations connected by airplane arcs").
		Class("w-full h-auto object-cover rounded-xl shadow-lg")
}

// ==========================================
// 11. Prohibited Items Page (/prohibited-items)
// ==========================================

// IMAGE: frontend/static/images/heroes/hero-prohibited.png
func ProhibitedHeroImg() app.UI {
	return app.Img().
		Src("/web/images/heroes/hero-prohibited.png").
		Alt("Prohibited Items - Package with prohibition signs").
		Class("w-full h-auto object-cover rounded-xl shadow-lg")
}

// To do: Add images that failed generation due to rate limit
