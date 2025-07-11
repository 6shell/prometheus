// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parser

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/prometheus/prometheus/promql/parser/posrange"
)

// Item represents a token or text string returned from the scanner.
type Item struct {
	Typ ItemType     // The type of this Item.
	Pos posrange.Pos // The starting position, in bytes, of this Item in the input string.
	Val string       // The value of this Item.
}

// String returns a descriptive string for the Item.
func (i Item) String() string {
	switch {
	case i.Typ == EOF:
		return "EOF"
	case i.Typ == ERROR:
		return i.Val
	case i.Typ == IDENTIFIER || i.Typ == METRIC_IDENTIFIER:
		return fmt.Sprintf("%q", i.Val)
	case i.Typ.IsKeyword():
		return fmt.Sprintf("<%s>", i.Val)
	case i.Typ.IsOperator():
		return fmt.Sprintf("<op:%s>", i.Val)
	case i.Typ.IsAggregator():
		return fmt.Sprintf("<aggr:%s>", i.Val)
	case len(i.Val) > 10:
		return fmt.Sprintf("%.10q...", i.Val)
	}
	return fmt.Sprintf("%q", i.Val)
}

// Pretty returns the prettified form of an item.
// This is same as the item's stringified format.
func (i Item) Pretty(int) string { return i.String() }

// IsOperator returns true if the Item corresponds to a arithmetic or set operator.
// Returns false otherwise.
func (i ItemType) IsOperator() bool { return i > operatorsStart && i < operatorsEnd }

// IsAggregator returns true if the Item belongs to the aggregator functions.
// Returns false otherwise.
func (i ItemType) IsAggregator() bool { return i > aggregatorsStart && i < aggregatorsEnd }

// IsAggregatorWithParam returns true if the Item is an aggregator that takes a parameter.
// Returns false otherwise.
func (i ItemType) IsAggregatorWithParam() bool {
	return i == TOPK || i == BOTTOMK || i == COUNT_VALUES || i == QUANTILE || i == LIMITK || i == LIMIT_RATIO
}

// IsExperimentalAggregator defines the experimental aggregation functions that are controlled
// with EnableExperimentalFunctions.
func (i ItemType) IsExperimentalAggregator() bool {
	return i == LIMITK || i == LIMIT_RATIO
}

// IsKeyword returns true if the Item corresponds to a keyword.
// Returns false otherwise.
func (i ItemType) IsKeyword() bool { return i > keywordsStart && i < keywordsEnd }

// IsComparisonOperator returns true if the Item corresponds to a comparison operator.
// Returns false otherwise.
func (i ItemType) IsComparisonOperator() bool {
	switch i {
	case EQLC, NEQ, LTE, LSS, GTE, GTR:
		return true
	default:
		return false
	}
}

// IsSetOperator returns whether the Item corresponds to a set operator.
func (i ItemType) IsSetOperator() bool {
	switch i {
	case LAND, LOR, LUNLESS:
		return true
	}
	return false
}

type ItemType int

// This is a list of all keywords in PromQL.
// When changing this list, make sure to also change
// the maybe_label grammar rule in the generated parser
// to avoid misinterpretation of labels as keywords.
var key = map[string]ItemType{
	// Operators.
	"and":    LAND,
	"or":     LOR,
	"unless": LUNLESS,
	"atan2":  ATAN2,

	// Aggregators.
	"sum":          SUM,
	"avg":          AVG,
	"count":        COUNT,
	"min":          MIN,
	"max":          MAX,
	"group":        GROUP,
	"stddev":       STDDEV,
	"stdvar":       STDVAR,
	"topk":         TOPK,
	"bottomk":      BOTTOMK,
	"count_values": COUNT_VALUES,
	"quantile":     QUANTILE,
	"limitk":       LIMITK,
	"limit_ratio":  LIMIT_RATIO,

	// Keywords.
	"offset":      OFFSET,
	"by":          BY,
	"without":     WITHOUT,
	"on":          ON,
	"ignoring":    IGNORING,
	"group_left":  GROUP_LEFT,
	"group_right": GROUP_RIGHT,
	"bool":        BOOL,

	// Preprocessors.
	"start": START,
	"end":   END,
	"step":  STEP,
}

