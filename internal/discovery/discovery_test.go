package discovery_test

import (
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/calculation"
	"github.com/make-no-mistakes-team/super-duper-calculator/internal/discovery"
)

func defaultRules(t *testing.T) discovery.Rules {
	t.Helper()
	rules, err := discovery.New(discovery.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	return rules
}

func assertMatches(t *testing.T, rules discovery.Rules, input discovery.Input, want ...string) {
	t.Helper()
	if got := rules.Match(input); !slices.Equal(got, want) {
		t.Errorf("Match(%+v) = %v, want %v", input, got, want)
	}
}

func success(id, expression, value string) contracts.CalculationRecord {
	return contracts.CalculationRecord{
		RequestID:  id,
		Expression: expression,
		Context:    contracts.CalculationContext{AngleUnit: contracts.Degrees},
		Outcome:    contracts.Outcome{Kind: "success", Value: value},
	}
}

func failure(id, expression string) contracts.CalculationRecord {
	record := success(id, expression, "")
	record.Outcome = contracts.Outcome{Kind: "error", Error: &contracts.MathError{Code: "DIVISION_BY_ZERO"}}
	return record
}

func engineRecord(t *testing.T, expression string) contracts.CalculationRecord {
	t.Helper()
	evaluation, err := calculation.New().Evaluate(t.Context(), calculation.Input{
		Expression: expression,
		AngleUnit:  contracts.Degrees,
	})
	if err != nil {
		t.Fatalf("Evaluate(%q): %v", expression, err)
	}
	return contracts.CalculationRecord{
		RequestID:  "action",
		Expression: expression,
		Context:    contracts.CalculationContext{AngleUnit: contracts.Degrees},
		Outcome:    evaluation.Outcome,
		Facts:      evaluation.Facts,
	}
}

func TestCanonicalResultRulesConsumeEngineOutcomes(t *testing.T) {
	rules := defaultRules(t)
	for _, test := range []struct {
		expression string
		want       []string
	}{
		{"40+2", []string{"answer_found"}},
		{"60+7", []string{"six_seven"}},
		{"70-1", []string{"nice_number"}},
		{"400+4", []string{"result_found"}},
		{"167", nil},
		{"67.00000000001", nil},
		{"67/0", nil},
		{"6*7", []string{"answer_found"}},
		{"67-1", nil},
		{"167-100", []string{"six_seven"}},
		{"41.99999999999999", nil},
		{"42.00000000001", nil},
		{"69.00000000001", nil},
		{"404.00000000001", nil},
		{"42/0", nil},
	} {
		t.Run(test.expression, func(t *testing.T) {
			assertMatches(t, rules, discovery.Input{Calculation: engineRecord(t, test.expression)}, test.want...)
		})
	}
	// A source containing the target text and an error outcome are not successes.
	errorWithValue := failure("error", "67/0")
	errorWithValue.Outcome.Value = "67"
	assertMatches(t, rules, discovery.Input{Calculation: errorWithValue})
}

func TestPeerReviewCountsDistinctConsecutiveAcceptedActions(t *testing.T) {
	rules := defaultRules(t)
	current := success("current", "2+2", "4")
	first := success("first", "2+2", "4")
	second := success("second", "2+2", "4")
	changedSource := success("source", "2 + 2", "4")
	changedAngle := success("angle", "2+2", "4")
	changedAngle.Context.AngleUnit = contracts.Radians
	for _, test := range []struct {
		name     string
		previous []contracts.CalculationRecord
		want     bool
	}{
		{"first action", nil, false},
		{"below threshold", []contracts.CalculationRecord{first}, false},
		{"three deliberate actions", []contracts.CalculationRecord{first, second}, true},
		{"retry cannot form a streak", []contracts.CalculationRecord{first, first}, false},
		{"retry between deliberate actions", []contracts.CalculationRecord{first, first, second}, true},
		{"retry of current action", []contracts.CalculationRecord{current, second}, false},
		{"retry beyond nominal window", []contracts.CalculationRecord{first, second, current}, false},
		{"changed source stops streak", []contracts.CalculationRecord{first, changedSource, second}, false},
		{"changed angle stops streak", []contracts.CalculationRecord{first, changedAngle, second}, false},
		{"accepted error stops streak", []contracts.CalculationRecord{first, failure("error", "2+2"), second}, false},
		{"unidentified action cannot form a streak", []contracts.CalculationRecord{success("", "2+2", "4"), second}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := discovery.Input{Calculation: current, Previous: test.previous}
			if got := slices.Contains(rules.Match(input), "peer_review"); got != test.want {
				t.Errorf("peer_review eligible = %v, want %v", got, test.want)
			}
		})
	}
	assertMatches(t, rules, discovery.Input{
		Calculation: failure("current-error", "2+2"),
		Previous:    []contracts.CalculationRecord{first, second},
	})
}

