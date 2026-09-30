// Package contracts contains the shared core data model.
package contracts

import "time"

type AngleUnit string

const (
	Degrees AngleUnit = "deg"
	Radians AngleUnit = "rad"
)

// OutcomeKind distinguishes a result from a stored mathematical error.
type OutcomeKind string

const (
	OutcomeSuccess OutcomeKind = "success"
	OutcomeError   OutcomeKind = "error"
)

// ErrorStage identifies whether a mathematical diagnostic came from parsing or evaluation.
type ErrorStage string

const (
	StageParse    ErrorStage = "parse"
	StageEvaluate ErrorStage = "evaluate"
)

// MathErrorCode names mathematical diagnostics without changing their JSON spelling.
// Expression limits reject a request rather than create a stored error outcome.
type MathErrorCode string

const (
	ErrorSyntax             MathErrorCode = "SYNTAX_ERROR"
	ErrorUnknownIdentifier  MathErrorCode = "UNKNOWN_IDENTIFIER"
	ErrorUnsupportedFeature MathErrorCode = "UNSUPPORTED_FEATURE"
	ErrorWrongArity         MathErrorCode = "WRONG_ARITY"
	ErrorDivisionByZero     MathErrorCode = "DIVISION_BY_ZERO"
	ErrorDomain             MathErrorCode = "DOMAIN_ERROR"
	ErrorNumericOverflow    MathErrorCode = "NUMERIC_OVERFLOW"
	ErrorExpressionLimit    MathErrorCode = "EXPRESSION_LIMIT"
)

type CapabilityLimits struct {
	ExpressionLength int `json:"expressionLength"`
	Tokens           int `json:"tokens"`
	Nesting          int `json:"nesting"`
}

type CapabilityFeatures struct {
	Factorial              bool `json:"factorial"`
	Percentage             bool `json:"percentage"`
	Remainder              bool `json:"remainder"`
	Statistics             bool `json:"statistics"`
	Achievements           bool `json:"achievements"`
	Themes                 bool `json:"themes"`
	MinimalPresentation    bool `json:"minimalPresentation"`
	Localization           bool `json:"localization"`
	ReductionPlayback      bool `json:"reductionPlayback"`
	Rooms                  bool `json:"rooms"`
	RoomReactions          bool `json:"roomReactions"`
	RoomPublicationControl bool `json:"roomPublicationControl"`
}

type Capabilities struct {
	SemanticsVersion string             `json:"semanticsVersion"`
	Operators        []string           `json:"operators"`
	Functions        map[string][]int   `json:"functions"`
	AngleUnits       []AngleUnit        `json:"angleUnits"`
	DefaultAngleUnit AngleUnit          `json:"defaultAngleUnit"`
	Limits           CapabilityLimits   `json:"limits"`
	Features         CapabilityFeatures `json:"features"`
}

type SourceSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type MathError struct {
	Code   MathErrorCode  `json:"code"`
	Stage  ErrorStage     `json:"stage"`
	Params map[string]any `json:"params"`
	Span   *SourceSpan    `json:"span"`
}

// Success has Value; a mathematical error has Error. The engine owns formatting.
type Outcome struct {
	Kind  OutcomeKind `json:"kind"`
	Value string      `json:"value,omitempty"`
	Error *MathError  `json:"error,omitempty"`
}

type CalculationContext struct {
	AngleUnit        AngleUnit `json:"angleUnit"`
	SemanticsVersion string    `json:"semanticsVersion"`
}

type CalculationFacts struct {
	Operators      map[string]int `json:"operators"`
	Functions      map[string]int `json:"functions"`
	OperationCount int            `json:"operationCount"`
	Depth          int            `json:"depth"`
}

type RoomContext struct {
	Code    string `json:"code"`
	Publish bool   `json:"publish"`
}

type CalculationRequest struct {
	RequestID  string       `json:"requestId"`
	Expression string       `json:"expression"`
	AngleUnit  AngleUnit    `json:"angleUnit"`
	Room       *RoomContext `json:"room,omitempty"`
}

type CalculationRecord struct {
	ID         string             `json:"id"`
	RequestID  string             `json:"requestId"`
	Expression string             `json:"expression"`
	Context    CalculationContext `json:"context"`
	Outcome    Outcome            `json:"outcome"`
	Facts      *CalculationFacts  `json:"facts,omitempty"`
	CreatedAt  time.Time          `json:"createdAt"`
}

type Publication struct {
	Status string `json:"status"`
}

type CalculationResponse struct {
	Calculation  CalculationRecord `json:"calculation"`
	Publication  Publication       `json:"publication"`
	Achievements []Achievement     `json:"achievements,omitempty"`
	FunEvents    []FunEvent        `json:"funEvents,omitempty"`
}

type HistoryPage struct {
	Items      []CalculationRecord `json:"items"`
	NextCursor *string             `json:"nextCursor"`
}

type LongestExpression struct {
	CalculationID string `json:"calculationId"`
	Expression    string `json:"expression"`
	Length        int    `json:"length"`
}

type PersonalStatistics struct {
	TotalCalculations      int64              `json:"totalCalculations"`
	Successes              int64              `json:"successes"`
	MathematicalErrors     int64              `json:"mathematicalErrors"`
	DivisionByZeroAttempts int64              `json:"divisionByZeroAttempts"`
	Operators              map[string]int64   `json:"operators"`
	Functions              map[string]int64   `json:"functions"`
	LongestExpression      *LongestExpression `json:"longestExpression"`
	MaxParsedDepth         *int               `json:"maxParsedDepth"`
}

type Achievement struct {
	ID       string    `json:"id"`
	EarnedAt time.Time `json:"earnedAt"`
}

type DiscoveryText struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Comment     string `json:"comment"`
}

type DiscoveryDefinition struct {
	ID string        `json:"id"`
	RU DiscoveryText `json:"ru"`
	EN DiscoveryText `json:"en"`
}

type FunEvent struct {
	ID        string         `json:"id"`
	RuleID    string         `json:"ruleId"`
	Kind      string         `json:"kind"`
	Scope     string         `json:"scope"`
	Params    map[string]any `json:"params"`
	CreatedAt time.Time      `json:"createdAt"`
	ExpiresAt time.Time      `json:"expiresAt"`
}

type SessionResponse struct {
	Alias                string                `json:"alias"`
	Identity             string                `json:"identity"`
	Achievements         []Achievement         `json:"achievements,omitempty"`
	DiscoveryCatalog     []DiscoveryDefinition `json:"discoveryCatalog,omitempty"`
	DiscoveriesAvailable bool                  `json:"discoveriesAvailable"`
}

type APIError struct {
	Code   string         `json:"code"`
	Params map[string]any `json:"params"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}
