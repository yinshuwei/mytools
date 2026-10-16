package tools

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// The arithmetic behind the calculator page. The parser is written out by
// hand rather than taken from a library: an expression is read, never run,
// so nothing typed into the box can reach the machine.
//
//	1+2*3        (1+2)*3        2^10        2^-1        -2^2
//	10 % 3       0xff            0b1010      0o17        1_000_000
//	1e3          sqrt(2)         min(3,1,2)  abs(-3)     pi/2

// calcEval works out an arithmetic expression.
func calcEval(src string) (float64, error) {
	p := &parser{src: src}
	v, err := p.expr()
	if err != nil {
		return 0, err
	}
	p.skip()
	if p.at < len(p.src) {
		return 0, fmt.Errorf("从 %q 起看不懂", p.src[p.at:])
	}
	return v, nil
}

// parser is a cursor over an expression.
type parser struct {
	src string
	at  int
}

// skip passes over the blanks between tokens.
func (p *parser) skip() {
	for p.at < len(p.src) && (p.src[p.at] == ' ' || p.src[p.at] == '\t') {
		p.at++
	}
}

// take passes over and returns the operator at the cursor when it is one of
// ops, and the zero byte when it is not.
func (p *parser) take(ops string) byte {
	p.skip()
	if p.at < len(p.src) && strings.IndexByte(ops, p.src[p.at]) >= 0 {
		c := p.src[p.at]
		p.at++
		return c
	}
	return 0
}

// expr reads sums: a term, then as many more as + and - join.
func (p *parser) expr() (float64, error) {
	v, err := p.term()
	if err != nil {
		return 0, err
	}
	for {
		op := p.take("+-")
		if op == 0 {
			return v, nil
		}
		n, err := p.term()
		if err != nil {
			return 0, err
		}
		if op == '+' {
			v += n
		} else {
			v -= n
		}
	}
}

// term reads products: a unary, then as many more as * / and % join.
func (p *parser) term() (float64, error) {
	v, err := p.unary()
	if err != nil {
		return 0, err
	}
	for {
		op := p.take("*/%")
		if op == 0 {
			return v, nil
		}
		n, err := p.unary()
		if err != nil {
			return 0, err
		}
		switch op {
		case '*':
			v *= n
		case '/':
			if n == 0 {
				return 0, errors.New("除以 0")
			}
			v /= n
		case '%':
			if n == 0 {
				return 0, errors.New("对 0 取余")
			}
			v = math.Mod(v, n)
		}
	}
}

// unary reads a sign, if there is one, and what it applies to: so that -2^2
// is -(2^2), as a mathematician writes it.
func (p *parser) unary() (float64, error) {
	switch p.take("+-") {
	case '-':
		v, err := p.unary()
		return -v, err
	case '+':
		return p.unary()
	}
	return p.power()
}

// power reads a value, and what it is raised to: right to left, so that
// 2^3^2 is 2^9, with the exponent taking a sign of its own (2^-1).
func (p *parser) power() (float64, error) {
	v, err := p.atom()
	if err != nil {
		return 0, err
	}
	if p.take("^") == 0 {
		return v, nil
	}
	e, err := p.unary()
	if err != nil {
		return 0, err
	}
	return math.Pow(v, e), nil
}

// atom reads one value: a parenthesized expression, a number, a constant,
// or a call.
func (p *parser) atom() (float64, error) {
	p.skip()
	if p.at >= len(p.src) {
		return 0, errors.New("算式还差一截")
	}
	c := p.src[p.at]
	switch {
	case c == '(':
		p.at++
		v, err := p.expr()
		if err != nil {
			return 0, err
		}
		if p.take(")") == 0 {
			return 0, errors.New("少了右括号")
		}
		return v, nil
	case c >= '0' && c <= '9' || c == '.':
		return p.number()
	case isNameByte(c):
		return p.name()
	}
	return 0, fmt.Errorf("看不懂 %q", string(c))
}

// number reads a number: 12, 1.5, .5, 1e3, or a whole one in another base
// as 0x1f, 0b1010 and 0o17, with _ allowed between any of the digits.
func (p *parser) number() (float64, error) {
	start := p.at
	if p.src[p.at] == '0' && p.at+1 < len(p.src) {
		switch p.src[p.at+1] {
		case 'x', 'X':
			p.at += 2
			return p.based(start, 16)
		case 'b', 'B':
			p.at += 2
			return p.based(start, 2)
		case 'o', 'O':
			p.at += 2
			return p.based(start, 8)
		}
	}
	dot, exp := false, false
scan:
	for p.at < len(p.src) {
		switch c := p.src[p.at]; {
		case c >= '0' && c <= '9' || c == '_':
			p.at++
		case c == '.' && !dot:
			dot = true
			p.at++
		case (c == 'e' || c == 'E') && !exp:
			// The sign of the exponent belongs to the number.
			exp = true
			p.at++
			if p.at < len(p.src) && (p.src[p.at] == '+' || p.src[p.at] == '-') {
				p.at++
			}
		default:
			break scan
		}
	}
	text := strings.ReplaceAll(p.src[start:p.at], "_", "")
	v, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, fmt.Errorf("%q 不是一个数", text)
	}
	return v, nil
}