func TestParsedFactBoundaries(t *testing.T) {
	rules := defaultRules(t)
	for _, test := range []struct {
		name    string
		facts   *contracts.CalculationFacts
		outcome contracts.Outcome
		want    []string
	}{
		{"no parsed facts", nil, contracts.Outcome{Kind: "success", Value: "10"}, nil},
		{"depth five", &contracts.CalculationFacts{Depth: 5}, contracts.Outcome{Kind: "success", Value: "10"}, nil},
		{"depth six", &contracts.CalculationFacts{Depth: 6}, contracts.Outcome{Kind: "success", Value: "10"}, []string{"bracket_architect"}},
		{"three calls to one function", &contracts.CalculationFacts{Functions: map[string]int{"sqrt": 3}}, contracts.Outcome{Kind: "success", Value: "10"}, nil},
		{"three required functions", &contracts.CalculationFacts{Functions: map[string]int{"sqrt": 1, "abs": 2, "ln": 1}}, contracts.Outcome{Kind: "success", Value: "10"}, []string{"scientific_method"}},
		{"optional function does not count", &contracts.CalculationFacts{Functions: map[string]int{"sqrt": 1, "abs": 1, "mod": 1}}, contracts.Outcome{Kind: "success", Value: "10"}, nil},
		{"zero or negative usage does not count", &contracts.CalculationFacts{Functions: map[string]int{"sqrt": 1, "abs": 1, "ln": 0, "cos": -1}}, contracts.Outcome{Kind: "success", Value: "10"}, nil},
		{"parsed error earns neither", &contracts.CalculationFacts{Depth: 6, Functions: map[string]int{"sqrt": 1, "abs": 1, "ln": 1}}, contracts.Outcome{Kind: "error", Error: &contracts.MathError{Code: "DOMAIN_ERROR"}}, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := success("action", "source does not establish facts", "10")
			record.Facts = test.facts
			record.Outcome = test.outcome
			assertMatches(t, rules, discovery.Input{Calculation: record}, test.want...)
		})
	}
}

func TestFactRulesUseParsedEngineFacts(t *testing.T) {
	rules := defaultRules(t)
	for _, test := range []struct {
		expression string
		want       []string
	}{
		{"(((((1)))))", nil},
		{"((((((1))))))", []string{"bracket_architect"}},
		{"sqrt(81)+sqrt(1)+ln(1)", nil},
		{"sqrt(81)+abs(-1)+ln(1)", []string{"scientific_method"}},
	} {
		t.Run(test.expression, func(t *testing.T) {
			assertMatches(t, rules, discovery.Input{Calculation: engineRecord(t, test.expression)}, test.want...)
		})
	}
}

func TestCombinedMatchesFollowCatalogOrderWithoutProgress(t *testing.T) {
	rules := defaultRules(t)
	current := engineRecord(t, "((((((sqrt(81)+abs(-1)+ln(1)+57))))))")
	first, second := current, current
	first.RequestID, second.RequestID = "first", "second"
	input := discovery.Input{
		Calculation:   current,
		Previous:      []contracts.CalculationRecord{first, second},
		AcceptedCount: 25,
	}
	want := []string{"six_seven", "peer_review", "bracket_architect", "scientific_method", "touch_grass"}
	assertMatches(t, rules, input, want...)
	assertMatches(t, rules, input, want...)
}

func TestTouchGrassUsesAcceptedCountIncludingErrors(t *testing.T) {
	rules := defaultRules(t)
	current := failure("error", "1/0")
	assertMatches(t, rules, discovery.Input{Calculation: current, AcceptedCount: 24})
	input := discovery.Input{Calculation: current, AcceptedCount: 25}
	assertMatches(t, rules, input, "touch_grass")
	assertMatches(t, rules, input, "touch_grass") // Re-evaluation cannot advance a count.
	assertMatches(t, rules, discovery.Input{Calculation: success("next", "1+1", "2"), AcceptedCount: 26}, "touch_grass")
	// History and retries must not add to AcceptedCount; the rule does not
	// derive accepted actions from the supplied history window.
	previous := make([]contracts.CalculationRecord, 25)
	assertMatches(t, rules, discovery.Input{Calculation: current, AcceptedCount: 24, Previous: previous})
}

