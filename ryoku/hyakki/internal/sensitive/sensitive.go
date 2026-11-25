// Package sensitive is Hyakki's one judge of private text. It knows the
// words that name a private field (a password, a PIN, a card or identity
// number, a one-time code, a recovery phrase, a key), the password managers
// and other places that hold secrets, the shapes of keys, tokens and
// one-time codes, and the card, social security, national identity and bank
// account numbers memory never keeps. Dictation, Hands' rules, the screen
// asks, the quick-command notes and memory all ask it, so there is one list
// to keep and one rule per shape. It needs only the standard library.
//
// Every check is a code rule, never a model's guess, and none keeps or logs
// the text it reads.
package sensitive

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// What a private-field word names, for refusal lines.
const (
	whatPassword   = "a password"
	whatPasscode   = "a passcode"
	whatPassphrase = "a passphrase"
	whatPasskey    = "a passkey"
	whatPIN        = "a PIN"
	whatCVV        = "a card security code"
	whatCard       = "a card number"
	whatExpiry     = "a card expiry date"
	whatSSN        = "a social security number"
	whatNI         = "a national insurance number"
	whatID         = "an identity number"
	whatPassport   = "a passport number"
	whatLicence    = "a licence number"
	whatTax        = "a tax number"
	whatBank       = "a bank account number"
	whatRouting    = "a bank routing number"
	whatSortCode   = "a bank sort code"
	whatOTP        = "a one-time code"
	whatRecovery   = "a recovery phrase"
	whatRecoveryCd = "a recovery code"
	whatKey        = "a secret key"
	whatSecret     = "a secret"
)

// singles are words that name a private field on their own. A trailing s
// also matches ("PINs", "passports").
var singles = map[string]string{
	"pin": whatPIN, "passkey": whatPasskey,
	"cvv": whatCVV, "cvv2": whatCVV, "cvc": whatCVV, "cvn": whatCVV,
	"ssn": whatSSN, "passport": whatPassport,
	"otp": whatOTP, "totp": whatOTP, "2fa": whatOTP, "mfa": whatOTP,
	"iban": whatBank, "mnemonic": whatRecovery, "secret": whatSecret,
	"pwd": whatPassword, "passwd": whatPassword, "passwort": whatPassword, "contraseña": whatPassword,
	"contrasena": whatPassword, "senha": whatPassword, "пароль": whatPassword, "wachtwoord": whatPassword,
	"lösenord": whatPassword, "hasło": whatPassword,
}

// stems name a private field anywhere inside a word ("newpassword").
var stems = []struct{ stem, what string }{
	{"password", whatPassword}, {"passcode", whatPasscode}, {"passphrase", whatPassphrase},
	{"パスワード", whatPassword}, {"密码", whatPassword}, {"密碼", whatPassword}, {"비밀번호", whatPassword},
}

// phrases are word sequences that name a private field. The last word may
// take a trailing s, and the words written together as one word also match
// ("pincode", "creditcards").
var phrases = []struct{ phrase, what string }{
	{"mot de passe", whatPassword},
	{"pin code", whatPIN}, {"pin number", whatPIN},
	{"security code", whatCVV}, {"card verification", whatCVV},
	{"card number", whatCard}, {"credit card", whatCard}, {"debit card", whatCard}, {"bank card", whatCard},
	{"card no", whatCard}, {"expiry date", whatExpiry}, {"expiration date", whatExpiry},
	{"account number", whatBank}, {"account no", whatBank}, {"bank account", whatBank},
	{"routing number", whatRouting}, {"sort code", whatSortCode},
	{"social security", whatSSN}, {"national insurance", whatNI},
	{"driver license", whatLicence}, {"drivers license", whatLicence}, {"driver s license", whatLicence},
	{"driver licence", whatLicence}, {"drivers licence", whatLicence}, {"driver s licence", whatLicence},
	{"driving licence", whatLicence}, {"driving license", whatLicence},
	{"id number", whatID}, {"identity number", whatID}, {"national id", whatID},
	{"passport number", whatPassport},
	{"tax id", whatTax}, {"tax number", whatTax},
	{"license number", whatLicence}, {"licence number", whatLicence},
	{"one time code", whatOTP}, {"verification code", whatOTP}, {"authentication code", whatOTP},
	{"auth code", whatOTP}, {"login code", whatOTP}, {"two factor", whatOTP},
	{"recovery phrase", whatRecovery}, {"seed phrase", whatRecovery},
	{"recovery code", whatRecoveryCd}, {"backup code", whatRecoveryCd},
	{"secret key", whatKey}, {"private key", whatKey}, {"api key", whatKey}, {"access token", whatKey},
	{"bearer token", whatKey},
}

