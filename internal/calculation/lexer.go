package calculation

import (
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/make-no-mistakes-team/super-duper-calculator/contracts"
)

type token struct {
	text  string
	start int // byte and UTF-16 offset: accepted tokens contain only ASCII
}

func (t token) span() contracts.SourceSpan {
	return contracts.SourceSpan{Start: t.start, End: t.start + len(t.text)}
}

// tokenStream scans only when the parser requests a token. One-token lookahead
// supports function syntax without examining the rest of the expression. The
// length check and the scan each visit the input once; prefixes are not reparsed.
type tokenStream struct {
	source      string
	offset      int
	count       int
	buffered    token
	hasBuffered bool
}

func newTokenStream(source string) (*tokenStream, *contracts.MathError) {
	length := 0
	for _, r := range source {
		length += utf16.RuneLen(r)
		if length > maxLength {
			return nil, &contracts.MathError{Code: contracts.ErrorExpressionLimit, Stage: contracts.StageParse,
				Params: map[string]any{"length": maxLength}}
		}
	}
	return &tokenStream{source: source}, nil
}

func (s *tokenStream) next() (token, *contracts.MathError) {
	if s.hasBuffered {
		found := s.buffered
		s.hasBuffered = false
		return found, nil
	}
	return s.scan()
}

func (s *tokenStream) peek() (token, *contracts.MathError) {
	if s.hasBuffered {
		return s.buffered, nil
	}
	found, err := s.scan()
	if err == nil {
		s.buffered = found
		s.hasBuffered = true
	}
	return found, err
}

func (s *tokenStream) scan() (token, *contracts.MathError) {
	for s.offset < len(s.source) {
		b := s.source[s.offset]
		if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			break
		}
		s.offset++
	}
	if s.offset == len(s.source) {
		return token{}, nil
	}

	start := s.offset
	b := s.source[start]
	var text string
	switch {
	case b == '+', b == '-', b == '*', b == '/', b == '^', b == '(', b == ')', b == ',', b == '!', b == '%':
		s.offset++
		text = s.source[start:s.offset]

	case b >= '0' && b <= '9', b == '.':
		j := start
		digits := 0
		for j < len(s.source) && s.source[j] >= '0' && s.source[j] <= '9' {
			j++
			digits++
		}
		if j < len(s.source) && s.source[j] == '.' {
			j++
			for j < len(s.source) && s.source[j] >= '0' && s.source[j] <= '9' {
				j++
				digits++
			}
		}
		if digits == 0 {
			return token{}, &contracts.MathError{Code: contracts.ErrorSyntax, Stage: contracts.StageParse,
				Params: map[string]any{"expected": "digit"}, Span: &contracts.SourceSpan{Start: start, End: j}}
		}
		if j < len(s.source) && (s.source[j] == 'e' || s.source[j] == 'E') {
			e := j
			j++
			if j < len(s.source) && (s.source[j] == '+' || s.source[j] == '-') {
				j++
			}
			exponent := j
			for j < len(s.source) && s.source[j] >= '0' && s.source[j] <= '9' {
				j++
			}
			if j == exponent {
				return token{}, &contracts.MathError{Code: contracts.ErrorSyntax, Stage: contracts.StageParse,
					Params: map[string]any{"expected": "exponent"}, Span: &contracts.SourceSpan{Start: e, End: j}}
			}
		}
		// 1.2.3, 1e5e3, and 2pi have no implicit continuation.
		if j < len(s.source) {
			c := s.source[j]
			if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '.' {
				return token{}, &contracts.MathError{Code: contracts.ErrorSyntax, Stage: contracts.StageParse,
					Params: map[string]any{"unexpected": string(c)}, Span: &contracts.SourceSpan{Start: j, End: j + 1}}
			}
		}
		text = strings.ToLower(s.source[start:j])
		s.offset = j

	case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z':
		word := make([]byte, 0, 8)
		j := start
		for j < len(s.source) {
			c := s.source[j]
			if c >= 'A' && c <= 'Z' {
				c |= 0x20
			}
			if c < 'a' || c > 'z' {
				break
			}
			word = append(word, c)
			j++
		}
		text = string(word)
		s.offset = j

	default:
		if b < 0x80 {
			return token{}, &contracts.MathError{Code: contracts.ErrorSyntax, Stage: contracts.StageParse,
				Params: map[string]any{"unexpected": s.source[start : start+1]},
				Span:   &contracts.SourceSpan{Start: start, End: start + 1}}
		}
		r, _ := utf8.DecodeRuneInString(s.source[start:])
		return token{}, &contracts.MathError{Code: contracts.ErrorSyntax, Stage: contracts.StageParse,
			Params: map[string]any{"unexpected": string(r)},
			Span:   &contracts.SourceSpan{Start: start, End: start + utf16.RuneLen(r)}}
	}

	s.count++
	if s.count > maxTokens {
		return token{}, &contracts.MathError{Code: contracts.ErrorExpressionLimit, Stage: contracts.StageParse,
			Params: map[string]any{"tokens": maxTokens}}
	}
	return token{text: text, start: start}, nil
}
