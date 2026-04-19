package services

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/resend/resend-go/v3"
)

const supportEmail = "support@qcs-cargo.com"

var missingResendKeyLogOnce sync.Once

func fromAddress() string {
	if s := os.Getenv("FROM_EMAIL"); s != "" {
		return s
	}
	return "onboarding@resend.dev"
}

// fakeResendKeyPrefixes are obvious test fixtures; treat them as no-op so we
// do not make real HTTP calls during integration tests. Pass 2 audit follow-up:
// stabilizes test runs under the race detector where the SDK's HTTPS attempt
// to a placeholder key adds enough latency to trip Fiber's 1s test timeout.
var fakeResendKeyPrefixes = []string{"re_test_fake", "re_fake_", "re_dummy_"}

func resendClient() *resend.Client {
	key := strings.TrimSpace(os.Getenv("RESEND_API_KEY"))
	if key == "" {
		missingResendKeyLogOnce.Do(func() {
			log.Print("[email] RESEND_API_KEY not set; email sends are no-op")
		})
		return nil
	}
	for _, p := range fakeResendKeyPrefixes {
		if strings.HasPrefix(key, p) {
			return nil
		}
	}
	return resend.NewClient(key)
}

func appURL() string {
	if s := os.Getenv("APP_URL"); s != "" {
		return s
	}
	return "https://qcs-cargo.com"
}

// ---------------------------------------------------------------------------
// Branded email layout
// ---------------------------------------------------------------------------

const brandNavy = "#0F172A"
const brandBlue = "#2563EB"
const brandOrange = "#F97316"

