package main

import (
	"strings"
	"unicode"
)

const (
	sightEnglish = iota
	sightSpanish
	sightFrench
	sightGerman
	sightItalian
	sightPortuguese
	sightDutch
	sightSwedish
	sightNorwegian
	sightFinnish
	sightPolish
	sightCzech
	sightHungarian
	sightRomanian
	sightTurkish
	sightIndonesian
	sightVietnamese
	sightLatinLanguages
)

var sightStopwords = [...]struct{ code, words string }{
	{"en", "the and is are was were this that with from for not you your of to it i he she we they be have has had will would can could but or if so what all there do does did just my me his her our their an at by as"},
	{"es", "el la los las un una de del que y en es por para con lo le su al como más pero sus está este esta muy también yo tu te me mi qué hay ser ha"},
	{"fr", "le la les un une des de du et est sont dans pour avec pas que qui ce cette il elle on ne au aux sur par plus mais je tu nous vous ils ça bien"},
	{"de", "der die das ein eine und ist sind nicht mit für von zu auf den dem des sich es ich sie wir auch als aber wie du dir mich mir ihr hat haben wird noch nur so"},
	{"it", "il lo la gli le un una di del e è che non per con della dei nel si sono come anche ma più questo io tu mi ti ci ha ho"},
	{"pt", "o a os as um uma de do da e é que não para com dos das no na em se por mais mas como foi ser eu você ele ela isso"},
	{"nl", "de het een en is zijn niet met voor van op dat dit als ook te ik je we er maar bij aan wat heb hij zij"},
	{"sv", "och att det som en ett är på för med inte av den till har jag du vi men om kan så vad här hur"},
	{"nb", "og at det som en et er på for med ikke av den til har jeg du vi men om kan så hva her hvordan"},
	{"fi", "ja on ei se että tämä ovat kun kanssa mutta tai myös joka niin ole olen hän he me te mikä minä sinä"},
	{"pl", "i w na z do jest nie to się że jak dla oraz ten ale co być są tak już po od przez"},
	{"cs", "a v na je se že to s pro do z jsou není jako ale co být jsem jsi už po od tak"},
	{"hu", "a az és hogy nem van egy ez is meg de el ha mint vagy én te ő mi ti ők nagyon csak még"},
	{"ro", "și în este sunt un o de la cu pentru nu pe din că care eu tu el ea noi voi mai foarte dar"},
	{"tr", "ve bir bu için ile de da değil olan olarak çok ama daha var ne ben sen biz siz o mi mı ki gibi"},
	{"id", "dan yang di ke dari ini itu untuk dengan tidak adalah pada dalam juga akan saya kamu kami kita mereka bisa ada"},
	{"vi", "và là của có trong một những không được cho với này các đã để tôi bạn anh chị em rất nhưng"},
}

// Shared words vote for every matching language, without allocating per request.
var sightStopwordVotes = func() map[string]uint32 {
	votes := make(map[string]uint32, sightLatinLanguages*15)
	for language, entry := range sightStopwords {
		for _, word := range strings.Fields(entry.words) {
			votes[word] |= 1 << language
		}
	}
	return votes
}()

