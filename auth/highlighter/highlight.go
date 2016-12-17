// Package syntaxhighlight provides syntax highlighting for code. It currently
// uses a language-independent lexer and performs decently on JavaScript, Java,
// Ruby, Python, Go, and C.
package highlighter

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"text/scanner"
	"unicode"
	"unicode/utf8"
)

//go:generate gostringer -type=Kind

type spanPair struct {
	Class string
	Text  string
}

func tokenClass(tok rune, tokText string) string {
	switch tok {
	case scanner.Ident:
		if _, isKW := keywords[tokText]; isKW {
			return "kwd"
		}
		if r, _ := utf8.DecodeRuneInString(tokText); unicode.IsUpper(r) {
			return "typ"
		}
		return "pln"
	case scanner.Float, scanner.Int:
		return "dec"
	case scanner.Char, scanner.String, scanner.RawString:
		return "str"
	case scanner.Comment:
		return "com"
	}
	if unicode.IsSpace(tok) {
		return ""
	}

	return "pun"
}

// DefaultHTMLConfig's class names match those of google-code-prettify
// (https://code.google.com/p/google-code-prettify/).
var _ = fmt.Sprintln
var reSpace = regexp.MustCompile(`(\r|\s|\t)`)
var reURL = regexp.MustCompile(`(http[s]?(?:.*?):(?:.*?)\/\/(?:[a-zA-Z]|[0-9]|[$-_@.&+]|[!*,]|(?:%[0-9a-fA-F][0-9a-fA-F]))+?)(?:[\(\)\[\]"'\>\<\s\n\r\t]|$)`)
var tabs = "&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;"

func joinStack(spans []spanPair, zebra *bool, linenum *int, tabspace int) string {
	cons := bytes.Buffer{}
	for _, sp := range spans {
		text := reSpace.ReplaceAllStringFunc(sp.Text, func(in string) string {
			switch in {
			case "\t":
				if tabspace <= 8 {
					return tabs[0 : tabspace*6]
				} else {
					return tabs
				}
			default:
				return "&nbsp;"
			}
		})

		re := []string{}
		if matches := reURL.FindAllStringSubmatch(html.UnescapeString(text), -1); len(matches) > 0 {
			for _, m := range matches {
				re = append(re, m[1], "<a href='"+m[1]+"'>"+m[1]+"</a>")
			}
		}

		text = strings.NewReplacer(re...).Replace(text)

		if text != "" {
			if sp.Class != "" {
				cons.WriteString("<span class='" + sp.Class + "'>")
				cons.WriteString(text)
				cons.WriteString("</span>")
			} else {
				cons.WriteString(text)
			}
		}
	}

	ret := bytes.Buffer{}

	if *zebra = !*zebra; *zebra {
		ret.WriteString("<tr class='li1'>")
	} else {
		ret.WriteString("<tr class='li0'>")
	}

	*linenum++
	ln := strconv.Itoa(*linenum)
	ret.WriteString("<td class='lin' id='line-" + ln + "'>" + ln + "</td><td>")

	if cons.String() == "" {
		if *linenum == 1 {
			// Line 1 is empty, omitted
			*linenum = 0
			return ""
		} else {
			ret.WriteString("&nbsp;")
		}
	} else {
		ret.WriteString(cons.String())
	}
	ret.WriteString("</td></tr>")

	return ret.String()
}

func Highlight(src []byte, tabspace int) string {
	if len(src) > 3 && string(src)[0:3] == "```" {
		return Plain(string(src)[3:], tabspace)
	} else {
		s := &scanner.Scanner{}
		s.Init(bytes.NewReader(src))
		s.Error = func(_ *scanner.Scanner, _ string) {}
		s.Whitespace = 0
		s.Mode = s.Mode ^ scanner.SkipComments
		return Normal(s, tabspace)
	}
}

func Plain(text string, tabspace int) string {
	final := bytes.Buffer{}
	zebra := true
	linenum := 0
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		final.WriteString(joinStack([]spanPair{
			spanPair{
				Class: "pln",
				Text:  html.EscapeString(line),
			},
		}, &zebra, &linenum, tabspace))
	}

	return final.String()
}

func Normal(s *scanner.Scanner, tabspace int) string {
	tok := s.Scan()
	stack := make([]spanPair, 0)
	final := bytes.Buffer{}
	zebra := true
	linenum := 0

	for tok != scanner.EOF {
		tokText := s.TokenText()
		pairs := make([]spanPair, 0)
		lines := strings.Split(tokText, "\n")
		class := tokenClass(tok, tokText)

		for _, line := range lines {
			pairs = append(pairs, spanPair{Class: class, Text: html.EscapeString(line)})
		}

		if len(pairs) > 0 {
			stack = append(stack, pairs[0])
		}

		if len(lines) > 1 {
			final.WriteString(joinStack(stack, &zebra, &linenum, tabspace))

			for i := 1; i < len(pairs)-1; i++ {
				final.WriteString(joinStack([]spanPair{pairs[i]}, &zebra, &linenum, tabspace))
			}

			if len(pairs) > 1 {
				stack = []spanPair{pairs[len(pairs)-1]}
			} else {
				stack = []spanPair{}
			}
		}

		tok = s.Scan()
	}

	if len(stack) > 0 {
		final.WriteString(joinStack(stack, &zebra, &linenum, tabspace))
	}

	ret := final.String()

	if strings.HasSuffix(ret, "<td>&nbsp;</td></tr>") {
		return ret[:strings.LastIndex(ret, "<tr")]
	} else {
		return ret
	}
}
