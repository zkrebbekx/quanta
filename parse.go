package quanta

import (
	"errors"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxExponent bounds the exponent accepted in a unit expression. It is far
// larger than any dimension can hold, and exists so that a hostile input such
// as "m^999999999" cannot make the parser compute an astronomically large
// rational factor.
const maxExponent = 1000

// maxDepth bounds parenthesis nesting, so that deeply nested input cannot
// exhaust the goroutine stack.
const maxDepth = 64

type tokKind int

const (
	tkEOF tokKind = iota
	tkIdent
	tkInt
	tkSup // a run of Unicode superscript characters, already decoded
	tkMul
	tkDiv
	tkPow
	tkMinus
	tkPlus
	tkLParen
	tkRParen
)

type token struct {
	kind tokKind
	text string
	num  int
	pos  int
}

// isIdentRune reports whether r may appear in a unit symbol. Letters cover the
// bulk of it; the extras admit symbols such as °C, Ω, Δ°C, % and currency
// codes written with a currency sign.
func isIdentRune(r rune) bool {
	if unicode.IsLetter(r) || unicode.In(r, unicode.Sc) {
		return true
	}
	switch r {
	case '°', '\'', '"', '_', '%':
		return true
	}
	return false
}

// superscriptValue decodes a Unicode superscript digit or sign.
func superscriptValue(r rune) (digit int, sign int, ok bool) {
	switch r {
	case '⁻':
		return 0, -1, true
	case '⁺':
		return 0, 1, true
	case '⁰', '¹', '²', '³', '⁴', '⁵', '⁶', '⁷', '⁸', '⁹':
	default:
		return 0, 0, false
	}
	for i, d := range superscriptDigits {
		if string(r) == d {
			return i, 0, true
		}
	}
	return 0, 0, false
}

// lex splits a unit expression into tokens, recording each token's byte offset.
func lex(s string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(s) {
		r, w := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && w <= 1:
			return nil, parseErr(s, i, ErrSyntax, "invalid UTF-8")

		case unicode.IsSpace(r):
			i += w

		case r == '(':
			toks = append(toks, token{kind: tkLParen, pos: i})
			i += w
		case r == ')':
			toks = append(toks, token{kind: tkRParen, pos: i})
			i += w
		case r == '/' || r == '÷':
			toks = append(toks, token{kind: tkDiv, pos: i})
			i += w
		case r == '·' || r == '⋅' || r == '×' || r == '.':
			toks = append(toks, token{kind: tkMul, pos: i})
			i += w
		case r == '*':
			start := i
			i += w
			if i < len(s) && s[i] == '*' { // "**" is exponentiation
				i++
				toks = append(toks, token{kind: tkPow, pos: start})
			} else {
				toks = append(toks, token{kind: tkMul, pos: start})
			}
		case r == '^':
			toks = append(toks, token{kind: tkPow, pos: i})
			i += w
		case r == '-' || r == '−':
			toks = append(toks, token{kind: tkMinus, pos: i})
			i += w
		case r == '+':
			toks = append(toks, token{kind: tkPlus, pos: i})
			i += w

		case r >= '0' && r <= '9':
			start := i
			for i < len(s) && s[i] >= '0' && s[i] <= '9' {
				i++
			}
			n, err := strconv.Atoi(s[start:i])
			if err != nil || n > maxExponent {
				return nil, parseErr(s, start, ErrExponentRange,
					"integer %q out of range", s[start:i])
			}
			toks = append(toks, token{kind: tkInt, num: n, text: s[start:i], pos: start})

		default:
			if d, sign, ok := superscriptValue(r); ok {
				start := i
				value, mul, seen := d, 1, sign == 0
				if sign != 0 {
					mul = sign
					value = 0
				}
				i += w
				for i < len(s) {
					r2, w2 := utf8.DecodeRuneInString(s[i:])
					d2, s2, ok2 := superscriptValue(r2)
					if !ok2 || s2 != 0 {
						break
					}
					value = value*10 + d2
					seen = true
					i += w2
				}
				if !seen {
					return nil, parseErr(s, start, ErrSyntax,
						"superscript sign with no digits")
				}
				if value > maxExponent {
					return nil, parseErr(s, start, ErrExponentRange,
						"exponent %d out of range", value)
				}
				toks = append(toks, token{kind: tkSup, num: value * mul, pos: start})
				continue
			}
			if isIdentRune(r) {
				start := i
				for i < len(s) {
					r2, w2 := utf8.DecodeRuneInString(s[i:])
					if !isIdentRune(r2) {
						break
					}
					i += w2
				}
				toks = append(toks, token{kind: tkIdent, text: s[start:i], pos: start})
				continue
			}
			return nil, parseErr(s, i, ErrSyntax, "unexpected character %q", string(r))
		}
	}
	toks = append(toks, token{kind: tkEOF, pos: len(s)})
	return toks, nil
}

