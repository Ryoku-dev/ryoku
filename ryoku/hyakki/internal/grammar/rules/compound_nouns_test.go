package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCompoundNouns_WebCam(t *testing.T) {
	test_sentence := "The web cam captured a stunning image."
	expected := "The webcam captured a stunning image."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_NoteBook(t *testing.T) {
	test_sentence := "She always carries a note book to jot down ideas."
	expected := "She always carries a notebook to jot down ideas."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_MotherBoard(t *testing.T) {
	test_sentence := "After the upgrade, the mother board was replaced."
	expected := "After the upgrade, the motherboard was replaced."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_SmartPhone(t *testing.T) {
	test_sentence := "He bought a new smart phone last week."
	expected := "He bought a new smartphone last week."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_FirmWare(t *testing.T) {
	test_sentence := "The device's firm ware was updated overnight."
	expected := "The device's firmware was updated overnight."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_BackPlane(t *testing.T) {
	test_sentence := "A reliable back plane is essential for high-speed data transfer."
	expected := "A reliable backplane is essential for high-speed data transfer."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_SpreadSheet(t *testing.T) {
	test_sentence := "The accountant reviewed the spread sheet carefully."
	expected := "The accountant reviewed the spreadsheet carefully."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_SideBar(t *testing.T) {
	test_sentence := "The website's side bar offers quick navigation links."
	expected := "The website's sidebar offers quick navigation links."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_BackPack(t *testing.T) {
	test_sentence := "I packed my books in my back pack before leaving."
	expected := "I packed my books in my backpack before leaving."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_CupBoard(t *testing.T) {
	test_sentence := "She stored the dishes in the old cup board."
	expected := "She stored the dishes in the old cupboard."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_KeyBoard(t *testing.T) {
	test_sentence := "My key board stopped working during the meeting."
	expected := "My keyboard stopped working during the meeting."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_TouchScreen(t *testing.T) {
	test_sentence := "The device features a responsive touch screen."
	expected := "The device features a responsive touchscreen."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_HeadSet(t *testing.T) {
	test_sentence := "He bought a new head set for his workouts."
	expected := "He bought a new headset for his workouts."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_FrameWork(t *testing.T) {
	test_sentence := "The frame work of the app was built with care."
	expected := "The framework of the app was built with care."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_TouchPad(t *testing.T) {
	test_sentence := "The touch pad on my laptop is very sensitive."
	expected := "The touchpad on my laptop is very sensitive."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_MicroProcessor(t *testing.T) {
	test_sentence := "This micro processor is among the fastest available."
	expected := "This microprocessor is among the fastest available."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_HeadPhone(t *testing.T) {
	test_sentence := "I lost my head phone at the gym."
	expected := "I lost my headphone at the gym."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_MicroServices(t *testing.T) {
	test_sentence := "Our architecture now relies on micro services."
	expected := "Our architecture now relies on microservices."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_DashBoard(t *testing.T) {
	test_sentence := "The dash board shows real-time analytics."
	expected := "The dashboard shows real-time analytics."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_SiteMap(t *testing.T) {
	test_sentence := "A site map is provided at the footer of the website."
	expected := "A sitemap is provided at the footer of the website."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_FireWall(t *testing.T) {
	test_sentence := "A robust fire wall is essential for network security."
	expected := "A robust firewall is essential for network security."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_BitStream(t *testing.T) {
	test_sentence := "The bit stream was interrupted during transmission."
	expected := "The bitstream was interrupted during transmission."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_BlockChain(t *testing.T) {
	test_sentence := "The block chain is revolutionizing the financial sector."
	expected := "The blockchain is revolutionizing the financial sector."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_ThumbNail(t *testing.T) {
	test_sentence := "I saved the image as a thumb nail."
	expected := "I saved the image as a thumbnail."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_BathRoom(t *testing.T) {
	test_sentence := "They remodeled the bath room entirely."
	expected := "They remodeled the bathroom entirely."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_EveryOne(t *testing.T) {
	t.Skip("\"everyone\" is not a valid compound noun, it's a pronoun")
	test_sentence := "Every one should have access to quality education."
	expected := "Everyone should have access to quality education."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_PlayGround(t *testing.T) {
	test_sentence := "The kids spent the afternoon at the play ground."
	expected := "The kids spent the afternoon at the playground."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_RunWay(t *testing.T) {
	test_sentence := "The airplane taxied along the run way."
	expected := "The airplane taxied along the runway."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_CyberSpace(t *testing.T) {
	test_sentence := "Hackers roam the cyber space freely."
	expected := "Hackers roam the cyberspace freely."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_CyberAttack(t *testing.T) {
	test_sentence := "The network was hit by a cyber attack."
	expected := "The network was hit by a cyberattack."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_WebSocket(t *testing.T) {
	test_sentence := "Real-time updates are sent via a web socket."
	expected := "Real-time updates are sent via a websocket."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_FingerPrint(t *testing.T) {
	test_sentence := "The detective collected a finger print as evidence."
	expected := "The detective collected a fingerprint as evidence."
	linttest.AssertSuggestionResult(t, test_sentence, NewCompoundNouns(), expected)
}

func TestCompoundNouns_GotIsNotPossessive(t *testing.T) {
	linttest.AssertLintCount(t, "I got here by car...", NewCompoundNouns(), 0)
}

func TestCompoundNouns_AllowIssue662(t *testing.T) {
	linttest.AssertLintCount(t, "They are as old as *modern* computers ", NewCompoundNouns(), 0)
}

func TestCompoundNouns_AllowIssue661(t *testing.T) {
	linttest.AssertLintCount(t, "I may be wrong.", NewCompoundNouns(), 0)
}

func TestCompoundNouns_AllowIssue704(t *testing.T) {
	linttest.AssertLintCount(t, "Here are some ways to do that:", NewCompoundNouns(), 0)
}

func TestCompoundNouns_AllowsIssue721(t *testing.T) {
	linttest.AssertLintCount(t, "So if you adjust any one of these adjusters that can have a negative or a positive effect.", NewCompoundNouns(), 0)
}

func TestCompoundNouns_Allows678(t *testing.T) {
	linttest.AssertLintCount(t, "they can't catch all the bugs.", NewCompoundNouns(), 0)
}

func TestCompoundNouns_InaNotSuggested(t *testing.T) {
	linttest.AssertLintCount(t, "past mistakes or a character in a looping reality facing personal challenges.", NewCompoundNouns(), 0)
}

func TestCompoundNouns_AllowSuppressOr(t *testing.T) {
	linttest.AssertLintCount(t, "He must decide whether to suppress or coexist with his doppelgänger.", NewCompoundNouns(), 0)
}

func TestCompoundNouns_AllowAnArmAndALeg(t *testing.T) {
	linttest.AssertLintCount(t, "I have to pay an arm and a leg get a worker to come and be my assistant baker.", NewCompoundNouns(), 0)
}

func TestCompoundNouns_AllowWellAnd723(t *testing.T) {
	linttest.AssertLintCount(t, "I understood very well and decided to go.", NewCompoundNouns(), 0)
}

func TestCompoundNouns_AllowCanNot(t *testing.T) {
	linttest.AssertLintCount(t, "Size can not be determined.", NewCompoundNouns(), 0)
}

func TestCompoundNouns_DontFlagLotTo(t *testing.T) {
	linttest.AssertLintCount(t, "but you'd have to raise taxes a lot to do it.", NewCompoundNouns(), 0)
}

func TestCompoundNouns_DontFlagToMe(t *testing.T) {
	linttest.AssertLintCount(t, "There's no massive damage to the rockers or anything that to me would indicate that like the whole front of the car was off", NewCompoundNouns(), 0)
}

func TestCompoundNouns_AllowIssue1553(t *testing.T) {
	linttest.AssertNoLints(t, "I'm not sure if there's anyone else that may be interested in more fine-grained control, but as it stands, having the domain level toggle is sufficient for me.", NewCompoundNouns())
}

func TestCompoundNouns_AllowIssue1496(t *testing.T) {
	linttest.AssertNoLints(t, "I am not able to respond to messages.", NewCompoundNouns())
}

func TestCompoundNouns_AllowIssue1298(t *testing.T) {
	linttest.AssertNoLints(t, "A series of tests that cover all possible cases.", NewCompoundNouns())
}

func TestCompoundNouns_DontFlagProjectOr(t *testing.T) {
	linttest.AssertNoLints(t, "You can star or watch this project or follow author to get release notifications in time.", NewCompoundNouns())
}

func TestCompoundNouns_AllowIssue3950(t *testing.T) {
	linttest.AssertNoLints(t, "She is a high school teacher.", NewCompoundNouns())
}

func TestCompoundNouns_AllowPublicKeyServer(t *testing.T) {
	linttest.AssertNoLints(t, "Alternatively, you can import them from a public key server.", NewCompoundNouns())
}

func TestCompoundNouns_StillFlagsAfterUnrelatedAdjective(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The device features a responsive touch screen.", NewCompoundNouns(), "The device features a responsive touchscreen.")
}

func TestCompoundNouns_FixBigBreakThrough(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think a big break through is needed for AGI so I haven’t been worried about it.", NewCompoundNouns(), "I think a big breakthrough is needed for AGI so I haven’t been worried about it.")
}
