package auth

import (
	"fmt"
	"strings"

	"2pick.app/backend/internal/mailer"
)

// The reset mail's text, in the three languages the site is published in.
//
// THIS IS THE ONLY HUMAN-LANGUAGE TEXT IN THE GO API, and it is here rather than in the
// SPA because a mail is read outside the browser: nothing on the receiving end can look
// up a message code. Everywhere else — validation, errors — this API answers with codes
// and the SPA translates them, and three strings in one file are not a reason to change
// that.
//
// The wording is Laravel's resources/views/auth/passwords mail in substance: what the
// link is for, how long it lasts, and that ignoring it is safe.
type passwordResetText struct {
	// prefix is the locale's URL segment, which must match localeDefinitions in
	// frontend/src/i18n.ts (zh-tw / en / ja) or the link 404s.
	prefix  string
	subject string
	// body takes the link.
	body func(link string) string
}

var passwordResetTexts = map[string]passwordResetText{
	"zh_TW": {
		prefix:  "zh-tw",
		subject: "重設你的密碼",
		body: func(link string) string {
			return strings.Join([]string{
				"你要求重設殘酷二選一的帳號密碼。",
				"",
				"請開啟以下連結設定新密碼：",
				link,
				"",
				"這個連結一小時內有效，且只能使用一次。",
				"如果不是你本人要求的，請忽略這封信，你的密碼不會有任何改變。",
			}, "\r\n")
		},
	},
	"en": {
		prefix:  "en",
		subject: "Reset your password",
		body: func(link string) string {
			return strings.Join([]string{
				"You asked to reset the password for your 2pick account.",
				"",
				"Open this link to choose a new one:",
				link,
				"",
				"The link works for one hour, and only once.",
				"If you did not ask for this, ignore this mail; your password stays as it is.",
			}, "\r\n")
		},
	},
	"ja": {
		prefix:  "ja",
		subject: "パスワードの再設定",
		body: func(link string) string {
			return strings.Join([]string{
				"アカウントのパスワード再設定がリクエストされました。",
				"",
				"次のリンクから新しいパスワードを設定してください：",
				link,
				"",
				"このリンクは1時間のみ有効で、使用できるのは1回だけです。",
				"心当たりがない場合はこのメールを無視してください。パスワードは変更されません。",
			}, "\r\n")
		},
	},
}

// defaultResetLocale is the site's own language, used for a locale the caller did not
// send or this file does not know.
const defaultResetLocale = "zh_TW"

func passwordResetMessage(locale, email, appURL, token string) mailer.Message {
	text, ok := passwordResetTexts[locale]
	if !ok {
		text = passwordResetTexts[defaultResetLocale]
	}
	link := fmt.Sprintf("%s/%s/password/reset/%s", strings.TrimRight(appURL, "/"), text.prefix, token)
	return mailer.Message{To: email, Subject: text.subject, TextBody: text.body(link)}
}
