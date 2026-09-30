// Package discovery selects personal discoveries from trusted calculation records.
// Eligibility is separate from durable awards and presentation.
package discovery

import (
	"fmt"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

const (
	answerFound = iota
	sixSeven
	niceNumber
	resultFound
	peerReview
	bracketArchitect
	scientificMethod
	touchGrass
)

// Text is the authored copy for one locale.
type Text struct {
	Name        string
	Description string
	Comment     string
}

// Definition gives a discovery its stable ID and localized copy.
type Definition struct {
	ID string
	RU Text
	EN Text
}

var catalog = [...]Definition{
	answerFound: {
		ID: "answer_found",
		RU: Text{"Ответ найден", "Получить точный результат 42.", "Ответ есть. Вопрос всё ещё открыт."},
		EN: Text{"Answer found", "Get an exact result of 42.", "The answer is here. The question isn't."},
	},
	sixSeven: {
		ID: "six_seven",
		RU: Text{"Сикс-севен", "Получить точный результат 67.", "Сикс-севен!"},
		EN: Text{"Six seven", "Get an exact result of 67.", "Six seven!"},
	},
	niceNumber: {
		ID: "nice_number",
		RU: Text{"Красивое число", "Получить точный результат 69.", "Nice."},
		EN: Text{"Nice number", "Get an exact result of 69.", "Nice."},
	},
	resultFound: {
		ID: "result_found",
		RU: Text{"Результат найден", "Получить точный результат 404.", "Ошибка 404: результат найден."},
		EN: Text{"Result found", "Get an exact result of 404.", "404: result found."},
	},
	peerReview: {
		ID: "peer_review",
		RU: Text{"Рецензия пройдена", "Повторить без ошибок одно выражение в том же угловом режиме.", "Проверили ещё раз. Ответ тот же."},
		EN: Text{"Peer reviewed", "Repeat an expression without errors in the same angle mode.", "Checked again. Same answer."},
	},
	bracketArchitect: {
		ID: "bracket_architect",
		RU: Text{"Архитектор скобок", "Вычислить выражение с глубокой вложенностью скобок.", "Скобки держатся. Пока что."},
		EN: Text{"Bracket architect", "Calculate an expression with deeply nested parentheses.", "The parentheses are holding up. For now."},
	},
	scientificMethod: {
		ID: "scientific_method",
		RU: Text{"Научный метод", "В одном вычислении применить разные научные функции.", "Здесь уже не обойтись счётом на пальцах."},
		EN: Text{"Scientific method", "Use different scientific functions in one calculation.", "That's beyond counting on your fingers."},
	},
	touchGrass: {
		ID: "touch_grass",
		RU: Text{"Время сделать паузу", "Провести много вычислений, в том числе с ошибками.", "Может, пора выйти погулять?"},
		EN: Text{"Time for a break", "Make plenty of calculations, including ones that end in errors.", "Maybe take a walk?"},
	},
}

// Catalog returns all authored discoveries, independent of configuration.
// The returned slice can be changed without changing subsequent catalogs.
func Catalog() []Definition {
	definitions := make([]Definition, len(catalog))
	copy(definitions, catalog[:])
	return definitions
}

// Known reports whether id names a discovery in the authored catalog.
func Known(id string) bool {
	for i := range catalog {
		if catalog[i].ID == id {
			return true
		}
	}
	return false
}

// Config selects rules and their thresholds. An empty Enabled slice disables
// all rules; other fields must remain positive even for disabled rules.
type Config struct {
	PeerReviewCount     int
	BracketDepth        int
	ScientificFunctions int
	TouchGrassCount     int64
	Enabled             []string
}

// DefaultConfig selects all eight discoveries with their initial thresholds.
func DefaultConfig() Config {
	config := Config{
		PeerReviewCount:     3,
		BracketDepth:        6,
		ScientificFunctions: 3,
		TouchGrassCount:     25,
		Enabled:             make([]string, len(catalog)),
	}
	for i := range catalog {
		config.Enabled[i] = catalog[i].ID
	}
	return config
}

// Input is a snapshot of one accepted calculation and its owner's accepted
// actions immediately before it, newest first. The caller excludes retries,
// rejected requests and reads, and includes the current action in AcceptedCount.
type Input struct {
	Calculation   contracts.CalculationRecord
	AcceptedCount int64
	Previous      []contracts.CalculationRecord
}

// Rules is an immutable selection of discoveries and thresholds.
type Rules struct {
	enabled             [len(catalog)]bool
	peerReviewCount     int
	bracketDepth        int
	scientificFunctions int
	touchGrassCount     int64
}

// New validates config once and copies its selection without retaining Enabled.
func New(config Config) (Rules, error) {
	if config.PeerReviewCount <= 0 {
		return Rules{}, fmt.Errorf("peer review count must be positive: %d", config.PeerReviewCount)
	}
	if config.BracketDepth <= 0 {
		return Rules{}, fmt.Errorf("bracket depth must be positive: %d", config.BracketDepth)
	}
	if config.ScientificFunctions <= 0 {
		return Rules{}, fmt.Errorf("scientific function count must be positive: %d", config.ScientificFunctions)
	}
	if config.TouchGrassCount <= 0 {
		return Rules{}, fmt.Errorf("touch grass count must be positive: %d", config.TouchGrassCount)
	}

	rules := Rules{
		peerReviewCount:     config.PeerReviewCount,
		bracketDepth:        config.BracketDepth,
		scientificFunctions: config.ScientificFunctions,
		touchGrassCount:     config.TouchGrassCount,
	}
	for _, id := range config.Enabled {
		index := -1
		for i := range catalog {
			if id == catalog[i].ID {
				index = i
				break
			}
		}
		if index == -1 {
			return Rules{}, fmt.Errorf("unknown discovery %q", id)
		}
		if rules.enabled[index] {
			return Rules{}, fmt.Errorf("duplicate discovery %q", id)
		}
		rules.enabled[index] = true
	}
	return rules, nil
}

// Match selects eligible IDs in catalog order. It does not advance a streak,
// award a discovery or alter the snapshot; re-evaluating it gives the same IDs.
func (r Rules) Match(input Input) []string {
	current := &input.Calculation
	var matches []string
	if current.Outcome.Kind == contracts.OutcomeSuccess {
		switch current.Outcome.Value {
		case "42":
			if r.enabled[answerFound] {
				matches = append(matches, catalog[answerFound].ID)
			}
		case "67":
			if r.enabled[sixSeven] {
				matches = append(matches, catalog[sixSeven].ID)
			}
		case "69":
			if r.enabled[niceNumber] {
				matches = append(matches, catalog[niceNumber].ID)
			}
		case "404":
			if r.enabled[resultFound] {
				matches = append(matches, catalog[resultFound].ID)
			}
		}
		if r.enabled[peerReview] && r.matchesPeerReview(&input) {
			matches = append(matches, catalog[peerReview].ID)
		}
		if r.enabled[bracketArchitect] && current.Facts != nil && current.Facts.Depth >= r.bracketDepth {
			matches = append(matches, catalog[bracketArchitect].ID)
		}
		if r.enabled[scientificMethod] && hasScientificFunctions(current.Facts, r.scientificFunctions) {
			matches = append(matches, catalog[scientificMethod].ID)
		}
	}
	if r.enabled[touchGrass] && input.AcceptedCount >= r.touchGrassCount {
		matches = append(matches, catalog[touchGrass].ID)
	}
	return matches
}

func (r Rules) matchesPeerReview(input *Input) bool {
	current := &input.Calculation
	if current.RequestID == "" || len(input.Previous) < r.peerReviewCount-1 {
		return false
	}
	// A retry of the current action is not a new deliberate calculation,
	// even when older entries already supply the requested streak length.
	for i := range input.Previous {
		if input.Previous[i].RequestID == current.RequestID {
			return false
		}
	}
	count := 1
	if count >= r.peerReviewCount {
		return true
	}
	for i := range input.Previous {
		previous := &input.Previous[i]
		if previous.Outcome.Kind != contracts.OutcomeSuccess || previous.Expression != current.Expression ||
			previous.Context.AngleUnit != current.Context.AngleUnit || previous.RequestID == "" {
			return false
		}
		// An accidentally repeated action in a supplied window is not a new
		// deliberate calculation. Look farther back for the next distinct one.
		duplicate := false
		for j := range i {
			if input.Previous[j].RequestID == previous.RequestID {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		count++
		if count >= r.peerReviewCount {
			return true
		}
	}
	return false
}

func hasScientificFunctions(facts *contracts.CalculationFacts, required int) bool {
	if facts == nil {
		return false
	}
	count := 0
	for name, uses := range facts.Functions {
		if uses <= 0 || !requiredFunction(name) {
			continue
		}
		count++
		if count >= required {
			return true
		}
	}
	return false
}

func requiredFunction(name string) bool {
	switch name {
	case "sqrt", "abs", "exp", "ln", "log", "sin", "cos", "tan", "asin", "acos", "atan":
		return true
	default:
		return false
	}
}
