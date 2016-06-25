package auth

import (
	"../conf"
	"bytes"
	"crypto/sha1"
	"fmt"
	_html "golang.org/x/net/html"
	"html"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
)

type SStack struct {
	stack []string
}

func (ss *SStack) Push(s string) {
	if ss.stack == nil {
		ss.stack = make([]string, 0)
	}
	ss.stack = append(ss.stack, s)
}

func (ss *SStack) Pop() (ret string) {
	if len(ss.stack) > 0 {
		ret = ss.stack[len(ss.stack)-1]
		ss.stack = ss.stack[:len(ss.stack)-1]
	}
	return
}

var Escape = html.EscapeString
var Unescape = html.UnescapeString

type _time struct {
}

func (t *_time) Now() string {
	return time.Now().Format(stdTimeFormat)
}

func (t *_time) F(tt time.Time) string {
	return tt.Format(stdTimeFormat)
}

var Time _time

func ExtractContent(h string, u AuthUser) (string, string, bool) {
	tok := _html.NewTokenizer(bytes.NewBufferString(h))
	var ret1 bytes.Buffer
	var ret2 bytes.Buffer

	flag := false
	var stack SStack
	var allowed = conf.GlobalServerConfig.HTMLTags
	var allowedAttrs = conf.GlobalServerConfig.HTMLAttrs
	// var allowed = map[string]bool{
	// 	"strike": true, "img": true, "p": true, "ol": true, "ul": true, "li": true,
	// 	"b": true, "del": true, "strong": true, "em": true, "i": true, "u": true,
	// 	"sub": true, "sup": true, "div": true, "br": true, "hr": true, "span": true,
	// 	"font": true, "a": true, "table": true, "tr": true, "td": true, "th": true,
	// 	"thead": true, "tbody": true, "pre": true, "h1": true, "h2": true, "h3": true,
	// 	"h4": true, "h5": true, "script": true,
	// }
	var self = map[string]bool{
		"img": true,
		"hr":  true,
		"br":  true,
	}
	// var allowedAttrs = map[string]bool{
	// 	"href": true, "target": true, "src": true, "alt": true, "title": true,
	// 	"id": true, "class": true, "height": true, "width": true,
	// }

	// reimg := regexp.MustCompile(`(?i)<(img(\s.+?)?)\/?>`)
	// reclean := regexp.MustCompile(`(?i)style=".+?"`)
	regist := regexp.MustCompile(`(?i)"https\:\/\/gist\.github\.com\/.+\/[0-9a-f]{32}\.js"`)

	for {
		tt := tok.Next()
		if tt == _html.ErrorToken {
			break
		}
		_tag, _ := tok.TagName()
		_text := strings.TrimSpace(string(tok.Text()))
		_ = tok.Token()

		_raw := tok.Raw()

		// log.Println(tt.String(), tk, string(_raw))

		if tt == _html.TextToken {
			ret2.WriteString(_text)

			if !flag {
				ret1.WriteString(_text)
			}

			if ret1.Len() > 256 {
				flag = true
			}
		}

		if tt == _html.SelfClosingTagToken {
			tag := string(_tag)
			if allowed[tag] {
				ret2.WriteString(string(_raw))
			}
		}

		if tt == _html.EndTagToken {
			tag := string(_tag)
			if allowed[tag] {
				ret2.WriteString(string(_raw))
				if !self[tag] {
					stack.Pop()
				}
			}
		}

		if tt == _html.StartTagToken {
			tag := string(_tag)
			raw := string(_raw)
			if allowed[tag] {
				// if tag == "img" {
				// 	raw = reimg.ReplaceAllString(raw, "<$1 class='article-image'>")
				// }
				// raw = reclean.ReplaceAllString(raw, "")
				if tag == "script" {
					if regist.MatchString(raw) {
						ret2.WriteString(raw)
					} else {
						ret2.WriteString("<script type='text'>")
					}
				} else {
					ret2.WriteString("<" + tag)
					for {
						k, v, m := tok.TagAttr()

						key := string(k)
						if key == "class" && tag == "img" {
							ret2.WriteString(" class=\"article-image\"")
						} else {
							if allowedAttrs[key] {
								ret2.WriteString(" " + key + "=\"" +
									template.HTMLEscapeString(string(v)) + "\"")
							}
						}
						if !m {
							break
						}

					}
					ret2.WriteString(">")
					// ret2.WriteString(raw)
				}
				if !self[tag] {
					stack.Push(tag)
				}
			}
		}

	}

	for len(stack.stack) > 0 {
		t := stack.Pop()
		ret2.WriteString("</" + t + ">")
	}

	return ret1.String(), ret2.String(), true
}

func CleanString(s string) (ret string) {
	re := regexp.MustCompile(`(\s|\'|\"|\=|\+|\-|\:')`)
	ret = re.ReplaceAllString(s, "_")

	if len(ret) > 64 {
		ret = ret[:64]
	}

	return
}

func MakeHash(pass ...interface{}) string {
	pl := Salt

	if len(pass) == 0 {
		pl += strconv.Itoa(int(time.Now().UnixNano()))
	} else {
		for _, v := range pass {
			pl += fmt.Sprintf("%v", v)
		}
	}

	bpl := []byte(pl)

	for i := 0; i < 3; i++ {
		tmp := sha1.Sum(bpl)
		bpl = tmp[:20]
	}
	return fmt.Sprintf("%x", bpl)
}
