package rules

import (
	"ryoku-rashin/internal/grammar/linttest"
	"testing"
)

func TestSummarySummery_SummaryDress(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Beautiful summary dress product code mf 78.", NewSummarySummery(), "Beautiful summery dress product code mf 78.")
}

func TestSummarySummery_ExecutiveSummery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I will also post a sample executive summery found online.", NewSummarySummery(), "I will also post a sample executive summary found online.")
}

func TestSummarySummery_FinancialSummery(t *testing.T) {
	linttest.AssertSuggestionResult(t, "BOQs Financial Summery Application ADE 5838.", NewSummarySummery(), "BOQs Financial Summary Application ADE 5838.")
}

func TestSummarySummery_SummeryAbout(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Again, thanks for the nice summery about the fix.", NewSummarySummery(), "Again, thanks for the nice summary about the fix.")
}

func TestSummarySummery_SummeryAbove(t *testing.T) {
	linttest.AssertSuggestionResult(t, "One issue I can think of immediately while reading @gautelund's summery above is with the proposal", NewSummarySummery(), "One issue I can think of immediately while reading @gautelund's summary above is with the proposal")
}

func TestSummarySummery_SummeryBelow(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Report Summery Below", NewSummarySummery(), "Report Summary Below")
}

func TestSummarySummery_SummeryBy(t *testing.T) {
	linttest.AssertSuggestionResult(t, "node has crashed my system hard here is a summery by claude", NewSummarySummery(), "node has crashed my system hard here is a summary by claude")
}

func TestSummarySummery_SummeryFor(t *testing.T) {
	linttest.AssertSuggestionResult(t, "A tool that trys to make condensed summery for Bugzilla bugs", NewSummarySummery(), "A tool that trys to make condensed summary for Bugzilla bugs")
}

func TestSummarySummery_SummeryIn(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Scrapes and generates a descriptive summery in pdf from any url", NewSummarySummery(), "Scrapes and generates a descriptive summary in pdf from any url")
}

func TestSummarySummery_SummeryInto(t *testing.T) {
	linttest.AssertSuggestionResult(t, "It would be awesome if you can put your summery into API documentation", NewSummarySummery(), "It would be awesome if you can put your summary into API documentation")
}

func TestSummarySummery_SummeryOf(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Summery of the contamination and progress.", NewSummarySummery(), "Summary of the contamination and progress.")
}

func TestSummarySummery_SummeryTo(t *testing.T) {
	linttest.AssertSuggestionResult(t, "add archive summery to the console output/log.", NewSummarySummery(), "add archive summary to the console output/log.")
}

func TestSummarySummery_SummeryWith(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Yet not code-coverage summery with coverage percentage and missing lines", NewSummarySummery(), "Yet not code-coverage summary with coverage percentage and missing lines")
}

func TestSummarySummery_SummeryBox(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How to remove summery box in magento review form", NewSummarySummery(), "How to remove summary box in magento review form")
}

func TestSummarySummery_SummeryBoxes(t *testing.T) {
	linttest.AssertSuggestionResult(t, "those summery boxes are quite inviting because they do not only show the logo but also an example picture of the speaker/receiver", NewSummarySummery(), "those summary boxes are quite inviting because they do not only show the logo but also an example picture of the speaker/receiver")
}

func TestSummarySummery_SummeryControl(t *testing.T) {
	linttest.AssertSuggestionResult(t, "have successfully implemented this for the Validation Summery control, input controls and ValidationToolTips", NewSummarySummery(), "have successfully implemented this for the Validation Summary control, input controls and ValidationToolTips")
}

func TestSummarySummery_SummeryDiv(t *testing.T) {
	linttest.AssertSuggestionResult(t, "does form and summery DIV in same view ?", NewSummarySummery(), "does form and summary DIV in same view ?")
}

func TestSummarySummery_SummeryElement(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How use summery element for label in cell of table view?", NewSummarySummery(), "How use summary element for label in cell of table view?")
}

func TestSummarySummery_SummeryField(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Option without the summery field (if we can automate it via the description).", NewSummarySummery(), "Option without the summary field (if we can automate it via the description).")
}

func TestSummarySummery_SummeryFields(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Add missing summery fields whenever is required", NewSummarySummery(), "Add missing summary fields whenever is required")
}

func TestSummarySummery_SummeryFieldAllCaps(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The event with empty SUMMERY-field is not imported.", NewSummarySummery(), "The event with empty SUMMARY-field is not imported.")
}

func TestSummarySummery_SummeryLog(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Here is summery log: summary.log. Any advise please?", NewSummarySummery(), "Here is summary log: summary.log. Any advise please?")
}

func TestSummarySummery_SummeryObject(t *testing.T) {
	linttest.AssertSuggestionResult(t, "I could print the filerev summery object between the filerev task and the usemin task", NewSummarySummery(), "I could print the filerev summary object between the filerev task and the usemin task")
}

func TestSummarySummery_SummeryHyphenPage(t *testing.T) {
	linttest.AssertSuggestionResult(t, "The heat-map on the summery-page in the month-overview (stats) ignores the first 10 days.", NewSummarySummery(), "The heat-map on the summary-page in the month-overview (stats) ignores the first 10 days.")
}

func TestSummarySummery_SummeryRow(t *testing.T) {
	linttest.AssertSuggestionResult(t, "Automatically adding a month-summery row at the end of each month?", NewSummarySummery(), "Automatically adding a month-summery row at the end of each month?")
}

func TestSummarySummery_SummeryScreen(t *testing.T) {
	linttest.AssertSuggestionResult(t, "On the summery screen the four graphs at the bottom (Energy, Thermals, Disk,Network) are constantly resizing", NewSummarySummery(), "On the summary screen the four graphs at the bottom (Energy, Thermals, Disk,Network) are constantly resizing")
}

func TestSummarySummery_SummerySection(t *testing.T) {
	linttest.AssertSuggestionResult(t, "How can I change WooCommerce checkout page order summery section style?", NewSummarySummery(), "How can I change WooCommerce checkout page order summary section style?")
}

func TestSummarySummery_SummeryTab(t *testing.T) {
	linttest.AssertSuggestionResult(t, "double click on that bar and it should give you more infomation in the Summery tab", NewSummarySummery(), "double click on that bar and it should give you more infomation in the Summary tab")
}
