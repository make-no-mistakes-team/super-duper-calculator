// Package contracts contains the shared core data model.
package contracts

import "time"

type AngleUnit string

const (
	Degrees AngleUnit = "deg"
	Radians AngleUnit = "rad"
)

type SourceSpan struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type MathError struct {
	Code   string         `json:"code"`
	Stage  string         `json:"stage"`
	Params map[string]any `json:"params"`
	Span   *SourceSpan    `json:"span"`
}

// Success has Value; a mathematical error has Error. The engine owns formatting.
type Outcome struct {
	Kind  string     `json:"kind"`
	Value string     `json:"value,omitempty"`
	Error *MathError `json:"error,omitempty"`
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
	Calculation CalculationRecord `json:"calculation"`
	Publication Publication       `json:"publication"`
}

type HistoryPage struct {
	Items      []CalculationRecord `json:"items"`
	NextCursor *string             `json:"nextCursor"`
}

type APIError struct {
	Code   string         `json:"code"`
	Params map[string]any `json:"params"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}
