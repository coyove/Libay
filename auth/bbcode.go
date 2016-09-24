package auth

import (
	"../conf"
	"./highlighter"

	"bytes"
	"encoding/csv"
	"fmt"
	_html "golang.org/x/net/html"
	"html"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// [color=#ff0000]...[/color]
// [size=20%]...[/size]
// [:)]
// [list][*]...[*]...[/list]
//   [ul][li]{text}[/li][/ul]
// [table][tr][td]...[/td][/tr][/table]

// [XXX]
// [XXX=YYY]
// [XXX YYY=ZZZ UUU=VVV]
// [/XXX]

var (
	reBBCodeTokens = regexp.MustCompile(`(?Ui)\[(?:(/?)([a-z|\*]+)|([A-Za-z]+)=([^\]\s]+))\s*\]`)
)

type ErrUnknownTag string

func (e ErrUnknownTag) Error() string {
	return fmt.Sprintf("bbcode: unknown tag '%s'", string(e))
}

type ErrInvalidUrl string

func (e ErrInvalidUrl) Error() string {
	return fmt.Sprintf("bbcode: invalid url '%s'", string(e))
}

type ErrIncompleteTag string

func (e ErrIncompleteTag) Error() string {
	return fmt.Sprintf("bbcode: incomplete tag '%s'", string(e))
}

type ErrCannotCrossLine string

func (e ErrCannotCrossLine) Error() string {
	return fmt.Sprintf("bbcode: tag '%s' cannot cross line", string(e))
}

type Token struct {
	Text string

	Tag   string
	End   bool
	Value string

	stackTokens []*Token
}

type Tokenizer struct {
	bbcode      string
	hits        [][]int
	checkpoints []int
	index       int

	lastToken *Token
	zebra     bool
}

func TokenizeString(bbcode string, maxTags int) *Tokenizer {
	hits := reBBCodeTokens.FindAllStringSubmatchIndex(bbcode, maxTags)
	inCode := ""
	h := 0
	for h < len(hits) {
		tag := bbcode[hits[h][0]:hits[h][1]]
		_tag := strings.ToLower(tag)

		if _tag == "[/code]" || _tag == "[/html]" || _tag == "[/csv]" {
			if _tag[2:len(_tag)-1] == inCode {
				inCode = ""
			}
		}

		if inCode != "" {
			hits = append(hits[:h], hits[h+1:]...)
			continue
		}

		if _tag == "[code]" || _tag == "[html]" || _tag == "[csv]" {
			inCode = _tag[1 : len(_tag)-1]
		}

		h++
	}

	return &Tokenizer{
		bbcode:      bbcode,
		hits:        hits,
		checkpoints: make([]int, 0),
	}
}

func (t *Tokenizer) Begin() {
	t.checkpoints = append(t.checkpoints, t.index)
}

func (t *Tokenizer) Commit() {
	t.checkpoints = t.checkpoints[:len(t.checkpoints)-1]
}

func (t *Tokenizer) Rollback() {
	t.index = t.checkpoints[len(t.checkpoints)-1]
	t.Commit()
}

func (t *Tokenizer) Next() *Token {
	for t.index < len(t.hits)*2+1 {
		i := t.index / 2
		t.index++

		var idx []int
		if i < len(t.hits) {
			idx = t.hits[i]
		} else {
			idx = []int{len(t.bbcode), -1}
		}
		if t.index&1 == 1 {
			// text
			o := 0
			if i > 0 {
				o = t.hits[i-1][1]
			}
			txt := t.bbcode[o:idx[0]]
			if txt != "" {
				txt = strings.Replace(txt, "\r", "", -1)
				if t.lastToken != nil {
					return &Token{
						Text:        txt,
						stackTokens: append([]*Token{t.lastToken}, t.lastToken.stackTokens...),
					}
				} else {
					return &Token{Text: txt, stackTokens: make([]*Token, 0)}
				}
			}
		} else {
			tok := Token{stackTokens: make([]*Token, 0)}
			tok.Text = t.bbcode[idx[0]:idx[1]]
			if idx[2] >= 0 {
				// [tag] or [/tag]
				tok.Tag = strings.ToLower(t.bbcode[idx[4]:idx[5]])
				if tok.Tag == "hr" {
					// hr doesn't have an ending tag
					if t.lastToken != nil {
						tok.stackTokens = append(tok.stackTokens, t.lastToken)
						tok.stackTokens = append(tok.stackTokens, t.lastToken.stackTokens...)
					}
				} else {
					if idx[2] == idx[3]-1 {
						tok.End = true
						tok.stackTokens = t.lastToken.stackTokens
						if len(tok.stackTokens) > 0 {
							t.lastToken = tok.stackTokens[0]
						} else {
							t.lastToken = nil
						}
					} else {
						if t.lastToken != nil {
							tok.stackTokens = append(tok.stackTokens, t.lastToken)
							tok.stackTokens = append(tok.stackTokens, t.lastToken.stackTokens...)
						}

						t.lastToken = &tok
					}
				}

			} else {
				// [tag=value]
				tok.Tag = strings.ToLower(t.bbcode[idx[6]:idx[7]])
				tok.Value = t.bbcode[idx[8]:idx[9]]

				if t.lastToken != nil {
					tok.stackTokens = append(tok.stackTokens, t.lastToken)
					tok.stackTokens = append(tok.stackTokens, t.lastToken.stackTokens...)
				}

				t.lastToken = &tok
			}

			return &tok
		}
	}
	return nil
}

func (t *Tokenizer) Zebra() string {
	t.zebra = !t.zebra
	return strconv.FormatBool(t.zebra)
}

func buildHTMLTag(t *Token) (string, string, error) {
	end, start := "", ""

	for i := 0; i < len(t.stackTokens); i++ {
		if html := translate(t.stackTokens[i], true); html == "" {
			return "", "", ErrCannotCrossLine(t.stackTokens[i].Tag)
		} else {
			end += html
		}
	}

	for i := len(t.stackTokens) - 1; i >= 0; i-- {
		if html := translate(t.stackTokens[i], false); html == "" {
			return "", "", ErrCannotCrossLine(t.stackTokens[i].Tag)
		} else {
			start += html
		}
	}

	return start, end, nil
}

func translate(t *Token, end bool) string {
	switch t.Tag {
	case "b":
		if end || t.End {
			return "</strong>"
		} else {
			return "<strong>"
		}
	case "i":
		if end || t.End {
			return "</em>"
		} else {
			return "<em>"
		}
	case "u":
		if end || t.End {
			return "</u>"
		} else {
			return "<u>"
		}
	case "s":
		if end || t.End {
			return "</del>"
		} else {
			return "<del>"
		}
	case "size":
		if end || t.End {
			return "</span>"
		} else {
			return "<span style='font-size:" + t.Value + "'>"
		}
	case "color":
		if end || t.End {
			return "</span>"
		} else {
			return "<span style='color:" + t.Value + "'>"
		}
	case "bgcolor":
		if end || t.End {
			return "</span>"
		} else {
			return "<span style='background-color:" + t.Value + "; padding: 2px'>"
		}
	case "quote":
		if end || t.End {
			return "</blockquote>"
		} else {
			return "<blockquote>"
		}
	case "center", "left", "right":
		if end || t.End {
			return "</span>"
		} else {
			return "<div class='align' style='text-align:" + t.Tag + "'>"
		}
	case "header":
		if end || t.End {
			return "</h2>"
		} else {
			return "<h2>"
		}
	}

	return ""
}

func buildTable(table [][]string) string {
	tableHTML := &bytes.Buffer{}
	maxCols := 1
	flag := false
	for row, t := range table {
		if len(t) > 0 {
			maxCols = len(t)
			tag := "td"
			if row == 0 {
				tag = "th"
			}

			flag = !flag
			tableHTML.WriteString("<tr class='csv-" + strconv.FormatBool(flag) + "'>")
			for _, c := range t {
				tableHTML.WriteString("<" + tag + ">" + html.EscapeString(c) + "</" + tag + ">")
			}
			tableHTML.WriteString("</tr>")
		} else {
			if row < len(table)-1 && len(table[row+1]) == 0 {
				tableHTML.WriteString("<tr><td class=table-sep colspan=" + strconv.Itoa(maxCols) + "></td></tr>")
			}
		}
	}

	return "<table class='csv'>" + tableHTML.String() + "</table>"
}

func tokensToHTML(tok *Tokenizer) ([]string, []error) {
	bits := make([]string, 0, 32)
	var errors []error = nil
	inLink := false

	for t := tok.Next(); t != nil; t = tok.Next() {
		// fmt.Println(t.Text)
		if t.Tag == "" {
			lines := strings.Split(t.Text, "\n")
			if len(lines) == 1 {
				bits = append(bits, html.EscapeString(t.Text))
			} else if t.Text == "\n" {
				bits = append(bits, "</td></tr><tr class='zebra-"+tok.Zebra()+"'><td>")
			} else {
				start, end, err := buildHTMLTag(t)
				if err != nil {
					errors = append(errors, err)
					break
				}

				bits = append(bits, html.EscapeString(lines[0])+end+
					"</td></tr><tr class='zebra-"+tok.Zebra()+"'><td>")

				for i := 1; i < len(lines)-1; i++ {
					text := html.EscapeString(lines[i])
					if text == "" {
						text = "&nbsp;"
					}
					bits = append(bits, start+text+end+"</td></tr><tr class='zebra-"+tok.Zebra()+"'><td>")
				}

				bits = append(bits, start+html.EscapeString(lines[len(lines)-1]))
			}
		} else {
			switch t.Tag {
			case "b", "i", "u", "s", "size", "color", "bgcolor", "quote", "center", "left", "right", "header":
				bits = append(bits, translate(t, t.End))
			case "hr":
				if !t.End {
					bits = append(bits, "<hr>")
				}
			case "url":
				if t.End {
					if inLink {
						bits = append(bits, "</a>")
					} else {
						errors = append(errors, ErrIncompleteTag("url"))
					}
				} else {
					url, target, flag := t.Value, "", false

					if url == "" {
						t = tok.Next()
						if t == nil || t.Tag != "" {
							errors = append(errors, ErrIncompleteTag("url"))
							break
						}
						url = t.Text
						flag = true
					}

					if strings.HasPrefix(url, "_blank;") {
						target = "_blank"
						url = url[7:]
					}

					inLink = true
					escapedUrl := html.EscapeString(url)
					bits = append(bits, "<a class='link' href='", escapedUrl, "' target='", target, "'>")
					if flag {
						bits = append(bits, t.Text)
					}
				}
			case "img":
				if !t.End {
					tok.Begin()
					style := ""
					if t.Value != "" {
						style = html.EscapeString(t.Value)
					}

					t = tok.Next()
					if t == nil {
						tok.Commit()
						errors = append(errors, ErrIncompleteTag("img"))
						return bits, errors
					}

					if t.Tag != "" {
						tok.Rollback()
						errors = append(errors, ErrInvalidUrl(t.Tag))
					} else {
						tok.Commit()
						url := html.EscapeString(t.Text)
						bits = append(bits, "<img class='image' style='", style, "' alt='", url, "' src='", url, "'>")
					}
				}
			case "code", "html", "csv":
				if !t.End {
					tag := t.Tag

					tok.Begin()
					t = tok.Next()
					if t == nil {
						tok.Commit()
						errors = append(errors, ErrIncompleteTag(tag))
						return bits, errors
					}

					if t.Tag != "" {
						tok.Rollback()
						errors = append(errors, ErrIncompleteTag(tag))
					} else {
						tok.Commit()
						switch tag {
						case "code":
							bits = append(bits, "<table class='code'>"+
								highlighter.Highlight([]byte(t.Text), 4)+"</table>")
						case "html":
							bits = append(bits, "<span class='html'>"+FilterHTML(t.Text, 0)+"</span>")
						case "csv":
							r := csv.NewReader(strings.NewReader(t.Text))
							record, err := r.ReadAll()

							if err != nil && err != io.EOF {
								errors = append(errors, err)
							}
							bits = append(bits, buildTable(record))
						}
					}
				}
			}
		}
	}
	return bits, errors
}

func FilterHTML(h string, textOnly int) string {
	var tok = _html.NewTokenizer(bytes.NewBufferString(h))
	var self = map[string]bool{
		"img": true,
		"hr":  true,
		"br":  true,
	}

	var ret bytes.Buffer
	var stack SStack
	var allowed = conf.GlobalServerConfig.HTMLTags
	var allowedAttrs = conf.GlobalServerConfig.HTMLAttrs

	for {
		tt := tok.Next()
		if tt == _html.ErrorToken {
			break
		}

		_tag, _ := tok.TagName()
		tag := string(_tag)
		text := string(tok.Text())
		_ = tok.Token()
		raw := string(tok.Raw())

		if tt == _html.TextToken {
			// Here golang will automatically unescape the text, re-escaping is necessary
			ret.WriteString(Escape(text))
		}

		if textOnly != 0 {
			if ret.Len() > textOnly {
				return ret.String()
			}
			continue
		}

		if allowed[tag] {
			if tt == _html.SelfClosingTagToken {
				ret.WriteString(raw)
			}

			if tt == _html.EndTagToken {
				ret.WriteString(raw)
				if !self[tag] {
					stack.Pop()
				}
			}

			if tt == _html.StartTagToken {

				ret.WriteString("<" + tag)
				for {
					k, v, m := tok.TagAttr()

					key := string(k)
					if key == "class" && tag == "img" {
						ret.WriteString(" class=\"article-image " + Escape(string(v)) + "\"")
					} else if allowedAttrs[key] {
						ret.WriteString(" " + key + "=\"" + Escape(string(v)) + "\"")
					}

					if !m {
						break
					}
				}
				ret.WriteString(">")

				if !self[tag] {
					stack.Push(tag)
				}
			}
		}

	}

	for len(stack.stack) > 0 {
		t := stack.Pop()
		ret.WriteString("</" + t + ">")
	}

	return ret.String()
}

func BBCodeToHTML(bbcode string) (string, string, []error) {
	tok := TokenizeString(bbcode, -1)
	bits, errs := tokensToHTML(tok)

	html := "<table class='bbcode'><tr class='zebra-false'><td>" + strings.Join(bits, "") + "</td></tr></table>"
	preview := FilterHTML(html, 256)

	return html, preview, errs
}
