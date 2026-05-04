package mail

import (
	"fmt"

	"github.com/resend/resend-go/v2"
)

// ResendMailer はResend経由のメール送信実装
type ResendMailer struct {
	APIKey    string
	FromEmail string
	FromName  string
}

func (m *ResendMailer) Send(to, subject, htmlBody string) error {
	client := resend.NewClient(m.APIKey)

	from := m.FromEmail
	if m.FromName != "" {
		from = fmt.Sprintf("%s <%s>", m.FromName, m.FromEmail)
	}

	params := &resend.SendEmailRequest{
		From:    from,
		To:      []string{to},
		Subject: subject,
		Html:    htmlBody,
	}

	_, err := client.Emails.Send(params)
	return err
}

// SendBulk は複数の宛先に同じメールを送信する
func (m *ResendMailer) SendBulk(emails []string, subject, htmlBody string) []error {
	var errs []error
	for _, email := range emails {
		if err := m.Send(email, subject, htmlBody); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", email, err))
		}
	}
	return errs
}

// --- メールテンプレート ---

// RegistrationEmail は登録完了メールのHTML本文を生成する
func RegistrationEmail(handle, mypageURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja">
<head><meta charset="UTF-8"></head>
<body style="font-family: 'Helvetica Neue', Arial, sans-serif; background: #0f0f0f; color: #e0e0e0; padding: 32px;">
<div style="max-width: 560px; margin: 0 auto; background: #1a1a1a; border: 1px solid #2e2e2e; border-radius: 6px; padding: 32px;">
  <h1 style="color: #c0392b; font-size: 20px; margin-bottom: 24px;">ALT-FETISH 交流セッション</h1>
  <p>%s さん、ご登録ありがとうございます。</p>
  <p style="margin-top: 16px;">マイページのURLをお送りします。このURLは他人と共有しないでください。</p>
  <div style="margin: 24px 0; padding: 16px; background: #222; border-radius: 6px; border: 1px solid #2e2e2e;">
    <a href="%s" style="color: #c0392b; word-break: break-all;">%s</a>
  </div>
  <p style="color: #888; font-size: 14px;">このメールに心当たりがない場合は無視してください。</p>
  <hr style="border-color: #2e2e2e; margin: 24px 0;">
  <p style="color: #888; font-size: 12px;">市川（ALT-FETISH）</p>
</div>
</body>
</html>`, handle, mypageURL, mypageURL)
}

// NewEventEmail は新規開催日のお知らせメールHTML本文を生成する
func NewEventEmail(eventDate, eventTime string, capacity int, eventsURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja">
<head><meta charset="UTF-8"></head>
<body style="font-family: 'Helvetica Neue', Arial, sans-serif; background: #0f0f0f; color: #e0e0e0; padding: 32px;">
<div style="max-width: 560px; margin: 0 auto; background: #1a1a1a; border: 1px solid #2e2e2e; border-radius: 6px; padding: 32px;">
  <h1 style="color: #c0392b; font-size: 20px; margin-bottom: 24px;">新しい開催日のお知らせ</h1>
  <p>新しい交流セッションの開催日が登録されました。</p>
  <div style="margin: 24px 0; padding: 16px; background: #222; border-radius: 6px; border: 1px solid #2e2e2e;">
    <p><strong>日時：</strong>%s %s〜</p>
    <p><strong>定員：</strong>%d名</p>
  </div>
  <p>参加を希望される方は、マイページからご表明ください。</p>
  <div style="margin: 24px 0;">
    <a href="%s" style="display: inline-block; background: #c0392b; color: #fff; padding: 12px 24px; border-radius: 6px; text-decoration: none;">開催日一覧を見る</a>
  </div>
  <hr style="border-color: #2e2e2e; margin: 24px 0;">
  <p style="color: #888; font-size: 12px;">市川（ALT-FETISH）</p>
</div>
</body>
</html>`, eventDate, eventTime, capacity, eventsURL)
}