// joined maps each phrase written as one word to what it names.
var joined = func() map[string]string {
	out := make(map[string]string, len(phrases))
	for _, p := range phrases {
		out[strings.ReplaceAll(p.phrase, " ", "")] = p.what
	}
	return out
}()

// places are app ids and window titles that hold secrets wherever they
// appear inside a longer name, such as "org.keepassxc.KeePassXC".
var places = []string{
	"keepass", "bitwarden", "vaultwarden", "1password", "lastpass", "dashlane", "kwallet", "seahorse",
	"keyring", "pinentry",
}

// words lowercases s and splits it into its letter and digit runs.
func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

// single names what one word names, a trailing s allowed.
func single(w string) string {
	if what, ok := singles[w]; ok {
		return what
	}
	if what, ok := joined[w]; ok {
		return what
	}
	if base, ok := strings.CutSuffix(w, "s"); ok {
		if what, ok := singles[base]; ok {
			return what
		}
		if what, ok := joined[base]; ok {
			return what
		}
	}
	return ""
}

// Label names the private field s mentions, in a few words such as "a
// password" or "a card number", or "" when it names none. It reads words,
// not values: "Card number" is private, "Add card" is not.
func Label(s string) string {
	label := ""
	labels(s, func(what string) bool {
		label = what
		return false
	})
	return label
}

// labels calls each with what every private-field word in s names, in
// order, until each returns false.
func labels(s string, each func(what string) bool) {
	ws := words(s)
	for _, w := range ws {
		if what := single(w); what != "" && !each(what) {
			return
		}
		for _, p := range stems {
			if strings.Contains(w, p.stem) && !each(p.what) {
				return
			}
		}
	}
	line := " " + strings.Join(ws, " ") + " "
	for _, p := range phrases {
		if (strings.Contains(line, " "+p.phrase+" ") || strings.Contains(line, " "+p.phrase+"s ")) && !each(p.what) {
			return
		}
	}
}

// Place says whether s, an app id, a window title or a caption, names a
// secret or a place that holds one (a private-field word or a password
// manager), or shows a card, identity or bank account number.
func Place(s string) bool {
	if strings.TrimSpace(s) == "" {
		return false
	}
	if Label(s) != "" {
		return true
	}
	low := strings.ToLower(s)
	for _, name := range places {
		if strings.Contains(low, name) {
			return true
		}
	}
	return Number(s) != None
}

var (
	// otpShape is a whole text of six to eight digits, one space or dash
	// allowed between them: the shape of a one-time code.
	otpShape = regexp.MustCompile(`^\d{3,4}[ \-]?\d{3,4}$`)
	// tokenPrefixes open keys and tokens people paste from a provider.
	tokenPrefixes = []string{"sk-", "sk_", "pk_live_", "rk_live_", "ghp_", "gho_", "ghu_", "ghs_", "github_pat_",
		"glpat-", "xoxb-", "xoxp-", "xoxa-", "akia", "asia", "aiza", "hf_", "ya29."}
	// digitRun is six or more digits, single spaces or hyphens allowed
	// between them.
	digitRun = regexp.MustCompile(`\d(?:[\s-]?\d){5,}`)
)

// Secret says whether s is shaped like a secret: it is a Place, holds a key
// block, a token from a provider or a long run mixing upper case, lower
// case and digits, or it is a one-time code on its own.
func Secret(s string) bool {
	if Place(s) {
		return true
	}
	if strings.Contains(s, "-----BEGIN") || strings.Contains(strings.ToUpper(s), "PRIVATE KEY") {
		return true
	}
	t := strings.TrimSpace(s)
	if l := utf8.RuneCountInString(t); l >= 6 && l <= 9 && otpShape.MatchString(t) {
		return true
	}
	for _, f := range strings.Fields(s) {
		if token(f) {
			return true
		}
	}
	return false
}

// Note says whether words a person speaks to keep must not be kept: they
// are a Secret or hold a run of six or more digits. Speech recognition
// splits and joins spoken digits unpredictably, so no checksum is trusted
// there and any long run is refused.
func Note(s string) bool {
	return Secret(s) || digitRun.MatchString(s)
}

// token says whether one word reads as a key or token.
func token(w string) bool {
	low := strings.ToLower(w)
	for _, p := range tokenPrefixes {
		if strings.HasPrefix(low, p) && len(w) >= len(p)+12 {
			return true
		}
	}
	if len(w) < 20 {
		return false
	}
	var upper, lower, digit bool
	for _, r := range w {
		switch {
		case r >= 'A' && r <= 'Z':
			upper = true
		case r >= 'a' && r <= 'z':
			lower = true
		case r >= '0' && r <= '9':
			digit = true
		case strings.ContainsRune("_-+/=.", r):
		default:
			return false
		}
	}
	return upper && lower && digit
}