// parser is a recursive-descent parser over a token stream.
type parser struct {
	reg   *Registry
	input string
	toks  []token
	i     int
	depth int
}

func (p *parser) peek() token { return p.toks[p.i] }
func (p *parser) next() token { t := p.toks[p.i]; p.i++; return t }

func (p *parser) errAt(pos int, sentinel error, format string, args ...any) error {
	return parseErr(p.input, pos, sentinel, format, args...)
}

// wrap turns an error from unit algebra into a positioned parse error,
// preserving the sentinel so errors.Is still matches.
func (p *parser) wrap(pos int, err error) error {
	switch {
	case errors.Is(err, ErrAffineUnit):
		return p.errAt(pos, ErrAffineUnit,
			"an affine unit cannot appear in a compound unit expression")
	case errors.Is(err, ErrExponentRange):
		return p.errAt(pos, ErrExponentRange, "dimension exponent out of range")
	default:
		return p.errAt(pos, ErrSyntax, "%s", err)
	}
}

// parseExpr parses a sequence of terms combined by multiplication and
// division. Both operators bind equally and associate to the left, so
// "a/b/c" means "(a/b)/c" — that is, a·b⁻¹·c⁻¹.
func (p *parser) parseExpr() (Unit, error) {
	u, err := p.parseTerm()
	if err != nil {
		return Unit{}, err
	}
	for {
		t := p.peek()
		sign := 0
		switch t.kind {
		case tkMul:
			p.next()
			sign = 1
		case tkDiv:
			p.next()
			sign = -1
		case tkIdent, tkInt, tkLParen:
			// Juxtaposition (usually a space) is multiplication.
			sign = 1
		default:
			return u, nil
		}
		rhs, err := p.parseTerm()
		if err != nil {
			return Unit{}, err
		}
		out, err := combine(u, rhs, sign)
		if err != nil {
			return Unit{}, p.wrap(t.pos, err)
		}
		u = out
	}
}

// parseTerm parses an atom with an optional exponent.
func (p *parser) parseTerm() (Unit, error) {
	u, err := p.parseAtom()
	if err != nil {
		return Unit{}, err
	}
	t := p.peek()
	var exp int
	switch t.kind {
	case tkPow:
		p.next()
		exp, err = p.parseExponent()
		if err != nil {
			return Unit{}, err
		}
	case tkSup:
		p.next()
		exp = t.num
	default:
		return u, nil
	}
	if exp == 1 {
		return u, nil
	}
	out, err := powUnit(u, exp)
	if err != nil {
		return Unit{}, p.wrap(t.pos, err)
	}
	return out, nil
}

// parseExponent reads the signed integer that follows "^" or "**".
func (p *parser) parseExponent() (int, error) {
	sign := 1
	switch p.peek().kind {
	case tkMinus:
		p.next()
		sign = -1
	case tkPlus:
		p.next()
	}
	t := p.peek()
	switch t.kind {
	case tkInt:
		p.next()
		return sign * t.num, nil
	case tkSup:
		p.next()
		return sign * t.num, nil
	default:
		return 0, p.errAt(t.pos, ErrSyntax, "expected an integer exponent")
	}
}

// parseAtom parses a unit symbol, a parenthesised expression, or the literal 1.
func (p *parser) parseAtom() (Unit, error) {
	t := p.next()
	switch t.kind {
	case tkIdent:
		u, ok := p.reg.resolve(t.text)
		if !ok {
			return Unit{}, p.errAt(t.pos, ErrUnknownUnit, "unknown unit %q", t.text)
		}
		return u, nil

	case tkLParen:
		if p.depth >= maxDepth {
			return Unit{}, p.errAt(t.pos, ErrSyntax, "expression nested too deeply")
		}
		p.depth++
		u, err := p.parseExpr()
		p.depth--
		if err != nil {
			return Unit{}, err
		}
		if got := p.next(); got.kind != tkRParen {
			return Unit{}, p.errAt(got.pos, ErrSyntax, "expected %q", ")")
		}
		return u, nil

	case tkInt:
		if t.num != 1 {
			return Unit{}, p.errAt(t.pos, ErrSyntax,
				"only the numeral 1 may appear in a unit expression, not %d", t.num)
		}
		return One, nil

	case tkEOF:
		return Unit{}, p.errAt(t.pos, ErrSyntax, "unexpected end of input")

	default:
		return Unit{}, p.errAt(t.pos, ErrSyntax, "unexpected %q", tokenText(t, p.input))
	}
}