// ConfirmationEmail は参加確定通知メールHTML本文を生成する
func ConfirmationEmail(handle, eventDate, eventTime string, capacity int) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja">
<head><meta charset="UTF-8"></head>
<body style="font-family: 'Helvetica Neue', Arial, sans-serif; background: #0f0f0f; color: #e0e0e0; padding: 32px;">
<div style="max-width: 560px; margin: 0 auto; background: #1a1a1a; border: 1px solid #2e2e2e; border-radius: 6px; padding: 32px;">
  <h1 style="color: #c0392b; font-size: 20px; margin-bottom: 24px;">参加確定のお知らせ</h1>
  <p>%s さん、参加が確定しました。</p>
  <div style="margin: 24px 0; padding: 16px; background: #222; border-radius: 6px; border: 1px solid #2e2e2e;">
    <p><strong>日時：</strong>%s %s〜</p>
    <p><strong>定員：</strong>%d名</p>
  </div>
  <p>詳細は管理者よりご連絡します。当日はお気をつけてお越しください。</p>
  <hr style="border-color: #2e2e2e; margin: 24px 0;">
  <p style="color: #888; font-size: 12px;">市川（ALT-FETISH）</p>
</div>
</body>
</html>`, handle, eventDate, eventTime, capacity)
}

// DeclineEmail は見送り通知メールHTML本文を生成する
func DeclineEmail(handle, eventDate string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja">
<head><meta charset="UTF-8"></head>
<body style="font-family: 'Helvetica Neue', Arial, sans-serif; background: #0f0f0f; color: #e0e0e0; padding: 32px;">
<div style="max-width: 560px; margin: 0 auto; background: #1a1a1a; border: 1px solid #2e2e2e; border-radius: 6px; padding: 32px;">
  <h1 style="color: #c0392b; font-size: 20px; margin-bottom: 24px;">今回の参加について</h1>
  <p>%s さん、%s のセッションへのご応募ありがとうございました。</p>
  <p style="margin-top: 16px;">今回は定員の都合により、ご参加いただくことができませんでした。またの機会にぜひお申し込みください。</p>
  <hr style="border-color: #2e2e2e; margin: 24px 0;">
  <p style="color: #888; font-size: 12px;">市川（ALT-FETISH）</p>
</div>
</body>
</html>`, handle, eventDate)
}

// ReminderEmail は前日リマインダーメールHTML本文を生成する
func ReminderEmail(handle, eventDate, eventTime, mypageURL string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja">
<head><meta charset="UTF-8"></head>
<body style="font-family: 'Helvetica Neue', Arial, sans-serif; background: #0f0f0f; color: #e0e0e0; padding: 32px;">
<div style="max-width: 560px; margin: 0 auto; background: #1a1a1a; border: 1px solid #2e2e2e; border-radius: 6px; padding: 32px;">
  <h1 style="color: #c0392b; font-size: 20px; margin-bottom: 24px;">明日のセッションのご確認</h1>
  <p>%s さん、明日のセッションのリマインダーです。</p>
  <div style="margin: 24px 0; padding: 16px; background: #222; border-radius: 6px; border: 1px solid #2e2e2e;">
    <p><strong>日時：</strong>%s %s〜</p>
  </div>
  <p>ご不明な点はマイページのチャットよりお知らせください。</p>
  <div style="margin: 24px 0;">
    <a href="%s" style="display: inline-block; background: #c0392b; color: #fff; padding: 12px 24px; border-radius: 6px; text-decoration: none;">マイページを開く</a>
  </div>
  <hr style="border-color: #2e2e2e; margin: 24px 0;">
  <p style="color: #888; font-size: 12px;">市川（ALT-FETISH）</p>
</div>
</body>
</html>`, handle, eventDate, eventTime, mypageURL)
}

// CancelEmail はキャンセル通知メールHTML本文を生成する
func CancelEmail(handle, eventDate string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ja">
<head><meta charset="UTF-8"></head>
<body style="font-family: 'Helvetica Neue', Arial, sans-serif; background: #0f0f0f; color: #e0e0e0; padding: 32px;">
<div style="max-width: 560px; margin: 0 auto; background: #1a1a1a; border: 1px solid #2e2e2e; border-radius: 6px; padding: 32px;">
  <h1 style="color: #c0392b; font-size: 20px; margin-bottom: 24px;">セッションキャンセルのお知らせ</h1>
  <p>%s さん、誠に申し訳ありませんが、%s に予定しておりましたセッションをキャンセルさせていただきます。</p>
  <p style="margin-top: 16px;">またの機会にお申し込みいただければ幸いです。</p>
  <hr style="border-color: #2e2e2e; margin: 24px 0;">
  <p style="color: #888; font-size: 12px;">市川（ALT-FETISH）</p>
</div>
</body>
</html>`, handle, eventDate)
}
