package main

import (
	"strings"
	"testing"
)

func TestDetectSightLanguage(t *testing.T) {
	for _, test := range []struct{ language, text string }{
		{"ja", "この文章は日本語で書かれています。"},
		{"ko", "이 문장은 한국어로 작성되었습니다."},
		{"zh_CN", "我们会在学校门口见面，这里的书很有意思。"},
		{"zh_TW", "我們會在學校門口見面，這裡的書很有意思。"},
		{"ru", "Этот текст написан на русском языке."},
		{"uk", "Цей текст українською мовою пояснює, як працює система."},
		{"ar", "هذا النص مكتوب باللغة العربية."},
		{"fa", "این نوشته به زبان فارسی است و چگونگی کار برنامه را توضیح می‌دهد."},
		{"he", "הטקסט הזה כתוב בשפה העברית."},
		{"el", "Αυτό το κείμενο είναι γραμμένο στα ελληνικά."},
		{"th", "ข้อความนี้เขียนเป็นภาษาไทยและอ่านได้ง่าย"},
		{"hi", "यह पाठ हिंदी में लिखा गया है।"},
		{"bn", "এই লেখাটি বাংলা ভাষায় লেখা হয়েছে।"},
		{"ta", "இந்த உரை தமிழ் மொழியில் எழுதப்பட்டுள்ளது."},
		{"te", "ఈ వచనం తెలుగు భాషలో వ్రాయబడింది."},
		{"ml", "ഈ വാചകം മലയാളത്തിൽ എഴുതിയതാണ്."},
		{"en", "The window is open and you can read this text."},
		{"es", "El niño está en la escuela con una amiga y los profesores."},
		{"fr", "Les enfants sont dans le jardin avec une amie et des livres."},
		{"de", "Die Bücher sind für eine Freundin und nicht für den Lehrer."},
		{"it", "Il libro è per gli studenti che non sono con una guida."},
		{"pt", "Os alunos não estão com uma professora e o livro é da escola."},
		{"nl", "Het boek is voor een vriendin en dit zijn ook haar bloemen."},
		{"sv", "Det är ett brev för dig och inte för den som står på gatan."},
		{"nb", "Det er et brev til deg og ikke til den som står på gaten."},
		{"fi", "Tämä on kirja ja se ei ole uusi mutta myös tämä toimii."},
		{"pl", "To jest książka dla uczniów i nie ma jej w domu."},
		{"cs", "To je kniha pro děti a není jako ty, které jsou na stole."},
		{"hu", "Ez egy könyv, és nem az, amit a házban láttam."},
		{"ro", "Acesta este un text în care sunt și informații pentru o prietenă."},
		{"tr", "Bu bir kitap ve daha çok bilgi için bu sayfada var."},
		{"id", "Ini adalah buku yang ada di dalam rumah dan bukan untuk dijual."},
		{"vi", "Đây là một cuốn sách của tôi và không được để trong phòng này."},
	} {
		t.Run(test.language, func(t *testing.T) {
			if got := detectSightLanguage(test.text); got != test.language {
				t.Fatalf("detectSightLanguage(%q) = %q, want %q", test.text, got, test.language)
			}
		})
	}
}

func TestDetectSightLanguageUnknown(t *testing.T) {
	for _, text := range []string{"", " \n\t", "123!", "I", "an", "你好", "яї", "ñß", "the lantern", "ñññ", "Hello world", "de la", "con la", "bonjour hello"} {
		t.Run(text, func(t *testing.T) {
			if got := detectSightLanguage(text); got != "" {
				t.Fatalf("detectSightLanguage(%q) = %q, want unknown", text, got)
			}
		})
	}
}

func TestDetectSightLanguageChinesePairs(t *testing.T) {
	simplified := []rune("这们说时国会来对为发过还没见话经问开关机东车书学门")
	traditional := []rune("這們說時國會來對為發過還沒見話經問開關機東車書學門")
	for i, r := range simplified {
		for _, test := range []struct{ text, want string }{
			{strings.Repeat(string(r), 3), "zh_CN"},
			{strings.Repeat(string(traditional[i]), 3), "zh_TW"},
		} {
			if got := detectSightLanguage(test.text); got != test.want {
				t.Errorf("detectSightLanguage(%q) = %q, want %q", test.text, got, test.want)
			}
		}
	}
	for _, text := range []string{"这這人", "山水人"} {
		if got := detectSightLanguage(text); got != "zh_CN" {
			t.Errorf("Chinese tie %q = %q, want zh_CN", text, got)
		}
	}
}

func TestDetectSightLanguageCaseAndPunctuation(t *testing.T) {
	for _, test := range []struct{ text, want string }{
		{"THE,WINDOW!IS\nOPEN;AND?YOU CAN READ THIS.", "en"},
		{"ЦЕ УКРАЇНСЬКИЙ ТЕКСТ ПРО ПОДОРОЖ.", "uk"},
		{"ПРИВЕТ МИР", "ru"},
		{"É um livro para você e não para mim.", "pt"},
		{"Это 漢字の説明です", "ja"},
		{"漢字 한국어 설명입니다", "ko"},
	} {
		if got := detectSightLanguage(test.text); got != test.want {
			t.Errorf("detectSightLanguage(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}

// A highlighted chat line is short; common function words must carry it.
func TestDetectSightLanguageShortPhrases(t *testing.T) {
	for _, test := range []struct{ text, want string }{
		{"Give birth to it is insane", "en"},
		{"I think it is fine", "en"},
		{"Jeg har ikke sett det", "nb"},
		{"Jag har inte sett det", "sv"},
		{"Ich habe das nicht gesehen", "de"},
		{"Guten Morgen, wie geht es dir heute", "de"},
		{"Je ne sais pas ce que tu veux", "fr"},
		{"Yo no sé qué quieres decir", "es"},
		{"Eu não sei o que você quer", "pt"},
		{"Io non so cosa vuoi dire", "it"},
	} {
		if got := detectSightLanguage(test.text); got != test.want {
			t.Errorf("detectSightLanguage(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}

func TestDetectSightLanguageDiacriticHints(t *testing.T) {
	for _, test := range []struct{ text, want string }{
		{"La niña de la casa.", "es"},
		{"de que ã", "pt"},
		{"na do ą", "pl"},
		{"na do ř", "cs"},
		{"a de ő", "hu"},
		{"la de ă", "ro"},
		{"det som å", ""},
		{"det som ä", "sv"},
	} {
		if got := detectSightLanguage(test.text); got != test.want {
			t.Errorf("detectSightLanguage(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}