var histogramDesc = map[string]ItemType{
	"sum":                SUM_DESC,
	"count":              COUNT_DESC,
	"schema":             SCHEMA_DESC,
	"offset":             OFFSET_DESC,
	"n_offset":           NEGATIVE_OFFSET_DESC,
	"buckets":            BUCKETS_DESC,
	"n_buckets":          NEGATIVE_BUCKETS_DESC,
	"z_bucket":           ZERO_BUCKET_DESC,
	"z_bucket_w":         ZERO_BUCKET_WIDTH_DESC,
	"custom_values":      CUSTOM_VALUES_DESC,
	"counter_reset_hint": COUNTER_RESET_HINT_DESC,
}

var counterResetHints = map[string]ItemType{
	"unknown":   UNKNOWN_COUNTER_RESET,
	"reset":     COUNTER_RESET,
	"not_reset": NOT_COUNTER_RESET,
	"gauge":     GAUGE_TYPE,
}

// ItemTypeStr is the default string representations for common Items. It does not
// imply that those are the only character sequences that can be lexed to such an Item.
var ItemTypeStr = map[ItemType]string{
	OPEN_HIST:     "{{",
	CLOSE_HIST:    "}}",
	LEFT_PAREN:    "(",
	RIGHT_PAREN:   ")",
	LEFT_BRACE:    "{",
	RIGHT_BRACE:   "}",
	LEFT_BRACKET:  "[",
	RIGHT_BRACKET: "]",
	COMMA:         ",",
	EQL:           "=",
	COLON:         ":",
	SEMICOLON:     ";",
	BLANK:         "_",
	TIMES:         "x",
	SPACE:         "<space>",

	SUB:       "-",
	ADD:       "+",
	MUL:       "*",
	MOD:       "%",
	DIV:       "/",
	EQLC:      "==",
	NEQ:       "!=",
	LTE:       "<=",
	LSS:       "<",
	GTE:       ">=",
	GTR:       ">",
	EQL_REGEX: "=~",
	NEQ_REGEX: "!~",
	POW:       "^",
}

func init() {
	// Add keywords to Item type strings.
	for s, ty := range key {
		ItemTypeStr[ty] = s
	}
	// Special numbers.
	key["inf"] = NUMBER
	key["nan"] = NUMBER
}

func (i ItemType) String() string {
	if s, ok := ItemTypeStr[i]; ok {
		return s
	}
	return fmt.Sprintf("<Item %d>", i)
}

func (i Item) desc() string {
	if _, ok := ItemTypeStr[i.Typ]; ok {
		return i.String()
	}
	if i.Typ == EOF {
		return i.Typ.desc()
	}
	return fmt.Sprintf("%s %s", i.Typ.desc(), i)
}

func (i ItemType) desc() string {
	switch i {
	case ERROR:
		return "error"
	case EOF:
		return "end of input"
	case COMMENT:
		return "comment"
	case IDENTIFIER:
		return "identifier"
	case METRIC_IDENTIFIER:
		return "metric identifier"
	case STRING:
		return "string"
	case NUMBER:
		return "number"
	case DURATION:
		return "duration"
	}
	return fmt.Sprintf("%q", i)
}

const eof = -1

// stateFn represents the state of the scanner as a function that returns the next state.
type stateFn func(*Lexer) stateFn

type histogramState int

const (
	histogramStateNone histogramState = iota
	histogramStateOpen
	histogramStateMul
	histogramStateAdd
	histogramStateSub
)

