package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestGoodAt_FixGoodInBeingFrugal(t *testing.T) {
	linttest.AssertSuggestionResult(t, "but we found that Claude is not always very good in being frugal ( Gemini seemed better at it ) .", NewGoodAt(), "but we found that Claude is not always very good at being frugal ( Gemini seemed better at it ) .")
}

func TestGoodAt_FixImNotGoodInPython(t *testing.T) {
	linttest.AssertSuggestionResult(t, "can't run it i'm not good in python", NewGoodAt(), "can't run it i'm not good at python")
}

func TestGoodAt_FixNotGoodInGo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Hi, I have very similar problem and I'm not good in go either.", NewGoodAt(), "Hi, I have very similar problem and I'm not good at go either.")
}

func TestGoodAt_FixNotGoodInCodingStuff(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Unfortunately I can't help in anyway but testing, because I'm not good in coding stuff.", NewGoodAt(), "Unfortunately I can't help in anyway but testing, because I'm not good at coding stuff.")
}

func TestGoodAt_FixVeryGoodInMathematics(t *testing.T) {
	linttest.AssertSuggestionResult(t, "They were very good in Mathematics and were the pets of Ranjani Ma'am.", NewGoodAt(), "They were very good at Mathematics and were the pets of Ranjani Ma'am.")
}

func TestGoodAt_FixNotGoodInCoding(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Didnt know these things.. and since im not good in coding, maybe one day someone will work on this.", NewGoodAt(), "Didnt know these things.. and since im not good at coding, maybe one day someone will work on this.")
}

func TestGoodAt_FixVeryGoodInMostThings(t *testing.T) {
	linttest.AssertSuggestionResult(t, "But I do want to continue using riverpod because its very good in most things I need from my app.", NewGoodAt(), "But I do want to continue using riverpod because its very good at most things I need from my app.")
}

func TestGoodAt_FixNotGoodInLaravel(t *testing.T) {
	linttest.AssertSuggestionResult(t, "im not good in laravel.", NewGoodAt(), "im not good at laravel.")
}

func TestGoodAt_FiximNotGoodInEnglish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Sorry about my grammar im not good in English.", NewGoodAt(), "Sorry about my grammar im not good at English.")
}

func TestGoodAt_FixNotAllGoodInEnglish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I think if there is translation support will be great, our school society not all good in english.", NewGoodAt(), "I think if there is translation support will be great, our school society not all good at english.")
}

func TestGoodAt_FixIAmNotGoodInEnglish(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I am in Togo, i am not good in english but i will ask you to apologize me for my speaking.", NewGoodAt(), "I am in Togo, i am not good at english but i will ask you to apologize me for my speaking.")
}

func TestGoodAt_FixNotGoodInProgramming(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Is it My Drive,Chess or \"My Drive\",\"Chess\" or what? Because I'm not good in programming.", NewGoodAt(), "Is it My Drive,Chess or \"My Drive\",\"Chess\" or what? Because I'm not good at programming.")
}

func TestGoodAt_FixNotSoGoodInCoding(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I'm not so good in coding to understand how to handle all these bytes...", NewGoodAt(), "I'm not so good at coding to understand how to handle all these bytes...")
}

func TestGoodAt_FixImNotGoodInProgramming(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Im not good in programming , but can i ask what is the password of your codes?", NewGoodAt(), "Im not good at programming , but can i ask what is the password of your codes?")
}
