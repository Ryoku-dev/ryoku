package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestOneAndTheSame_CorrectsAfterAreAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... are one in the same ...", NewOneAndTheSame(), "... are one and the same ...")
}

func TestOneAndTheSame_CorrectsAfterWereAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... were one in the same ...", NewOneAndTheSame(), "... were one and the same ...")
}

func TestOneAndTheSame_DoesntFlagAfterOtherWordsAtomic(t *testing.T) {
	linttest.AssertLintCount(t, "... and another one in the same place ...", NewOneAndTheSame(), 0)
}

func TestOneAndTheSame_CorrectsGithubAre(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yes, I believe they are one in the same.", NewOneAndTheSame(), "Yes, I believe they are one and the same.")
}

func TestOneAndTheSame_CorrectsGithubWere(t *testing.T) {
	linttest.AssertSuggestionResult(t, "As prior to OpenShift 4.0, OAuth and Kubernetes REST API were one in the same, option (2) above should still work there.", NewOneAndTheSame(), "As prior to OpenShift 4.0, OAuth and Kubernetes REST API were one and the same, option (2) above should still work there.")
}

func TestOneAndTheSame_CorrectsBeforeAsAtomic(t *testing.T) {
	linttest.AssertSuggestionResult(t, "... one in the same as ...", NewOneAndTheSame(), "... one and the same as ...")
}

func TestOneAndTheSame_CorrectsBeforeAsGithub(t *testing.T) {
	linttest.AssertSuggestionResult(t, "In our case the slicedState is one in the same as the featureState", NewOneAndTheSame(), "In our case the slicedState is one and the same as the featureState")
}

func TestOneAndTheSame_CorrectsAtEnd(t *testing.T) {
	t.Skip("needs zero-width end-of-chunk pattern akin to regex `$`")
	linttest.AssertSuggestionResult(t, "I think this is one in the same.", NewOneAndTheSame(), "I think this is one and the same.")
}

func TestOneAndTheSame_CorrectsIsAs(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I believe this and this issue is one in the same as Next.js uses cloudflare workers for it's edge infra.", NewOneAndTheSame(), "I believe this and this issue is one and the same as Next.js uses cloudflare workers for it's edge infra.")
}

func TestOneAndTheSame_AvoidsFalsePositive(t *testing.T) {
	linttest.AssertLintCount(t, "If there is no postgresql.pg_hba either there is one in the same section of patroni.yaml or pg_hba.conf is not managed by Patroni.", NewOneAndTheSame(), 0)
}

func TestOneAndTheSame_CorrectsIsOf(t *testing.T) {
	t.Skip("Cannot detect unexpected ungrammatical `same of`")
	linttest.AssertSuggestionResult(t, "R3 that Stephan Buhre noted is one-in-the-same of what I posted.", NewOneAndTheSame(), "R3 that Stephan Buhre noted is one and the same of what I posted.")
}

func TestOneAndTheSame_DoesntFlagAmbiguousBeforeNoun(t *testing.T) {
	linttest.AssertLintCount(t, "I'm guessing this is one in the same request.", NewOneAndTheSame(), 0)
}
