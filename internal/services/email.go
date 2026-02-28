package services

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/resend/resend-go/v3"
)

const supportEmail = "support@qcs-cargo.com"

var missingResendKeyLogOnce sync.Once

// fromAddress returns the sender email for transactional mail. Prefers FROM_EMAIL env; otherwise Resend sandbox.
func fromAddress() string {
	if s := os.Getenv("FROM_EMAIL"); s != "" {
		return s
	}
	return "onboarding@resend.dev"
}

// resendClient returns a Resend client if RESEND_API_KEY is set, otherwise nil.
func resendClient() *resend.Client {
	key := os.Getenv("RESEND_API_KEY")
	if key == "" {
		missingResendKeyLogOnce.Do(func() {
			log.Print("[email] RESEND_API_KEY not set; email sends are no-op")
		})
		return nil
	}
	return resend.NewClient(key)
}

// appURL returns the base URL of the application, preferring APP_URL env; otherwise production default.
func appURL() string {
	if s := os.Getenv("APP_URL"); s != "" {
		return s
	}
	return "https://qcs-cargo.com"
}

// SendMagicLink sends a magic link email to the given address. Link is valid 10 minutes.
// No-op if RESEND_API_KEY is not set; callers should fall back to logging.
func SendMagicLink(to, link string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	from := fromAddress()
	subject := "Sign in to QCS Cargo"
	html := fmt.Sprintf(`<p>Click the link below to sign in. This link expires in 10 minutes.</p><p><a href="%s">Sign in to QCS Cargo</a></p><p>If you didn't request this, you can ignore this email.</p>`, link)
	text := fmt.Sprintf("Click to sign in (valid 10 min): %s", link)
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + from + ">",
		To:      []string{to},
		Subject: subject,
		Html:    html,
		Text:    text,
	})
	return err
}

// SendContactFormSubmission sends the contact form payload to support@qcs-cargo.com.
// subject may be empty; body is the message. No-op if RESEND_API_KEY is not set.
func SendContactFormSubmission(fromName, fromEmail, subject, body string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	subj := "Contact form: QCS Cargo"
	if subject != "" {
		subj = "Contact form: " + subject
	}
	html := fmt.Sprintf("<p><strong>From:</strong> %s &lt;%s&gt;</p><p><strong>Subject:</strong> %s</p><hr/><pre>%s</pre>",
		escapeHTML(fromName), escapeHTML(fromEmail), escapeHTML(subject), escapeHTML(body))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{supportEmail},
		ReplyTo: fromEmail,
		Subject: subj,
		Html:    html,
	})
	return err
}

// SendPasswordResetLink sends the password reset link to the user. Link is valid 1 hour.
// No-op if RESEND_API_KEY is not set.
func SendPasswordResetLink(to, link string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	subject := "Reset your QCS Cargo password"
	html := fmt.Sprintf(`<p>You requested a password reset. Click the link below to set a new password. This link expires in 1 hour.</p><p><a href="%s">Reset password</a></p><p>If you didn't request this, you can ignore this email.</p>`, link)
	text := fmt.Sprintf("Reset your password (valid 1 hour): %s", link)
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: subject,
		Html:    html,
		Text:    text,
	})
	return err
}

// SendVerificationEmail sends the account verification link to the user.
func SendVerificationEmail(to, link string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	subject := "Verify your QCS Cargo email"
	html := fmt.Sprintf(`<p>Welcome to QCS Cargo. Please verify your email address by clicking the link below. This link expires in 24 hours.</p><p><a href="%s">Verify email</a></p><p>If you didn't create this account, you can ignore this email.</p>`, link)
	text := fmt.Sprintf("Verify your email (valid 24 hours): %s", link)
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: subject,
		Html:    html,
		Text:    text,
	})
	return err
}

// --- PRD 10.1 lifecycle templates (call from warehouse/API when events occur) ---

// SendPackageArrived notifies customer when staff completes locker receive.
func SendPackageArrived(to, senderName string, weightLbs float64) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("New package from %s! Weight: %.1f lbs. Log in to view.", escapeHTML(senderName), weightLbs)
	dashboardLink := appURL() + "/dashboard/inbox"
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "New package at QCS Cargo",
		Html:    fmt.Sprintf("<p>%s</p><p><a href=\"%s\">View in My Packages</a></p>", body, dashboardLink),
		Text:    body,
	})
	return err
}

