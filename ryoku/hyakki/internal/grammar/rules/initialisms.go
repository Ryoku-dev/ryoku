package rules

import "ryoku-rashin/internal/grammar/lint"

func NewInitialismsLintGroup() *lint.LintGroup {
	g := lint.NewLintGroup()
	g.AddChunkExprLinter("AsFarAsICanTell", lint.NewInitialismLinter("afaict", []string{"as far as I can tell"}))
	g.AddChunkExprLinter("AsFarAsIKnow", lint.NewInitialismLinter("afaik", []string{"as far as I know"}))
	g.AddChunkExprLinter("AsIUnderstandIt", lint.NewInitialismLinter("aiui", []string{"as I understand it"}))
	g.AddChunkExprLinter("BeRightBack", lint.NewInitialismLinter("brb", []string{"be right back"}))
	g.AddChunkExprLinter("ByTheWay", lint.NewInitialismLinter("btw", []string{"by the way"}))
	g.AddChunkExprLinter("ExplainLikeImFive", lint.NewInitialismLinter("eli5", []string{"explain like i'm five"}))
	g.AddChunkExprLinter("ForWhatItsWorth", lint.NewInitialismLinter("fwiw", []string{"for what it's worth"}))
	g.AddChunkExprLinter("ForYourInformation", lint.NewInitialismLinter("fyi", []string{"for your information"}))
	g.AddChunkExprLinter("IDontKnow", lint.NewInitialismLinter("idk", []string{"I don't know"}))
	g.AddChunkExprLinter("IfIRecallCorrectly", lint.NewInitialismLinter("iirc", []string{"if I recall correctly"}))
	g.AddChunkExprLinter("IfIUnderstandCorrectly", lint.NewInitialismLinter("iiuc", []string{"if I understand correctly"}))
	g.AddChunkExprLinter("IfYouKnowYouKnow", lint.NewInitialismLinter("iykyk", []string{"if you know, you know"}))
	g.AddChunkExprLinter("InCaseYouMissedIt", lint.NewInitialismLinter("icymi", []string{"in case you missed it"}))
	g.AddChunkExprLinter("InMyHumbleOpinion", lint.NewInitialismLinter("imho", []string{"in my humble opinion", "in my honest opinion"}))
	g.AddChunkExprLinter("InMyOpinion", lint.NewInitialismLinter("imo", []string{"in my opinion"}))
	g.AddChunkExprLinter("InRealLife", lint.NewInitialismLinter("irl", []string{"in real life"}))
	g.AddChunkExprLinter("LetMeKnow", lint.NewInitialismLinter("lmk", []string{"let me know"}))
	g.AddChunkExprLinter("NeverMind", lint.NewInitialismLinter("nvm", []string{"never mind"}))
	g.AddChunkExprLinter("OhMyGod", lint.NewInitialismLinter("omg", []string{"oh my god"}))
	g.AddChunkExprLinter("OnTheOtherHand", lint.NewInitialismLinter("otoh", []string{"on the other hand"}))
	g.AddChunkExprLinter("OutOfTheBox", lint.NewInitialismLinter("ootb", []string{"out of the box"}))
	g.AddChunkExprLinter("PleaseTakeALook", lint.NewInitialismLinter("ptal", []string{"please take a look"}))
	g.AddChunkExprLinter("Really", lint.NewInitialismLinter("rly", []string{"really"}))
	g.AddChunkExprLinter("TalkToYouLater", lint.NewInitialismLinter("ttyl", []string{"talk to you later"}))
	g.AddChunkExprLinter("ToBeFair", lint.NewInitialismLinter("tbf", []string{"to be fair"}))
	g.AddChunkExprLinter("ToBeHonest", lint.NewInitialismLinter("tbh", []string{"to be honest"}))
	enabled := true
	g.SetAllRulesTo(&enabled)
	return g
}