// Lexer holds the state of the scanner.
type Lexer struct {
	input       string       // The string being scanned.
	state       stateFn      // The next lexing function to enter.
	pos         posrange.Pos // Current position in the input.
	start       posrange.Pos // Start position of this Item.
	width       posrange.Pos // Width of last rune read from input.
	lastPos     posrange.Pos // Position of most recent Item returned by NextItem.
	itemp       *Item        // Pointer to where the next scanned item should be placed.
	scannedItem bool         // Set to true every time an item is scanned.

	parenDepth  int  // Nesting depth of ( ) exprs.
	braceOpen   bool // Whether a { is opened.
	bracketOpen bool // Whether a [ is opened.
	gotColon    bool // Whether we got a ':' after [ was opened.
	gotDuration bool // Whether we got a duration after [ was opened.
	stringOpen  rune // Quote rune of the string currently being read.

	// series description variables for internal PromQL testing framework as well as in promtool rules unit tests.
	// see https://prometheus.io/docs/prometheus/latest/configuration/unit_testing_rules/#series
	seriesDesc     bool           // Whether we are lexing a series description.
	histogramState histogramState // Determines whether or not inside of a histogram description.
}

// next returns the next rune in the input.
func (l *Lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		l.width = 0
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.width = posrange.Pos(w)
	l.pos += l.width
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *Lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *Lexer) backup() {
	l.pos -= l.width
}

// emit passes an Item back to the client.
func (l *Lexer) emit(t ItemType) {
	*l.itemp = Item{t, l.start, l.input[l.start:l.pos]}
	l.start = l.pos
	l.scannedItem = true
}

// ignore skips over the pending input before this point.
func (l *Lexer) ignore() {
	l.start = l.pos
}

// accept consumes the next rune if it's from the valid set.
func (l *Lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}
	l.backup()
	return false
}

// is peeks and returns true if the next rune is contained in the provided string.
func (l *Lexer) is(valid string) bool {
	return strings.ContainsRune(valid, l.peek())
}