// SendPhotoReady notifies when photo service is completed.
func SendPhotoReady(to, senderName string, photoCount int) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("%d photos of your package from %s are ready.", photoCount, escapeHTML(senderName))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Your package photos are ready",
		Html:    "<p>" + body + "</p>",
		Text:    body,
	})
	return err
}

// SendServiceComplete notifies when any value-added service is completed.
func SendServiceComplete(to, serviceName, senderName string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("%s for package from %s is complete.", serviceName, escapeHTML(senderName))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Service complete – QCS Cargo",
		Html:    "<p>" + body + "</p>",
		Text:    body,
	})
	return err
}

// SendStorageWarning5Days notifies 5 days before free storage ends.
func SendStorageWarning5Days(to, senderName string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("Package from %s starts storage fees in 5 days.", escapeHTML(senderName))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Storage reminder – QCS Cargo",
		Html:    "<p>" + body + "</p>",
		Text:    body,
	})
	return err
}

// SendStorageWarning1Day notifies last day of free storage.
func SendStorageWarning1Day(to, senderName string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("Last day of free storage for %s package.", escapeHTML(senderName))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Last day of free storage – QCS Cargo",
		Html:    "<p>" + body + "</p>",
		Text:    body,
	})
	return err
}

// SendStorageFeeCharged notifies when daily storage fee is charged.
func SendStorageFeeCharged(to, senderName string, amount float64) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("Storage fee $%.2f charged for %s package.", amount, escapeHTML(senderName))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Storage fee charged – QCS Cargo",
		Html:    "<p>" + body + "</p>",
		Text:    body,
	})
	return err
}

// SendStorageFinalNotice notifies at day 55 (5 days before disposal).
func SendStorageFinalNotice(to, senderName string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("Package from %s will be disposed in 5 days unless shipped.", escapeHTML(senderName))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Final notice – package disposal – QCS Cargo",
		Html:    "<p>" + body + "</p>",
		Text:    body,
	})
	return err
}

// SendShipRequestPaid notifies when payment succeeds (e.g. after Stripe webhook).
func SendShipRequestPaid(to, code string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("Ship request %s confirmed! Being prepared.", escapeHTML(code))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Ship request confirmed – QCS Cargo",
		Html:    "<p>" + body + "</p>",
		Text:    body,
	})
	return err
}

// SendShipRequestShipped notifies when warehouse marks shipped.
func SendShipRequestShipped(to, code, tracking string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("Shipment %s on its way! Tracking: %s", escapeHTML(code), escapeHTML(tracking))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Shipment on its way – QCS Cargo",
		Html:    "<p>" + body + "</p>",
		Text:    body,
	})
	return err
}

// SendShipRequestDelivered notifies when carrier confirms delivery.
func SendShipRequestDelivered(to, code, destination string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("Shipment %s delivered in %s!", escapeHTML(code), escapeHTML(destination))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Shipment delivered – QCS Cargo",
		Html:    "<p>" + body + "</p>",
		Text:    body,
	})
	return err
}

// SendInboundDelivered notifies when tracking shows delivered to QCS.
func SendInboundDelivered(to, retailerName string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("%s package delivered to QCS. Check locker shortly.", escapeHTML(retailerName))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Package arrived at QCS Cargo",
		Html:    "<p>" + body + "</p>",
		Text:    body,
	})
	return err
}

// SendCustomsHold notifies when ship request needs customs attention.
func SendCustomsHold(to, code string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("Ship request %s needs customs attention.", escapeHTML(code))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Customs notice – QCS Cargo",
		Html:    "<p>" + body + "</p>",
		Text:    body,
	})
	return err
}

// SendBookingConfirmed notifies when booking payment succeeds.
func SendBookingConfirmed(to, code, date string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("Booking %s confirmed for %s.", escapeHTML(code), escapeHTML(date))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Booking confirmed – QCS Cargo",
		Html:    "<p>" + body + "</p>",
		Text:    body,
	})
	return err
}

// SendShipmentStatus notifies on shipment status change.
func SendShipmentStatus(to, trackingNumber, status string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := fmt.Sprintf("Shipment %s: now %s", escapeHTML(trackingNumber), escapeHTML(status))
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Shipment update – QCS Cargo",
		Html:    "<p>" + body + "</p>",
		Text:    body,
	})
	return err
}

func escapeHTML(s string) string {
	var out []byte
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			out = append(out, '&', 'a', 'm', 'p', ';')
		case '<':
			out = append(out, '&', 'l', 't', ';')
		case '>':
			out = append(out, '&', 'g', 't', ';')
		case '"':
			out = append(out, '&', 'q', 'u', 'o', 't', ';')
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}
