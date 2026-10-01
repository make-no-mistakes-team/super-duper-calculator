// Package discovery selects personal discoveries from trusted calculation records.
// Eligibility is separate from durable awards and presentation.
package discovery

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

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
	secondWind
	alternateRoutes
	quietAfterStorm
	paperTiger
	gainingAltitude
	mirrorRoom
	troubleCollector
	unscathed
	grandScale
	lastPixel
	parallelWorlds
	timeLoop
	fourthWall
	unexpectedTail
)

// Text is the authored copy for one locale.
type Text struct {
	Name        string
	Description string
	Comment     string
}

// Definition gives a discovery its stable ID and localized copy.
type Definition struct {
	ID     string
	Secret bool
	RU     Text
	EN     Text
}

var catalog = [...]Definition{
	answerFound: {
		ID: "answer_found", Secret: true,
		RU: Text{"Главный вопрос", "Получить точный результат 42.", "Ответ есть. Вопрос всё ещё открыт."},
		EN: Text{"The Big Question", "Get an exact result of 42.", "The answer is here. The question isn't."},
	},
	sixSeven: {
		ID: "six_seven", Secret: true,
		RU: Text{"Мем года", "Получить точный результат 67.", "Сикс-севен!"},
		EN: Text{"Meme of the Year", "Get an exact result of 67.", "Six seven!"},
	},
	niceNumber: {
		ID: "nice_number", Secret: true,
		RU: Text{"Тонкий намёк", "Получить точный результат 69.", "Nice."},
		EN: Text{"A Subtle Hint", "Get an exact result of 69.", "Nice."},
	},
	resultFound: {
		ID: "result_found", Secret: true,
		RU: Text{"Потерянный сигнал", "Получить точный результат 404.", "Ошибка 404: результат найден."},
		EN: Text{"Lost Signal", "Get an exact result of 404.", "404: result found."},
	},
	peerReview: {
		ID: "peer_review",
		RU: Text{"Всё под контролем", "Три раза подряд вычислить одно и то же выражение без ошибок.", "Проверили ещё раз. Ответ тот же."},
		EN: Text{"Under Control", "Calculate the same expression successfully three times in a row.", "Checked again. Same answer."},
	},
	bracketArchitect: {
		ID: "bracket_architect",
		RU: Text{"Внутренний мир", "Вычислить выражение с глубиной вложенности скобок не менее шести.", "Скобки держатся. Пока что."},
		EN: Text{"Inner World", "Calculate an expression with at least six levels of nested parentheses.", "The parentheses are holding up. For now."},
	},
	scientificMethod: {
		ID: "scientific_method",
		RU: Text{"Исследователь", "В одном успешном вычислении применить не менее трёх разных научных функций.", "Здесь уже не обойтись счётом на пальцах."},
		EN: Text{"Explorer", "Use at least three different scientific functions in one successful calculation.", "That's beyond counting on your fingers."},
	},
	touchGrass: {
		ID: "touch_grass",
		RU: Text{"Снаружи тоже жизнь", "Провести 25 вычислений; вычисления с математическими ошибками тоже считаются.", "Может, пора выйти погулять?"},
		EN: Text{"Life Outside", "Make 25 calculations; mathematical errors count too.", "Maybe take a walk?"},
	},
	secondWind: {
		ID: "second_wind",
		RU: Text{"Второе дыхание", "После математической ошибки успешно выполнить следующее вычисление.", "Ошибка осталась позади. Продолжаем."},
		EN: Text{"Second Wind", "Succeed on the next calculation after a mathematical error.", "The error is behind us. Carry on."},
	},
	alternateRoutes: {
		ID: "alternate_routes",
		RU: Text{"Другой маршрут", "Получить один и тот же точный результат в трёх выражениях с разной структурой операций или функций.", "Три маршрута. Одна точка назначения."},
		EN: Text{"Another Route", "Reach the same exact result through three different operator or function structures.", "Three routes. One destination."},
	},
	quietAfterStorm: {
		ID: "quiet_after_storm", Secret: true,
		RU: Text{"Тишина после бури", "Получить точный ноль в выражении с восемью или более операциями.", "Столько движения — и полная тишина."},
		EN: Text{"Quiet After the Storm", "Get exact zero from an expression with at least eight operations.", "All that motion, then complete silence."},
	},
	paperTiger: {
		ID: "paper_tiger", Secret: true,
		RU: Text{"Бумажный тигр", "Получить точную единицу, применив не менее трёх разных научных функций.", "Выглядело грозно. Оказалось единицей."},
		EN: Text{"Paper Tiger", "Get exact one using at least three different scientific functions.", "Looked fierce. Turned out to be one."},
	},
	gainingAltitude: {
		ID: "gaining_altitude",
		RU: Text{"Набираем высоту", "Выполнить пять успешных вычислений подряд; каждый следующий результат должен быть строго больше предыдущего.", "Каждый ответ — ступенькой выше."},
		EN: Text{"Gaining Altitude", "Get five consecutive successful results, each strictly greater than the last.", "Every answer is another step up."},
	},
	mirrorRoom: {
		ID: "mirror_room", Secret: true,
		RU: Text{"Зеркальная комната", "В трёх успешных вычислениях подряд получить ненулевое число, противоположное ему число и снова исходное.", "Отражение есть. Выход тоже найдётся."},
		EN: Text{"Mirror Room", "In three consecutive successes, get a nonzero number, its negation, then the original number.", "There's a reflection. We'll find the exit."},
	},
	troubleCollector: {
		ID: "trouble_collector",
		RU: Text{"Коллекционер неприятностей", "Встретить синтаксическую ошибку, деление на ноль и ошибку области определения.", "Три вида неприятностей. Коллекция завершена."},
		EN: Text{"Trouble Collector", "Encounter a syntax error, division by zero, and a domain error.", "Three kinds of trouble. Collection complete."},
	},
	unscathed: {
		ID: "unscathed",
		RU: Text{"Без единой царапины", "Успешно выполнить десять вычислений подряд без математических ошибок.", "Десять ответов. Ни одной царапины."},
		EN: Text{"Unscathed", "Complete ten consecutive calculations without a mathematical error.", "Ten answers. Not a scratch."},
	},
	grandScale: {
		ID: "grand_scale",
		RU: Text{"Большой размах", "Получить успешный результат, модуль которого строго больше 10¹².", "Этому числу тесно на обычной линейке."},
		EN: Text{"Grand Scale", "Get a successful result with absolute value strictly above 10¹².", "An ordinary ruler won't hold this one."},
	},
	lastPixel: {
		ID: "last_pixel",
		RU: Text{"Последний пиксель", "Получить ненулевой результат, модуль которого строго меньше 10⁻¹².", "Почти ничего. Но всё-таки не ноль."},
		EN: Text{"The Last Pixel", "Get a nonzero result with absolute value strictly below 10⁻¹².", "Almost nothing. Still not zero."},
	},
	parallelWorlds: {
		ID: "parallel_worlds", Secret: true,
		RU: Text{"Параллельные миры", "Получить один точный результат тригонометрических вычислений с оператором ° и без него.", "Градусы и радианы встретились в одном ответе."},
		EN: Text{"Parallel Worlds", "Get the same exact trigonometric result with and without the ° operator.", "Degrees and radians met at the same answer."},
	},
	timeLoop: {
		ID: "time_loop",
		RU: Text{"Петля времени", "Повторить успешное выражение не раньше чем через семь дней; пробелы, регистр и лишние скобки не важны.", "Прошла неделя. Выражение вернулось."},
		EN: Text{"Time Loop", "Repeat a successful expression at least seven days later; spacing, case, and redundant parentheses don't matter.", "A week passed. The expression came back."},
	},
	fourthWall: {
		ID: "fourth_wall", Secret: true,
		RU: Text{"Четвёртая стена", "Отправить отдельное слово «привет» или «hello»; математическая ошибка сохранится.", "Привет! Считать это я всё ещё не умею."},
		EN: Text{"The Fourth Wall", "Submit only “привет” or “hello”; the mathematical error is still saved.", "Hello! I still can't calculate that."},
	},
	unexpectedTail: {
		ID: "unexpected_tail", Secret: true,
		RU: Text{"Незваный хвост", "Выполнить хотя бы одну операцию и получить точный результат 0.30000000000000004.", "Хвост не приглашали. Он пришёл сам."},
		EN: Text{"An Unexpected Tail", "Perform at least one operation and get exactly 0.30000000000000004.", "Nobody invited the tail. It came anyway."},
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

// DefaultConfig selects all discoveries with their initial thresholds.
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

// Input supplies one accepted action, prior durable state and indexed evidence.
// The caller excludes retries/rejections and includes this action in AcceptedCount.
type Input struct {
	Calculation   contracts.CalculationRecord
	AcceptedCount int64
	State         *State
	Evidence      Evidence
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
		peer := r.peerReviewCount == 1
		if input.State != nil {
			peer = peer || current.Expression == input.State.PeerExpression && input.State.PeerStreak+1 >= r.peerReviewCount
		}
		if r.enabled[peerReview] && peer {
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
	state := State{}
	if input.State != nil {
		state = *input.State
	}
	success := current.Outcome.Kind == contracts.OutcomeSuccess
	value, numeric := numericValue(current.Outcome)
	facts := current.Facts
	eligible := [len(catalog)]bool{}
	eligible[secondWind] = success && state.LastWasError
	if success {
		if facts != nil {
			route := facts.StructureIdentity
			count := len(input.Evidence.Routes)
			if route != "" && !slices.Contains(input.Evidence.Routes, route) {
				count++
			}
			eligible[alternateRoutes] = route != "" && count >= 3
			eligible[quietAfterStorm] = current.Outcome.Value == "0" && facts.OperationCount >= 8
			eligible[paperTiger] = current.Outcome.Value == "1" && hasScientificFunctions(facts, 3)
			eligible[unexpectedTail] = current.Outcome.Value == "0.30000000000000004" && facts.OperationCount > 0
			trig := TrigVariant(facts)
			eligible[parallelWorlds] = trig != 0 && input.Evidence.Trig|trig == 3
			eligible[timeLoop] = facts.NormalizedExpression != "" && !input.Evidence.Earliest.IsZero() &&
				current.CreatedAt.Sub(input.Evidence.Earliest) >= 7*24*time.Hour
		}
		last, lastErr := strconv.ParseFloat(state.LastValue, 64)
		previous, prevErr := strconv.ParseFloat(state.PreviousValue, 64)
		eligible[gainingAltitude] = numeric && lastErr == nil && value > last && state.IncreasingStreak >= 4
		eligible[mirrorRoom] = numeric && value != 0 && state.SuccessStreak >= 2 &&
			lastErr == nil && prevErr == nil && value == previous && value == -last
		eligible[unscathed] = state.SuccessStreak >= 9
		eligible[grandScale] = numeric && math.Abs(value) > 1e12
		eligible[lastPixel] = numeric && math.Abs(value) > 0 && math.Abs(value) < 1e-12
	}
	eligible[troubleCollector] = state.ErrorKinds|errorKind(current.Outcome) == 7
	greeting := strings.ToLower(strings.TrimSpace(current.Expression))
	eligible[fourthWall] = current.Outcome.Kind == contracts.OutcomeError && (greeting == "привет" || greeting == "hello")
	for i := secondWind; i < len(catalog); i++ {
		if r.enabled[i] && eligible[i] {
			matches = append(matches, catalog[i].ID)
		}
	}
	return matches
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