func detectSightLanguage(text string) string {
	// Kana and Hangul take precedence; the other scripts use their rune counts.
	codes := [...]string{"ja", "ko", "zh_CN", "ru", "ar", "he", "el", "th", "hi", "bn", "ta", "te", "ml", ""}
	var counts [len(codes)]int
	var hints [sightLatinLanguages]int
	letters, chinese, ukrainian, persian := 0, 0, false, false
	for _, r := range text {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		counts[sightScript(r)]++
		r = unicode.ToLower(r)
		switch r {
		case '这', '们', '说', '时', '国', '会', '来', '对', '为', '发', '过', '还', '没', '见', '话', '经', '问', '开', '关', '机', '东', '车', '书', '学', '门':
			chinese--
		case '這', '們', '說', '時', '國', '會', '來', '對', '為', '發', '過', '還', '沒', '見', '話', '經', '問', '開', '關', '機', '東', '車', '書', '學', '門':
			chinese++
		case 'і', 'ї', 'є', 'ґ':
			ukrainian = true
		case 'پ', 'چ', 'ژ', 'گ':
			persian = true
		default:
			sightLatinHint(r, &hints)
		}
	}
	if letters < 3 {
		return ""
	}
	if counts[0] > 0 {
		return "ja"
	}
	if counts[1] > 0 {
		return "ko"
	}
	best, tied := 0, false
	for script, count := range counts {
		if count > counts[best] {
			best, tied = script, false
		} else if count > 0 && script != best && count == counts[best] {
			tied = true
		}
	}
	if tied {
		return ""
	}
	switch codes[best] {
	case "zh_CN":
		if chinese > 0 {
			return "zh_TW"
		}
	case "ru":
		if ukrainian {
			return "uk"
		}
	case "ar":
		if persian {
			return "fa"
		}
	case "":
		return sightLatinLanguage(text, hints)
	}
	return codes[best]
}

// The last bucket includes Latin and unsupported scripts, which stay unknown
// unless there is enough stopword evidence.
func sightScript(r rune) int {
	switch {
	case unicode.In(r, unicode.Hiragana, unicode.Katakana):
		return 0
	case unicode.Is(unicode.Hangul, r):
		return 1
	case unicode.Is(unicode.Han, r):
		return 2
	case unicode.Is(unicode.Cyrillic, r):
		return 3
	case unicode.Is(unicode.Arabic, r):
		return 4
	case unicode.Is(unicode.Hebrew, r):
		return 5
	case unicode.Is(unicode.Greek, r):
		return 6
	case unicode.Is(unicode.Thai, r):
		return 7
	case unicode.Is(unicode.Devanagari, r):
		return 8
	case unicode.Is(unicode.Bengali, r):
		return 9
	case unicode.Is(unicode.Tamil, r):
		return 10
	case unicode.Is(unicode.Telugu, r):
		return 11
	case unicode.Is(unicode.Malayalam, r):
		return 12
	default:
		return 13
	}
}

func sightLatinHint(r rune, hints *[sightLatinLanguages]int) {
	switch r {
	case 'ñ':
		hints[sightSpanish]++
	case 'ß':
		hints[sightGerman]++
	case 'ã', 'õ':
		hints[sightPortuguese]++
	case 'ą', 'ę', 'ł':
		hints[sightPolish]++
	case 'ř', 'ě', 'ů':
		hints[sightCzech]++
	case 'ő', 'ű':
		hints[sightHungarian]++
	case 'ş', 'ğ', 'ı':
		hints[sightTurkish]++
	case 'ă', 'ș', 'ț':
		hints[sightRomanian]++
	case 'đ', 'ơ', 'ư':
		hints[sightVietnamese]++
	case 'å':
		hints[sightSwedish]++
		hints[sightNorwegian]++
	case 'æ', 'ø':
		hints[sightNorwegian]++
	case 'ä', 'ö':
		hints[sightGerman]++
		hints[sightFinnish]++
		hints[sightSwedish]++
	}
}

func sightLatinLanguage(text string, hints [sightLatinLanguages]int) string {
	var hits [sightLatinLanguages]int
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsMark(r)
	})
	for _, word := range words {
		votes := sightStopwordVotes[word]
		for language := range hits {
			if votes&(1<<language) != 0 {
				hits[language]++
			}
		}
	}
	best, score, tied := -1, 0, false
	for language, count := range hits {
		candidate := count + hints[language]
		if candidate > score {
			best, score, tied = language, candidate, false
		} else if candidate == score {
			tied = true
		}
	}
	if best < 0 || tied || hits[best] < 2 {
		return ""
	}
	return sightStopwords[best].code
}