func TestConfigSelectionAndThresholds(t *testing.T) {
	config := discovery.DefaultConfig()
	config.Enabled = []string{"touch_grass", "scientific_method", "bracket_architect", "peer_review"}
	config.PeerReviewCount = 2
	config.BracketDepth = 5
	config.ScientificFunctions = 2
	config.TouchGrassCount = 24
	rules, err := discovery.New(config)
	if err != nil {
		t.Fatal(err)
	}
	current := success("current", "1", "67")
	current.Facts = &contracts.CalculationFacts{Depth: 5, Functions: map[string]int{"sqrt": 1, "ln": 1}}
	previous := []contracts.CalculationRecord{success("previous", "1", "1")}
	input := discovery.Input{Calculation: current, Previous: previous, AcceptedCount: 24}
	want := []string{"peer_review", "bracket_architect", "scientific_method", "touch_grass"}
	assertMatches(t, rules, input, want...)
	beforeFunctions := maps.Clone(current.Facts.Functions)
	beforePrevious := slices.Clone(previous)
	config.Enabled[0] = "six_seven"
	config.PeerReviewCount = 100
	config.BracketDepth = 100
	config.ScientificFunctions = 100
	config.TouchGrassCount = 100
	assertMatches(t, rules, input, want...)
	if !maps.Equal(current.Facts.Functions, beforeFunctions) || !reflect.DeepEqual(previous, beforePrevious) ||
		input.AcceptedCount != 24 || input.Calculation.RequestID != "current" {
		t.Fatal("Match mutated its input")
	}
	config = discovery.DefaultConfig()
	config.Enabled = nil
	none, err := discovery.New(config)
	if err != nil {
		t.Fatal(err)
	}
	assertMatches(t, none, input)
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*discovery.Config)
	}{
		{"peer count", func(c *discovery.Config) { c.PeerReviewCount = 0 }},
		{"bracket depth", func(c *discovery.Config) { c.BracketDepth = -1 }},
		{"scientific functions", func(c *discovery.Config) { c.ScientificFunctions = 0 }},
		{"accepted count", func(c *discovery.Config) { c.TouchGrassCount = -1 }},
		{"unknown ID", func(c *discovery.Config) { c.Enabled = append(c.Enabled, "not_a_rule") }},
		{"duplicate ID", func(c *discovery.Config) { c.Enabled = append(c.Enabled, "six_seven") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := discovery.DefaultConfig()
			test.change(&config)
			if _, err := discovery.New(config); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}

func TestCatalogStableLocalizedAndCopied(t *testing.T) {
	want := []string{
		"answer_found", "six_seven", "nice_number", "result_found",
		"peer_review", "bracket_architect", "scientific_method", "touch_grass",
	}
	definitions := discovery.Catalog()
	if len(definitions) != len(want) {
		t.Fatalf("catalog length = %d, want %d", len(definitions), len(want))
	}
	config := discovery.DefaultConfig()
	if !slices.Equal(config.Enabled, want) {
		t.Errorf("default enabled rules = %v, want %v", config.Enabled, want)
	}
	if config.PeerReviewCount != 3 || config.BracketDepth != 6 ||
		config.ScientificFunctions != 3 || config.TouchGrassCount != 25 {
		t.Errorf("default thresholds = %+v", config)
	}
	for i, definition := range definitions {
		if definition.ID != want[i] || !discovery.Known(definition.ID) {
			t.Errorf("catalog[%d] has unknown or reordered ID %q", i, definition.ID)
		}
		if definition.RU.Name == "" || definition.RU.Description == "" || definition.RU.Comment == "" ||
			definition.EN.Name == "" || definition.EN.Description == "" || definition.EN.Comment == "" {
			t.Errorf("catalog[%d] lacks localized copy", i)
		}
	}
	if discovery.Known("not_a_rule") {
		t.Fatal("unknown discovery accepted")
	}
	if definitions[1].RU.Comment != "Сикс-севен!" || definitions[1].EN.Comment != "Six seven!" {
		t.Fatal("required six-seven reaction is missing")
	}
	definitions[0].ID = "changed"
	definitions[1].EN.Comment = "changed"
	config.Enabled[0] = "changed"
	fresh := discovery.Catalog()
	if fresh[0].ID != want[0] || fresh[1].EN.Comment != "Six seven!" ||
		discovery.DefaultConfig().Enabled[0] != want[0] || !discovery.Known(want[0]) {
		t.Fatal("catalog or default configuration leaked caller mutation")
	}
}