func emailLayout(preheader, heading, body string) string {
	base := appURL()
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" xmlns="http://www.w3.org/1999/xhtml">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
<meta name="color-scheme" content="light"/>
<meta name="supported-color-schemes" content="light"/>
<title>QCS Cargo</title>
<!--[if mso]><noscript><xml><o:OfficeDocumentSettings><o:PixelsPerInch>96</o:PixelsPerInch></o:OfficeDocumentSettings></xml></noscript><![endif]-->
</head>
<body style="margin:0;padding:0;background-color:#F1F5F9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;-webkit-font-smoothing:antialiased;">
<div style="display:none;font-size:1px;color:#F1F5F9;line-height:1px;max-height:0;max-width:0;opacity:0;overflow:hidden;">%s</div>
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:#F1F5F9;">
<tr><td align="center" style="padding:40px 16px;">
<table role="presentation" width="600" cellpadding="0" cellspacing="0" border="0" style="max-width:600px;width:100%%;background-color:#ffffff;border-radius:12px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,0.08);">

<!-- Header -->
<tr><td style="background-color:%s;padding:28px 40px;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
<tr>
<td><a href="%s" style="color:#ffffff;text-decoration:none;font-size:20px;font-weight:700;letter-spacing:-0.3px;">QCS&nbsp;Cargo</a></td>
<td align="right"><span style="color:#94A3B8;font-size:12px;text-transform:uppercase;letter-spacing:1px;">%s</span></td>
</tr>
</table>
</td></tr>

<!-- Body -->
<tr><td style="padding:40px;">
%s
</td></tr>

<!-- Footer -->
<tr><td style="border-top:1px solid #E2E8F0;padding:24px 40px;background-color:#F8FAFC;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
<tr><td style="color:#64748B;font-size:12px;line-height:20px;">
<p style="margin:0 0 4px;">QCS Cargo &middot; 35 Obrien St, E12 &middot; Kearny, NJ 07032</p>
<p style="margin:0;"><a href="%s/dashboard" style="color:%s;text-decoration:none;">Dashboard</a> &nbsp;&middot;&nbsp; <a href="mailto:%s" style="color:%s;text-decoration:none;">Contact support</a></p>
</td></tr>
</table>
</td></tr>

</table>
</td></tr>
</table>
</body>
</html>`, preheader, brandNavy, base, heading, body, base, brandBlue, supportEmail, brandBlue)
}

func emailButton(href, label, color string) string {
	if color == "" {
		color = brandBlue
	}
	return fmt.Sprintf(`<table role="presentation" cellpadding="0" cellspacing="0" border="0" style="margin:28px 0 8px;">
<tr><td style="background-color:%s;border-radius:8px;padding:14px 32px;">
<a href="%s" style="color:#ffffff;text-decoration:none;font-size:15px;font-weight:600;display:inline-block;">%s</a>
</td></tr>
</table>`, color, href, label)
}

func emailParagraph(text string) string {
	return fmt.Sprintf(`<p style="margin:0 0 16px;color:#334155;font-size:15px;line-height:24px;">%s</p>`, text)
}

func emailMuted(text string) string {
	return fmt.Sprintf(`<p style="margin:16px 0 0;color:#94A3B8;font-size:13px;line-height:20px;">%s</p>`, text)
}

func emailInfoRow(label, value string) string {
	return fmt.Sprintf(`<tr>
<td style="padding:8px 0;color:#64748B;font-size:14px;width:140px;vertical-align:top;">%s</td>
<td style="padding:8px 0;color:#0F172A;font-size:14px;font-weight:500;vertical-align:top;">%s</td>
</tr>`, label, value)
}

func emailInfoCard(rows string) string {
	return fmt.Sprintf(`<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0" style="background-color:#F8FAFC;border:1px solid #E2E8F0;border-radius:8px;margin:20px 0;">
<tr><td style="padding:16px 20px;">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" border="0">
%s
</table>
</td></tr>
</table>`, rows)
}

// ---------------------------------------------------------------------------
// Auth emails
// ---------------------------------------------------------------------------

func SendMagicLink(to, link string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := emailParagraph("We received a sign-in request for your account. Click the button below to log in securely. This link expires in <strong>10 minutes</strong>.") +
		emailButton(link, "Sign in to QCS Cargo", brandBlue) +
		emailMuted("If you didn't request this link, you can safely ignore this email. No action is needed.")

	html := emailLayout("Sign in to your QCS Cargo account", "Sign In", body)
	text := fmt.Sprintf("Sign in to QCS Cargo (valid 10 min): %s", link)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Sign in to QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

func SendVerificationEmail(to, link string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := emailParagraph("Welcome to QCS Cargo! Please verify your email address to activate your account and start receiving packages at your personal US suite.") +
		emailButton(link, "Verify my email", brandBlue) +
		emailMuted("This link expires in 24 hours. If you didn't create this account, you can ignore this email.")

	html := emailLayout("Verify your QCS Cargo email address", "Email Verification", body)
	text := fmt.Sprintf("Verify your email (valid 24 hours): %s", link)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Verify your QCS Cargo email",
		Html:    html,
		Text:    text,
	})
	return err
}

func SendPasswordResetLink(to, link string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := emailParagraph("We received a request to reset your password. Click the button below to choose a new one. This link expires in <strong>1 hour</strong>.") +
		emailButton(link, "Reset my password", brandBlue) +
		emailMuted("If you didn't request a password reset, no changes have been made to your account. You can safely ignore this email.")

	html := emailLayout("Reset your QCS Cargo password", "Password Reset", body)
	text := fmt.Sprintf("Reset your password (valid 1 hour): %s", link)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Reset your QCS Cargo password",
		Html:    html,
		Text:    text,
	})
	return err
}

// ---------------------------------------------------------------------------
// Contact form (internal)
// ---------------------------------------------------------------------------

func SendContactFormSubmission(fromName, fromEmail, subject, body string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	subj := "Contact form: QCS Cargo"
	if subject != "" {
		subj = "Contact form: " + subject
	}
	rows := emailInfoRow("From", escapeHTML(fromName)+" &lt;"+escapeHTML(fromEmail)+"&gt;") +
		emailInfoRow("Subject", escapeHTML(subject))
	content := emailInfoCard(rows) +
		fmt.Sprintf(`<div style="background-color:#F8FAFC;border:1px solid #E2E8F0;border-radius:8px;padding:16px 20px;margin:20px 0;"><pre style="margin:0;white-space:pre-wrap;font-family:monospace;font-size:14px;color:#334155;">%s</pre></div>`, escapeHTML(body))

	html := emailLayout("New contact form submission", "Contact Form", content)
	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{supportEmail},
		ReplyTo: fromEmail,
		Subject: subj,
		Html:    html,
	})
	return err
}

// ---------------------------------------------------------------------------
// Package lifecycle emails
// ---------------------------------------------------------------------------

func SendPackageArrived(to, senderName string, weightLbs float64) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	dashboardLink := appURL() + "/dashboard/inbox"
	rows := emailInfoRow("Sender", escapeHTML(senderName)) +
		emailInfoRow("Weight", fmt.Sprintf("%.1f lbs", weightLbs))
	body := emailParagraph("A new package has arrived at your QCS Cargo suite and is ready for you to manage.") +
		emailInfoCard(rows) +
		emailButton(dashboardLink, "View in My Packages", brandBlue)

	html := emailLayout("New package at your QCS suite from "+escapeHTML(senderName), "Package Arrived", body)
	text := fmt.Sprintf("New package from %s! Weight: %.1f lbs. Log in to view.", senderName, weightLbs)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "New package at QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

func SendPhotoReady(to, senderName string, photoCount int) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	dashboardLink := appURL() + "/dashboard/inbox"
	body := emailParagraph(fmt.Sprintf("<strong>%d photo%s</strong> of your package from <strong>%s</strong> %s now ready to view in your dashboard.",
		photoCount, pluralS(photoCount), escapeHTML(senderName), pluralVerb(photoCount))) +
		emailButton(dashboardLink, "View photos", brandBlue)

	html := emailLayout("Your package photos are ready", "Photos Ready", body)
	text := fmt.Sprintf("%d photos of your package from %s are ready.", photoCount, senderName)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Your package photos are ready",
		Html:    html,
		Text:    text,
	})
	return err
}

func SendServiceComplete(to, serviceName, senderName string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	dashboardLink := appURL() + "/dashboard/inbox"
	rows := emailInfoRow("Service", escapeHTML(serviceName)) +
		emailInfoRow("Package from", escapeHTML(senderName))
	body := emailParagraph("A value-added service you requested has been completed by our warehouse team.") +
		emailInfoCard(rows) +
		emailButton(dashboardLink, "View package", brandBlue)

	html := emailLayout(serviceName+" complete for "+senderName+" package", "Service Complete", body)
	text := fmt.Sprintf("%s for package from %s is complete.", serviceName, senderName)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Service complete – QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

// ---------------------------------------------------------------------------
// Storage warning emails
// ---------------------------------------------------------------------------

func SendStorageWarning5Days(to, senderName string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	dashboardLink := appURL() + "/dashboard/inbox"
	body := emailParagraph(fmt.Sprintf("Your package from <strong>%s</strong> will begin accruing daily storage fees in <strong>5 days</strong>. Ship or pick up your package before then to avoid charges.", escapeHTML(senderName))) +
		emailButton(dashboardLink, "Manage package", brandOrange)

	html := emailLayout("Storage fees starting soon for "+senderName+" package", "Storage Reminder", body)
	text := fmt.Sprintf("Package from %s starts storage fees in 5 days.", senderName)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Storage reminder – QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

func SendStorageWarning1Day(to, senderName string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	dashboardLink := appURL() + "/dashboard/inbox"
	body := emailParagraph(fmt.Sprintf("Today is the <strong>last day of free storage</strong> for your package from <strong>%s</strong>. Daily fees begin tomorrow.", escapeHTML(senderName))) +
		emailButton(dashboardLink, "Ship now", brandOrange)

	html := emailLayout("Last day of free storage for "+senderName+" package", "Storage – Final Day", body)
	text := fmt.Sprintf("Last day of free storage for %s package.", senderName)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Last day of free storage – QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

func SendStorageFeeCharged(to, senderName string, amount float64) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	dashboardLink := appURL() + "/dashboard/invoices"
	rows := emailInfoRow("Package from", escapeHTML(senderName)) +
		emailInfoRow("Fee charged", fmt.Sprintf("$%.2f", amount))
	body := emailParagraph("A daily storage fee has been applied to your account. Ship your package to stop recurring charges.") +
		emailInfoCard(rows) +
		emailButton(dashboardLink, "View invoices", brandBlue)

	html := emailLayout(fmt.Sprintf("$%.2f storage fee charged", amount), "Storage Fee", body)
	text := fmt.Sprintf("Storage fee $%.2f charged for %s package.", amount, senderName)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Storage fee charged – QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

func SendStorageFinalNotice(to, senderName string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	dashboardLink := appURL() + "/dashboard/inbox"
	body := emailParagraph(fmt.Sprintf("Your package from <strong>%s</strong> will be <strong>disposed of in 5 days</strong> unless you arrange for shipping. Please take action now to avoid losing your package.", escapeHTML(senderName))) +
		emailButton(dashboardLink, "Ship my package", "#DC2626")

	html := emailLayout("URGENT: Package disposal in 5 days", "Final Notice", body)
	text := fmt.Sprintf("Package from %s will be disposed in 5 days unless shipped.", senderName)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Final notice – package disposal – QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

// ---------------------------------------------------------------------------
// Shipping lifecycle emails
// ---------------------------------------------------------------------------

func SendShipRequestPaid(to, code string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	dashboardLink := appURL() + "/dashboard/ship-requests"
	body := emailParagraph(fmt.Sprintf("Payment received! Your ship request <strong>%s</strong> is confirmed and our warehouse team is preparing your package.", escapeHTML(code))) +
		emailButton(dashboardLink, "Track ship request", brandBlue)

	html := emailLayout("Ship request "+code+" confirmed", "Ship Request Confirmed", body)
	text := fmt.Sprintf("Ship request %s confirmed! Being prepared.", code)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Ship request confirmed – QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

func SendShipRequestShipped(to, code, tracking string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	dashboardLink := appURL() + "/dashboard/shipments"
	rows := emailInfoRow("Ship request", escapeHTML(code)) +
		emailInfoRow("Tracking", escapeHTML(tracking))
	body := emailParagraph("Your package is on its way! You can track its progress using the details below.") +
		emailInfoCard(rows) +
		emailButton(dashboardLink, "Track shipment", brandBlue)

	html := emailLayout("Shipment "+code+" is on its way", "Shipped", body)
	text := fmt.Sprintf("Shipment %s on its way! Tracking: %s", code, tracking)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Shipment on its way – QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

func SendShipRequestDelivered(to, code, destination string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	rows := emailInfoRow("Ship request", escapeHTML(code)) +
		emailInfoRow("Delivered to", escapeHTML(destination))
	body := emailParagraph("Great news — your shipment has been delivered!") +
		emailInfoCard(rows)

	html := emailLayout("Shipment "+code+" delivered", "Delivered", body)
	text := fmt.Sprintf("Shipment %s delivered in %s!", code, destination)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Shipment delivered – QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

func SendInboundDelivered(to, retailerName string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	dashboardLink := appURL() + "/dashboard/inbound"
	body := emailParagraph(fmt.Sprintf("Your package from <strong>%s</strong> has been delivered to QCS Cargo. It will appear in your locker once our team processes it.", escapeHTML(retailerName))) +
		emailButton(dashboardLink, "View expected packages", brandBlue)

	html := emailLayout(retailerName+" package delivered to QCS", "Inbound Delivered", body)
	text := fmt.Sprintf("%s package delivered to QCS. Check locker shortly.", retailerName)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Package arrived at QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

func SendCustomsHold(to, code string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	dashboardLink := appURL() + "/dashboard/customs"
	body := emailParagraph(fmt.Sprintf("Ship request <strong>%s</strong> requires customs documentation before it can proceed. Please review and provide the necessary information.", escapeHTML(code))) +
		emailButton(dashboardLink, "Resolve customs issue", brandOrange)

	html := emailLayout("Customs attention needed for "+code, "Customs Notice", body)
	text := fmt.Sprintf("Ship request %s needs customs attention.", code)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Customs notice – QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

func SendBookingConfirmed(to, code, date string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	dashboardLink := appURL() + "/dashboard/bookings"
	rows := emailInfoRow("Booking", escapeHTML(code)) +
		emailInfoRow("Date", escapeHTML(date))
	body := emailParagraph("Your booking has been confirmed. Here are the details:") +
		emailInfoCard(rows) +
		emailButton(dashboardLink, "View booking", brandBlue)

	html := emailLayout("Booking "+code+" confirmed", "Booking Confirmed", body)
	text := fmt.Sprintf("Booking %s confirmed for %s.", code, date)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Booking confirmed – QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

func SendShipmentStatus(to, trackingNumber, status string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	dashboardLink := appURL() + "/dashboard/shipments"
	rows := emailInfoRow("Tracking", escapeHTML(trackingNumber)) +
		emailInfoRow("Status", escapeHTML(status))
	body := emailParagraph("There's an update on your shipment:") +
		emailInfoCard(rows) +
		emailButton(dashboardLink, "View shipment", brandBlue)

	html := emailLayout("Shipment "+trackingNumber+" update: "+status, "Shipment Update", body)
	text := fmt.Sprintf("Shipment %s: now %s", trackingNumber, status)

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Shipment update – QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

// SendSecurityAlert sends a generic security notification (e.g. MFA disabled,
// password changed, new sign-in). Pass 2 audit fix H-2 / defense-in-depth.
func SendSecurityAlert(to, subject, message string) error {
	client := resendClient()
	if client == nil {
		return nil
	}
	body := emailParagraph(escapeHTML(message)) +
		emailParagraph("If this was not you, please reset your password and contact "+supportEmail+".") +
		emailButton(appURL()+"/dashboard/settings/security", "Review security", brandBlue)
	html := emailLayout("Security notice: "+subject, "Security Notice", body)
	text := subject + "\n\n" + message + "\n\nIf this was not you, contact " + supportEmail + "."

	_, err := client.Emails.Send(&resend.SendEmailRequest{
		From:    "QCS Cargo <" + fromAddress() + ">",
		To:      []string{to},
		Subject: "Security notice – QCS Cargo",
		Html:    html,
		Text:    text,
	})
	return err
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func pluralVerb(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
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
