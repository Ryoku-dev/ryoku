package lint

func (k LintKind) IsAgreement() bool      { return k == Agreement }
func (k LintKind) IsBoundaryError() bool  { return k == BoundaryError }
func (k LintKind) IsCapitalization() bool { return k == Capitalization }
func (k LintKind) IsEggcorn() bool        { return k == Eggcorn }
func (k LintKind) IsEnhancement() bool    { return k == Enhancement }
func (k LintKind) IsFormatting() bool     { return k == Formatting }
func (k LintKind) IsGrammar() bool        { return k == Grammar }
func (k LintKind) IsMalapropism() bool    { return k == Malapropism }
func (k LintKind) IsMiscellaneous() bool  { return k == Miscellaneous }
func (k LintKind) IsNonstandard() bool    { return k == Nonstandard }
func (k LintKind) IsPunctuation() bool    { return k == Punctuation }
func (k LintKind) IsReadability() bool    { return k == Readability }
func (k LintKind) IsRedundancy() bool     { return k == Redundancy }
func (k LintKind) IsRegionalism() bool    { return k == Regionalism }
func (k LintKind) IsRepetition() bool     { return k == Repetition }
func (k LintKind) IsSpelling() bool       { return k == Spelling }
func (k LintKind) IsStyle() bool          { return k == Style }
func (k LintKind) IsTypo() bool           { return k == Typo }
func (k LintKind) IsUsage() bool          { return k == Usage }
func (k LintKind) IsWordChoice() bool     { return k == WordChoice }
func (k LintKind) IsWordOrder() bool      { return k == WordOrder }