// tokenText renders a token for an error message.
func tokenText(t token, input string) string {
	if t.text != "" {
		return t.text
	}
	if t.pos < len(input) {
		r, _ := utf8.DecodeRuneInString(input[t.pos:])
		return string(r)
	}
	return "end of input"
}

// ParseUnit parses a compound unit expression against the registry.
//
// The grammar accepts unit symbols, SI and binary prefixes, parentheses,
// multiplication (written "·", "*", "×", "." or simply a space), division
// ("/"), and integer exponents written "^2", "**2" or with Unicode
// superscripts ("²"). Exponents may be negative, and the numeral 1 may stand
// alone as a numerator, so "1/s", "s^-1" and "s⁻¹" all denote the hertz.
//
// Multiplication and division bind equally and associate to the left:
// "J/kg·K" parses as "(J/kg)·K", which is probably not what was meant — write
// "J/(kg·K)". This is stated rather than guessed at, because unit strings in
// the wild disagree with each other.
//
// An exact registered symbol always wins over a prefixed reading: "min" is the
// minute, not a milli-inch, and "cd" is the candela, not a centi-day.
//
// Affine units such as °C may be parsed alone but not as part of a compound
// expression; that returns an error wrapping [ErrAffineUnit]. An empty or
// blank expression denotes the dimensionless unit one.
//
// Errors are always a [*ParseError] carrying the input and the byte offset of
// the problem, wrapping [ErrSyntax], [ErrUnknownUnit], [ErrAffineUnit] or
// [ErrExponentRange].
func (r *Registry) ParseUnit(s string) (Unit, error) {
	if strings.TrimSpace(s) == "" {
		return One, nil
	}
	toks, err := lex(s)
	if err != nil {
		return Unit{}, err
	}
	p := &parser{reg: r, input: s, toks: toks}
	u, err := p.parseExpr()
	if err != nil {
		return Unit{}, err
	}
	if t := p.peek(); t.kind != tkEOF {
		return Unit{}, p.errAt(t.pos, ErrSyntax, "unexpected %q", tokenText(t, s))
	}
	return u, nil
}

// MustParseUnit is [Registry.ParseUnit] for expressions known to be valid, such
// as package-level variables and tests. It panics on error.
func (r *Registry) MustParseUnit(s string) Unit {
	u, err := r.ParseUnit(s)
	if err != nil {
		panic(err)
	}
	return u
}

// Parse parses a quantity: a float64 magnitude followed by an optional unit
// expression, as in "9.81 m/s^2", "1.5 kW·h", "-40 °C" or "1e3" (which is the
// dimensionless 1000). The space between magnitude and unit is optional.
//
// The unit part is parsed by [Registry.ParseUnit] and obeys all of its rules.
func (r *Registry) Parse(s string) (Quantity, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return Quantity{}, parseErr(s, 0, ErrSyntax, "empty quantity")
	}
	n := numberPrefixLen(trimmed)
	if n == 0 {
		return Quantity{}, parseErr(s, strings.Index(s, trimmed), ErrSyntax,
			"expected a number at the start of the quantity")
	}
	v, err := strconv.ParseFloat(trimmed[:n], 64)
	if err != nil {
		return Quantity{}, parseErr(s, strings.Index(s, trimmed), ErrSyntax,
			"invalid number %q", trimmed[:n])
	}
	u, err := r.ParseUnit(trimmed[n:])
	if err != nil {
		var pe *ParseError
		if errors.As(err, &pe) {
			// Re-anchor the offset onto the original input.
			pe.Offset += strings.Index(s, trimmed) + n
			pe.Input = s
		}
		return Quantity{}, err
	}
	return Quantity{value: v, unit: u}, nil
}

// MustParse is [Registry.Parse] for inputs known to be valid, such as
// package-level variables and tests. It panics on error.
func (r *Registry) MustParse(s string) Quantity {
	q, err := r.Parse(s)
	if err != nil {
		panic(err)
	}
	return q
}

// numberPrefixLen returns the length of the longest prefix of s that looks like
// a float64 literal. It stops before an exponent marker that is not followed by
// digits, so that "5eV" reads as 5 electronvolts rather than a broken exponent.
func numberPrefixLen(s string) int {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return 0
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		j := i + 1
		if j < len(s) && (s[j] == '+' || s[j] == '-') {
			j++
		}
		expDigits := 0
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
			expDigits++
		}
		if expDigits > 0 {
			i = j
		}
	}
	return i
}