// based reads a whole number in another base, the cursor standing after its
// 0x / 0b / 0o prefix.
func (p *parser) based(start, base int) (float64, error) {
	digits := 0
	for p.at < len(p.src) {
		c := p.src[p.at]
		if c == '_' {
			p.at++
			continue
		}
		if d := digitValue(c); d < 0 || d >= base {
			break
		}
		digits++
		p.at++
	}
	text := strings.ReplaceAll(p.src[start:p.at], "_", "")
	if digits == 0 {
		return 0, fmt.Errorf("%q 后面没有数字", text)
	}
	v, err := strconv.ParseUint(text[2:], base, 64)
	if err != nil {
		return 0, fmt.Errorf("%q 不是一个数", text)
	}
	return float64(v), nil
}

// name reads a constant, or the name of a call.
func (p *parser) name() (float64, error) {
	start := p.at
	for p.at < len(p.src) && isNameByte(p.src[p.at]) {
		p.at++
	}
	name := strings.ToLower(p.src[start:p.at])
	p.skip()
	if p.at < len(p.src) && p.src[p.at] == '(' {
		return p.call(name)
	}
	if v, ok := calcConsts[name]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("不认识 %q", name)
}

// call reads the arguments of a function: none, one, or a list of them.
func (p *parser) call(name string) (float64, error) {
	fn, ok := calcFuncs[name]
	if !ok {
		return 0, fmt.Errorf("没有名为 %q 的函数", name)
	}
	p.at++ // the (
	var args []float64
	if p.take(")") != 0 {
		return fn(args)
	}
	for {
		v, err := p.expr()
		if err != nil {
			return 0, err
		}
		args = append(args, v)
		if p.take(",") != 0 {
			continue
		}
		if p.take(")") != 0 {
			return fn(args)
		}
		return 0, errors.New("参数之间少了逗号，或者少了右括号")
	}
}

// isNameByte reports whether c may be part of a name of a constant or
// function.
func isNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

// digitValue is what a digit stands for, or -1 when it is not one.
func digitValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

// calcConsts are the names an expression may use as numbers.
var calcConsts = map[string]float64{
	"pi":  math.Pi,
	"e":   math.E,
	"tau": 2 * math.Pi,
}

// calcFuncs are the functions an expression may call. There are no
// trigonometric ones: a toolbox works in numbers, and a sine wants to know
// whether it is being given degrees or radians.
var calcFuncs = map[string]func([]float64) (float64, error){
	"abs":   oneNumber("abs", math.Abs),
	"sqrt":  oneNumber("sqrt", math.Sqrt),
	"cbrt":  oneNumber("cbrt", math.Cbrt),
	"round": oneNumber("round", math.Round),
	"floor": oneNumber("floor", math.Floor),
	"ceil":  oneNumber("ceil", math.Ceil),
	"trunc": oneNumber("trunc", math.Trunc),
	"exp":   oneNumber("exp", math.Exp),
	"ln":    oneNumber("ln", math.Log),
	"log":   oneNumber("log", math.Log10),
	"log2":  oneNumber("log2", math.Log2),
	"min":   someNumbers("min", math.Min),
	"max":   someNumbers("max", math.Max),
	"hypot": someNumbers("hypot", math.Hypot),
}

// oneNumber makes a call of exactly one number.
func oneNumber(name string, f func(float64) float64) func([]float64) (float64, error) {
	return func(args []float64) (float64, error) {
		if len(args) != 1 {
			return 0, fmt.Errorf("%s 要 1 个数，给了 %d 个", name, len(args))
		}
		return f(args[0]), nil
	}
}

// someNumbers makes a call of two or more numbers, folded left to right, as
// min and max take as many as they are given.
func someNumbers(name string, f func(a, b float64) float64) func([]float64) (float64, error) {
	return func(args []float64) (float64, error) {
		if len(args) < 2 {
			return 0, fmt.Errorf("%s 至少要 2 个数", name)
		}
		v := args[0]
		for _, x := range args[1:] {
			v = f(v, x)
		}
		return v, nil
	}
}

// formatNumber writes a number the way a calculator does: a whole one
// without a decimal point, any other as short as it can be written.
func formatNumber(v float64) string {
	switch {
	case math.IsNaN(v):
		return "不是一个数"
	case math.IsInf(v, 1):
		return "∞"
	case math.IsInf(v, -1):
		return "-∞"
	case v == math.Trunc(v) && math.Abs(v) < 1e15:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// calcBases writes a whole number in hex and binary as well, which is half
// of what one reaches for a calculator for. It says nothing for a number
// that has no such form, or one too large to hold exactly.
func calcBases(v float64) string {
	if v != math.Trunc(v) || math.Abs(v) >= 1<<53 {
		return ""
	}
	n := int64(v)
	out := fmt.Sprintf("0x%x", n)
	if v >= math.MinInt32 && v <= math.MaxUint32 {
		out += fmt.Sprintf(" · 0b%b", n)
	}
	return out
}
