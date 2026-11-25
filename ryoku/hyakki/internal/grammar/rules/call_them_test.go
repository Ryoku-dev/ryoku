package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestCallThem_PreferPlugAndReceptacle(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I prefer to call them as Plug (male) and Receptacle (female). Receptacles are seen in laptops, mobile phones etc..", NewCallThem(), "I prefer to call them Plug (male) and Receptacle (female). Receptacles are seen in laptops, mobile phones etc..")
}

func TestCallThem_BuiltinsId(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I’d categorically ignore *id* as a builtin, and when you do need it in a module, make it super explicit and `import builtins` and call it as `builtins.id`.", NewCallThem(), "I’d categorically ignore *id* as a builtin, and when you do need it in a module, make it super explicit and `import builtins` and call it `builtins.id`.")
}

func TestCallThem_NonModalDialogue(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We usually call it as non-modal dialogue e.g. when hit Gmail compose button, a nonmodal dialogue opens.", NewCallThem(), "We usually call it non-modal dialogue e.g. when hit Gmail compose button, a nonmodal dialogue opens.")
}

func TestCallThem_PreferToCallThem(t *testing.T) {
	linttest.AssertSuggestionResult(t, "So, how do you typically prefer to call them as?", NewCallThem(), "So, how do you typically prefer to call them?")
}

func TestCallThem_CalledThemAllies(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yes as tribes or nomads you called them as allies but you didn’t get their levies as your own.", NewCallThem(), "Yes as tribes or nomads you called them allies but you didn’t get their levies as your own.")
}

func TestCallThem_CharacterDevelopment(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I call this as character development.", NewCallThem(), "I call this character development.")
}

func TestCallThem_FateOrTime(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Should I Call It As Fate Or Time", NewCallThem(), "Should I Call It Fate Or Time")
}

func TestCallThem_AbstractLatteArt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Can we just call it as abstract latte art.", NewCallThem(), "Can we just call it abstract latte art.")
}

func TestCallThem_SoundingBoards(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I call them as my ‘sounding boards’", NewCallThem(), "I call them my ‘sounding boards’")
}

func TestCallThem_CallingThemDisaster(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I totally disagree with your point listed and calling them as disaster.", NewCallThem(), "I totally disagree with your point listed and calling them disaster.")
}

func TestCallThem_BattleOfBoxes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Windows Sandbox and VirtualBox or I would like to call this as “Battle of Boxes.”", NewCallThem(), "Windows Sandbox and VirtualBox or I would like to call this “Battle of Boxes.”")
}

func TestCallThem_CalledHerShinnasan(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Nice meeting a follower from reddit I called her as Shinna-san, welcome again to Toram!!", NewCallThem(), "Nice meeting a follower from reddit I called her Shinna-san, welcome again to Toram!!")
}

func TestCallThem_CallingItOtp(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Calling it as OTP in this case misleading", NewCallThem(), "Calling it OTP in this case misleading")
}

func TestCallThem_CallItProcrastination(t *testing.T) {
	linttest.AssertSuggestionResult(t, "To summarise it in just one word I would call it as procrastination.", NewCallThem(), "To summarise it in just one word I would call it procrastination.")
}

func TestCallThem_CallHerImportant(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Liked the article overall but to call her as important to rap as Jay or Dre is a bold overstatement.", NewCallThem(), "Liked the article overall but to call her important to rap as Jay or Dre is a bold overstatement.")
}

func TestCallThem_CallHimKindles(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The days when I had my first best friend, I would rather call him as human version of kindle audiobook, who keeps on talking about everything under the umbrella.", NewCallThem(), "The days when I had my first best friend, I would rather call him human version of kindle audiobook, who keeps on talking about everything under the umbrella.")
}

func TestCallThem_CallThemDefenders(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Declaring war challenging land of a vassal should call them as defenders!", NewCallThem(), "Declaring war challenging land of a vassal should call them defenders!")
}

func TestCallThem_CallItMagical(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I would like to call it as magical.", NewCallThem(), "I would like to call it magical.")
}

func TestCallThem_ForwardLateral(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Surprised the refs didn’t call this as a forward lateral.", NewCallThem(), "Surprised the refs didn’t call this a forward lateral.")
}

func TestCallThem_CallingBestFriend(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Meet my buddy! I love calling him as my best friend, because he never failed to bring some cheer in me!", NewCallThem(), "Meet my buddy! I love calling him my best friend, because he never failed to bring some cheer in me!")
}

func TestCallThem_CallingEveryoneTitles(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Currently, I’m teaching in Asia and the students have the local custom of calling everyone as Mr. Givenname or Miss Givenname", NewCallThem(), "Currently, I’m teaching in Asia and the students have the local custom of calling everyone Mr. Givenname or Miss Givenname")
}

func TestCallThem_CalledAsHe(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I prefer to be called as he when referred in 3rd person and I’m sure that everyone would be ok to call me as he.", NewCallThem(), "I prefer to be called he when referred in 3rd person and I’m sure that everyone would be ok to call me he.")
}

func TestCallThem_CallsHimBob(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In Twelve Monkeys, Cole hears someone who calls him as “Bob”", NewCallThem(), "In Twelve Monkeys, Cole hears someone who calls him “Bob”")
}

func TestCallThem_PlinyCalledIt(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Pliny the Elder called it as lake of Gennesaret or Taricheae in his encyclopedia, Natural History.", NewCallThem(), "Pliny the Elder called it lake of Gennesaret or Taricheae in his encyclopedia, Natural History.")
}

func TestCallThem_StudentsCallYou(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In the same way your students will call you as ~先生 even after they graduated/move to higher education.", NewCallThem(), "In the same way your students will call you ~先生 even after they graduated/move to higher education.")
}

func TestCallThem_ParadoxicalReaction(t *testing.T) {
	linttest.AssertSuggestionResult(t, "We can call it as Paradoxical Reaction which means a medicine which is used to reduce pain increases the pain when it is", NewCallThem(), "We can call it Paradoxical Reaction which means a medicine which is used to reduce pain increases the pain when it is")
}

func TestCallThem_RustModule(t *testing.T) {
	linttest.AssertNoLints(t, "I want to call them as if they were just another Rust module", NewCallThem())
}

func TestCallThem_WantToDo(t *testing.T) {
	linttest.AssertNoLints(t, "however its a design choice to not call it as it does things I don't want to do.", NewCallThem())
}