// acceptRun consumes a run of runes from the valid set.
func (l *Lexer) acceptRun(valid string) {
	for strings.ContainsRune(valid, l.next()) {
		// Consume.
	}
	l.backup()
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.NextItem.
func (l *Lexer) errorf(format string, args ...interface{}) stateFn {
	*l.itemp = Item{ERROR, l.start, fmt.Sprintf(format, args...)}
	l.scannedItem = true

	return nil
}

// NextItem writes the next item to the provided address.
func (l *Lexer) NextItem(itemp *Item) {
	l.scannedItem = false
	l.itemp = itemp

	if l.state != nil {
		for !l.scannedItem {
			l.state = l.state(l)
		}
	} else {
		l.emit(EOF)
	}

	l.lastPos = l.itemp.Pos
}

// Lex creates a new scanner for the input string.
func Lex(input string) *Lexer {
	l := &Lexer{
		input: input,
		state: lexStatements,
	}
	return l
}

// lineComment is the character that starts a line comment.
const lineComment = "#"

// lexStatements is the top-level state for lexing.
func lexStatements(l *Lexer) stateFn {
	if l.histogramState != histogramStateNone {
		return lexHistogram
	}
	if l.braceOpen {
		return lexInsideBraces
	}
	if strings.HasPrefix(l.input[l.pos:], lineComment) {
		return lexLineComment
	}

	switch r := l.next(); {
	case r == eof:
		switch {
		case l.parenDepth != 0:
			return l.errorf("unclosed left parenthesis")
		case l.bracketOpen:
			return l.errorf("unclosed left bracket")
		}
		l.emit(EOF)
		return nil
	case r == ',':
		l.emit(COMMA)
	case isSpace(r):
		return lexSpace
	case r == '*':
		l.emit(MUL)
	case r == '/':
		l.emit(DIV)
	case r == '%':
		l.emit(MOD)
	case r == '+':
		l.emit(ADD)
	case r == '-':
		l.emit(SUB)
	case r == '^':
		l.emit(POW)
	case r == '=':
		switch t := l.peek(); t {
		case '=':
			l.next()
			l.emit(EQLC)
		case '~':
			return l.errorf("unexpected character after '=': %q", t)
		default:
			l.emit(EQL)
		}
	case r == '!':
		if t := l.next(); t != '=' {
			return l.errorf("unexpected character after '!': %q", t)
		}
		l.emit(NEQ)
	case r == '<':
		if t := l.peek(); t == '=' {
			l.next()
			l.emit(LTE)
		} else {
			l.emit(LSS)
		}
	case r == '>':
		if t := l.peek(); t == '=' {
			l.next()
			l.emit(GTE)
		} else {
			l.emit(GTR)
		}
	case isDigit(r) || (r == '.' && isDigit(l.peek())):
		l.backup()
		return lexNumberOrDuration
	case r == '"' || r == '\'':
		l.stringOpen = r
		return lexString
	case r == '`':
		l.stringOpen = r
		return lexRawString
	case isAlpha(r) || r == ':':
		if !l.bracketOpen {
			l.backup()
			return lexKeywordOrIdentifier
		}
		switch r {
		case ':':
			if l.gotColon {
				return l.errorf("unexpected colon %q", r)
			}
			l.emit(COLON)
			l.gotColon = true
			return lexStatements
		case 's', 'S', 'm', 'M':
			if l.scanDurationKeyword() {
				return lexStatements
			}
		}
		return l.errorf("unexpected character: %q, expected %q", r, ':')
	case r == '(':
		l.emit(LEFT_PAREN)
		l.parenDepth++
		return lexStatements
	case r == ')':
		l.emit(RIGHT_PAREN)
		l.parenDepth--
		if l.parenDepth < 0 {
			return l.errorf("unexpected right parenthesis %q", r)
		}
		return lexStatements
	case r == '{':
		l.emit(LEFT_BRACE)
		l.braceOpen = true
		return lexInsideBraces
	case r == '[':
		if l.bracketOpen {
			return l.errorf("unexpected left bracket %q", r)
		}
		l.gotColon = false
		l.emit(LEFT_BRACKET)
		if isSpace(l.peek()) {
			skipSpaces(l)
		}
		l.bracketOpen = true
		return lexDurationExpr
	case r == ']':
		if !l.bracketOpen {
			return l.errorf("unexpected right bracket %q", r)
		}
		l.emit(RIGHT_BRACKET)
		l.bracketOpen = false
	case r == '@':
		l.emit(AT)
	default:
		return l.errorf("unexpected character: %q", r)
	}
	return lexStatements
}

func lexHistogram(l *Lexer) stateFn {
	switch l.histogramState {
	case histogramStateMul:
		l.histogramState = histogramStateNone
		l.next()
		l.emit(TIMES)
		return lexValueSequence
	case histogramStateAdd:
		l.histogramState = histogramStateNone
		l.next()
		l.emit(ADD)
		return lexValueSequence
	case histogramStateSub:
		l.histogramState = histogramStateNone
		l.next()
		l.emit(SUB)
		return lexValueSequence
	}

	if l.bracketOpen {
		return lexBuckets
	}
	switch r := l.next(); {
	case isSpace(r):
		l.emit(SPACE)
		return lexSpace
	case isAlpha(r):
		l.backup()
		return lexHistogramDescriptor
	case r == ':':
		l.emit(COLON)
		return lexHistogram
	case r == '-':
		l.emit(SUB)
		return lexHistogram
	case r == 'x':
		l.emit(TIMES)
		return lexNumber
	case isDigit(r):
		l.backup()
		return lexNumber
	case r == '[':
		l.bracketOpen = true
		l.gotColon = false
		l.gotDuration = false
		l.emit(LEFT_BRACKET)
		return lexBuckets
	case r == '}' && l.peek() == '}':
		l.next()
		l.emit(CLOSE_HIST)
		switch l.peek() {
		case 'x':
			l.histogramState = histogramStateMul
			return lexHistogram
		case '+':
			l.histogramState = histogramStateAdd
			return lexHistogram
		case '-':
			l.histogramState = histogramStateSub
			return lexHistogram
		default:
			l.histogramState = histogramStateNone
			return lexValueSequence
		}
	default:
		return l.errorf("histogram description incomplete unexpected: %q", r)
	}
}

func lexHistogramDescriptor(l *Lexer) stateFn {
Loop:
	for {
		switch r := l.next(); {
		case isAlpha(r):
			// absorb.
		default:
			l.backup()

			word := l.input[l.start:l.pos]
			if desc, ok := histogramDesc[strings.ToLower(word)]; ok {
				if l.peek() == ':' {
					l.emit(desc)
					return lexHistogram
				}
				l.errorf("missing `:` for histogram descriptor")
				break Loop
			}
			// Current word is Inf or NaN.
			if desc, ok := key[strings.ToLower(word)]; ok {
				if desc == NUMBER {
					l.emit(desc)
					return lexHistogram
				}
			}
			if desc, ok := counterResetHints[strings.ToLower(word)]; ok {
				l.emit(desc)
				return lexHistogram
			}

			l.errorf("bad histogram descriptor found: %q", word)
			break Loop
		}
	}
	return lexStatements
}

func lexBuckets(l *Lexer) stateFn {
	switch r := l.next(); {
	case isSpace(r):
		l.emit(SPACE)
		return lexSpace
	case r == '-':
		l.emit(SUB)
		return lexNumber
	case isDigit(r):
		l.backup()
		return lexNumber
	case r == ']':
		l.bracketOpen = false
		l.emit(RIGHT_BRACKET)
		return lexHistogram
	case isAlpha(r):
		// Current word is Inf or NaN.
		word := l.input[l.start:l.pos]
		if desc, ok := key[strings.ToLower(word)]; ok {
			if desc == NUMBER {
				l.emit(desc)
				return lexStatements
			}
		}
		return lexBuckets
	default:
		return l.errorf("invalid character in buckets description: %q", r)
	}
}

// lexInsideBraces scans the inside of a vector selector. Keywords are ignored and
// scanned as identifiers.
func lexInsideBraces(l *Lexer) stateFn {
	if strings.HasPrefix(l.input[l.pos:], lineComment) {
		return lexLineComment
	}

	switch r := l.next(); {
	case r == eof:
		return l.errorf("unexpected end of input inside braces")
	case isSpace(r):
		return lexSpace
	case isAlpha(r):
		l.backup()
		return lexIdentifier
	case r == ',':
		l.emit(COMMA)
	case r == '"' || r == '\'':
		l.stringOpen = r
		return lexString
	case r == '`':
		l.stringOpen = r
		return lexRawString
	case r == '=':
		if l.next() == '~' {
			l.emit(EQL_REGEX)
			break
		}
		l.backup()
		l.emit(EQL)
	case r == '!':
		switch nr := l.next(); nr {
		case '~':
			l.emit(NEQ_REGEX)
		case '=':
			l.emit(NEQ)
		default:
			return l.errorf("unexpected character after '!' inside braces: %q", nr)
		}
	case r == '{':
		return l.errorf("unexpected left brace %q", r)
	case r == '}':
		l.emit(RIGHT_BRACE)
		l.braceOpen = false

		if l.seriesDesc {
			return lexValueSequence
		}
		return lexStatements
	default:
		return l.errorf("unexpected character inside braces: %q", r)
	}
	return lexInsideBraces
}

// lexValueSequence scans a value sequence of a series description.
func lexValueSequence(l *Lexer) stateFn {
	if l.histogramState != histogramStateNone {
		return lexHistogram
	}
	switch r := l.next(); {
	case r == eof:
		return lexStatements
	case r == '{' && l.peek() == '{':
		if l.histogramState != histogramStateNone {
			return l.errorf("unexpected histogram opening {{")
		}
		l.histogramState = histogramStateOpen
		l.next()
		l.emit(OPEN_HIST)
		return lexHistogram
	case isSpace(r):
		l.emit(SPACE)
		lexSpace(l)
	case r == '+':
		l.emit(ADD)
	case r == '-':
		l.emit(SUB)
	case r == 'x':
		l.emit(TIMES)
	case r == '_':
		l.emit(BLANK)
	case isDigit(r) || (r == '.' && isDigit(l.peek())):
		l.backup()
		lexNumber(l)
	case isAlpha(r):
		l.backup()
		// We might lex invalid Items here but this will be caught by the parser.
		return lexKeywordOrIdentifier
	default:
		return l.errorf("unexpected character in series sequence: %q", r)
	}
	return lexValueSequence
}

// lexEscape scans a string escape sequence. The initial escaping character (\)
// has already been seen.
//
// NOTE: This function as well as the helper function digitVal() and associated
// tests have been adapted from the corresponding functions in the "go/scanner"
// package of the Go standard library to work for Prometheus-style strings.
// None of the actual escaping/quoting logic was changed in this function - it
// was only modified to integrate with our lexer.
func lexEscape(l *Lexer) stateFn {
	var n int
	var base, maxVal uint32

	ch := l.next()
	switch ch {
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', l.stringOpen:
		return lexString
	case '0', '1', '2', '3', '4', '5', '6', '7':
		n, base, maxVal = 3, 8, 255
	case 'x':
		ch = l.next()
		n, base, maxVal = 2, 16, 255
	case 'u':
		ch = l.next()
		n, base, maxVal = 4, 16, unicode.MaxRune
	case 'U':
		ch = l.next()
		n, base, maxVal = 8, 16, unicode.MaxRune
	case eof:
		l.errorf("escape sequence not terminated")
		return lexString
	default:
		l.errorf("unknown escape sequence %#U", ch)
		return lexString
	}

	var x uint32
	for n > 0 {
		d := uint32(digitVal(ch))
		if d >= base {
			if ch == eof {
				l.errorf("escape sequence not terminated")
				return lexString
			}
			l.errorf("illegal character %#U in escape sequence", ch)
			return lexString
		}
		x = x*base + d
		n--

		// Don't seek after last rune.
		if n > 0 {
			ch = l.next()
		}
	}

	if x > maxVal || 0xD800 <= x && x < 0xE000 {
		l.errorf("escape sequence is an invalid Unicode code point")
	}
	return lexString
}

// digitVal returns the digit value of a rune or 16 in case the rune does not
// represent a valid digit.
func digitVal(ch rune) int {
	switch {
	case '0' <= ch && ch <= '9':
		return int(ch - '0')
	case 'a' <= ch && ch <= 'f':
		return int(ch - 'a' + 10)
	case 'A' <= ch && ch <= 'F':
		return int(ch - 'A' + 10)
	}
	return 16 // Larger than any legal digit val.
}

// skipSpaces skips the spaces until a non-space is encountered.
func skipSpaces(l *Lexer) {
	for isSpace(l.peek()) {
		l.next()
	}
	l.ignore()
}

// lexString scans a quoted string. The initial quote has already been seen.
func lexString(l *Lexer) stateFn {
Loop:
	for {
		switch l.next() {
		case '\\':
			return lexEscape
		case utf8.RuneError:
			l.errorf("invalid UTF-8 rune")
			return lexString
		case eof, '\n':
			return l.errorf("unterminated quoted string")
		case l.stringOpen:
			break Loop
		}
	}
	l.emit(STRING)
	return lexStatements
}

// lexRawString scans a raw quoted string. The initial quote has already been seen.
func lexRawString(l *Lexer) stateFn {
Loop:
	for {
		switch l.next() {
		case utf8.RuneError:
			l.errorf("invalid UTF-8 rune")
			return lexRawString
		case eof:
			l.errorf("unterminated raw string")
			return lexRawString
		case l.stringOpen:
			break Loop
		}
	}
	l.emit(STRING)
	return lexStatements
}

// lexSpace scans a run of space characters. One space has already been seen.
func lexSpace(l *Lexer) stateFn {
	for isSpace(l.peek()) {
		l.next()
	}
	l.ignore()
	return lexStatements
}

// lexLineComment scans a line comment. Left comment marker is known to be present.
func lexLineComment(l *Lexer) stateFn {
	l.pos += posrange.Pos(len(lineComment))
	for r := l.next(); !isEndOfLine(r) && r != eof; {
		r = l.next()
	}
	l.backup()
	l.emit(COMMENT)
	return lexStatements
}

// lexNumber scans a number: decimal, hex, oct or float.
func lexNumber(l *Lexer) stateFn {
	if !l.scanNumber() {
		return l.errorf("bad number syntax: %q", l.input[l.start:l.pos])
	}
	l.emit(NUMBER)
	return lexStatements
}

func (l *Lexer) scanDurationKeyword() bool {
	for {
		switch r := l.next(); {
		case isAlpha(r):
			// absorb.
		default:
			l.backup()
			word := l.input[l.start:l.pos]
			kw := strings.ToLower(word)
			switch kw {
			case "step":
				l.emit(STEP)
				return true
			case "min":
				l.emit(MIN)
				return true
			case "max":
				l.emit(MAX)
				return true
			default:
				return false
			}
		}
	}
}

// lexNumberOrDuration scans a number or a duration Item.
func lexNumberOrDuration(l *Lexer) stateFn {
	if l.scanNumber() {
		l.emit(NUMBER)
		return lexStatements
	}
	// Next two chars must be a valid unit and a non-alphanumeric.
	if acceptRemainingDuration(l) {
		l.backup()
		l.emit(DURATION)
		return lexStatements
	}
	return l.errorf("bad number or duration syntax: %q", l.input[l.start:l.pos])
}

func acceptRemainingDuration(l *Lexer) bool {
	// Next two char must be a valid duration.
	if !l.accept("smhdwy") {
		return false
	}
	// Support for ms. Bad units like hs, ys will be caught when we actually
	// parse the duration.
	l.accept("s")
	// Next char can be another number then a unit.
	for l.accept("0123456789") {
		for l.accept("0123456789") {
		}
		// y is no longer in the list as it should always come first in
		// durations.
		if !l.accept("smhdw") {
			return false
		}
		// Support for ms. Bad units like hs, ys will be caught when we actually
		// parse the duration.
		l.accept("s")
	}
	return !isAlphaNumeric(l.next())
}

// scanNumber scans numbers of different formats. The scanned Item is
// not necessarily a valid number. This case is caught by the parser.
func (l *Lexer) scanNumber() bool {
	initialPos := l.pos
	// Modify the digit pattern if the number is hexadecimal.
	digitPattern := "0123456789"
	// Disallow hexadecimal in series descriptions as the syntax is ambiguous.
	if !l.seriesDesc &&
		l.accept("0") && l.accept("xX") {
		l.accept("_") // eg., 0X_1FFFP-16 == 0.1249847412109375
		digitPattern = "0123456789abcdefABCDEF"
	}
	const (
		// Define dot, exponent, and underscore patterns.
		dotPattern        = "."
		exponentPattern   = "eE"
		underscorePattern = "_"
		// Anti-patterns are rune sets that cannot follow their respective rune.
		dotAntiPattern        = "_."
		exponentAntiPattern   = "._eE" // and EOL.
		underscoreAntiPattern = "._eE" // and EOL.
	)
	// All numbers follow the prefix: [.][d][d._eE]*
	l.accept(dotPattern)
	l.accept(digitPattern)
	// [d._eE]* hereon.
	dotConsumed := false
	exponentConsumed := false
	for l.is(digitPattern + dotPattern + underscorePattern + exponentPattern) {
		// "." cannot repeat.
		if l.is(dotPattern) {
			if dotConsumed {
				l.accept(dotPattern)
				return false
			}
		}
		// "eE" cannot repeat.
		if l.is(exponentPattern) {
			if exponentConsumed {
				l.accept(exponentPattern)
				return false
			}
		}
		// Handle dots.
		if l.accept(dotPattern) {
			dotConsumed = true
			if l.accept(dotAntiPattern) {
				return false
			}
			// Fractional hexadecimal literals are not allowed.
			if len(digitPattern) > 10 /* 0x[\da-fA-F].[\d]+p[\d] */ {
				return false
			}
			continue
		}
		// Handle exponents.
		if l.accept(exponentPattern) {
			exponentConsumed = true
			l.accept("+-")
			if l.accept(exponentAntiPattern) || l.peek() == eof {
				return false
			}
			continue
		}
		// Handle underscores.
		if l.accept(underscorePattern) {
			if l.accept(underscoreAntiPattern) || l.peek() == eof {
				return false
			}

			continue
		}
		// Handle digits at the end since we already consumed before this loop.
		l.acceptRun(digitPattern)
	}
	// Empty string is not a valid number.
	if l.pos == initialPos {
		return false
	}
	// Next thing must not be alphanumeric unless it's the times token
	// for series repetitions.
	if r := l.peek(); (l.seriesDesc && r == 'x') || !isAlphaNumeric(r) {
		return true
	}
	return false
}

// lexIdentifier scans an alphanumeric identifier. The next character
// is known to be a letter.
func lexIdentifier(l *Lexer) stateFn {
	for isAlphaNumeric(l.next()) {
		// absorb
	}
	l.backup()
	l.emit(IDENTIFIER)
	return lexStatements
}

// lexKeywordOrIdentifier scans an alphanumeric identifier which may contain
// a colon rune. If the identifier is a keyword the respective keyword Item
// is scanned.
func lexKeywordOrIdentifier(l *Lexer) stateFn {
Loop:
	for {
		switch r := l.next(); {
		case isAlphaNumeric(r) || r == ':':
			// absorb.
		default:
			l.backup()
			word := l.input[l.start:l.pos]
			switch kw, ok := key[strings.ToLower(word)]; {
			case ok:
				l.emit(kw)
			case !strings.Contains(word, ":"):
				l.emit(IDENTIFIER)
			default:
				l.emit(METRIC_IDENTIFIER)
			}
			break Loop
		}
	}
	if l.seriesDesc && l.peek() != '{' {
		return lexValueSequence
	}
	return lexStatements
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// isEndOfLine reports whether r is an end-of-line character.
func isEndOfLine(r rune) bool {
	return r == '\r' || r == '\n'
}

// isAlphaNumeric reports whether r is an alphabetic, digit, or underscore.
func isAlphaNumeric(r rune) bool {
	return isAlpha(r) || isDigit(r)
}

// isDigit reports whether r is a digit. Note: we cannot use unicode.IsDigit()
// instead because that also classifies non-Latin digits as digits. See
// https://github.com/prometheus/prometheus/issues/939.
func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

// isAlpha reports whether r is an alphabetic or underscore.
func isAlpha(r rune) bool {
	return r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}

// lexDurationExpr scans arithmetic expressions within brackets for duration expressions.
func lexDurationExpr(l *Lexer) stateFn {
	switch r := l.next(); {
	case r == eof:
		return l.errorf("unexpected end of input in duration expression")
	case r == ']':
		l.emit(RIGHT_BRACKET)
		l.bracketOpen = false
		l.gotColon = false
		return lexStatements
	case r == ':':
		l.emit(COLON)
		if !l.gotDuration {
			return l.errorf("unexpected colon before duration in duration expression")
		}
		if l.gotColon {
			return l.errorf("unexpected repeated colon in duration expression")
		}
		l.gotColon = true
		return lexDurationExpr
	case r == '(':
		l.emit(LEFT_PAREN)
		l.parenDepth++
		return lexDurationExpr
	case r == ')':
		l.emit(RIGHT_PAREN)
		l.parenDepth--
		if l.parenDepth < 0 {
			return l.errorf("unexpected right parenthesis %q", r)
		}
		return lexDurationExpr
	case isSpace(r):
		skipSpaces(l)
		return lexDurationExpr
	case r == '+':
		l.emit(ADD)
		return lexDurationExpr
	case r == '-':
		l.emit(SUB)
		return lexDurationExpr
	case r == '*':
		l.emit(MUL)
		return lexDurationExpr
	case r == '/':
		l.emit(DIV)
		return lexDurationExpr
	case r == '%':
		l.emit(MOD)
		return lexDurationExpr
	case r == '^':
		l.emit(POW)
		return lexDurationExpr
	case r == ',':
		l.emit(COMMA)
		return lexDurationExpr
	case r == 's' || r == 'S' || r == 'm' || r == 'M':
		if l.scanDurationKeyword() {
			return lexDurationExpr
		}
		return l.errorf("unexpected character in duration expression: %q", r)
	case isDigit(r) || (r == '.' && isDigit(l.peek())):
		l.backup()
		l.gotDuration = true
		return lexNumberOrDuration
	default:
		return l.errorf("unexpected character in duration expression: %q", r)
	}
}
