// Derived from oh-my-pi (MIT), packages/coding-agent/src/discovery/builtin-rules/{go-ioutil,go-join-hostport,go-rand-v2,ts-no-any,ts-bare-catch,rs-result-type}.md@c7fff67.
package rules

import "ryoku-rashin/internal/protocol"

func builtinRules() []protocol.Rule {
	return []protocol.Rule{
		{Name: "go-ioutil", Description: "Use io and os instead of deprecated io/ioutil", Condition: []string{`"io/ioutil"`}, Scope: "tool:write(*.go), tool:edit(*.go)", Body: "New Go code imports io or os instead of io/ioutil. ReadAll and NopCloser live in io; ReadFile, WriteFile, ReadDir and CreateTemp live in os. Check ReadDir's os.DirEntry return type.", Enabled: true, Interrupt: protocol.RuleInterruptNever},
		{Name: "go-join-hostport", Description: "Use net.JoinHostPort for constructed host:port addresses", Condition: []string{`fmt\.Sprintf\("%s:%d"|host\s*\+\s*":"\s*\+`}, Scope: "tool:write(*.go), tool:edit(*.go)", Body: "Construct network addresses with net.JoinHostPort(host, port), not string concatenation. JoinHostPort handles IPv6 brackets correctly.", Enabled: true, Interrupt: protocol.RuleInterruptNever},
		{Name: "go-rand-v2", Description: "Use math/rand/v2 for new non-cryptographic random numbers", Condition: []string{`"math/rand"`}, Scope: "tool:write(*.go), tool:edit(*.go)", Body: "For new Go code use math/rand/v2 for non-cryptographic random values. For secrets use crypto/rand. Leave existing seeded rand/v1 programs alone if changing their sequence would be harmful.", Enabled: true, Interrupt: protocol.RuleInterruptNever},
		{Name: "ts-no-any", Description: "Avoid unchecked any in TypeScript annotations and assertions", Condition: []string{`(?m)^(?![ \t]*(?://|/\*|\*(?:\s|/)))(?:(?!//)[^\n])*?(?:: |\bas )any(?![\w$.]|[ \t]*\?\.|[ \t]+(?!(?:as|satisfies|in|instanceof|extends)\b)[A-Za-z_$])`}, Scope: "tool:write(*.ts), tool:write(*.tsx), tool:edit(*.ts), tool:edit(*.tsx)", Body: "Use unknown for unvalidated input, a precise type, generic, schema parse or type guard rather than : any or as any. A checked library boundary may need a justified cast.", Enabled: true, Interrupt: protocol.RuleInterruptNever},
		{Name: "rs-result-type", Description: "Default the error parameter in Rust Result aliases", Condition: []string{`type\s+Result<[A-Za-z_]\w*>\s*=`}, Scope: "tool:write(*.rs), tool:edit(*.rs)", Body: "Use a defaulted error parameter in Result aliases, for example type Result<T, E = anyhow::Error> = std::result::Result<T, E>.", Enabled: true, Interrupt: protocol.RuleInterruptNever},
	}
}
