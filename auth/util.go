package auth

import (
	"../conf"

	"github.com/golang/glog"
	_html "golang.org/x/net/html"

	"bytes"
	"crypto/sha1"
	"fmt"
	"html"
	"os"
	"reflect"
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

func Select1(table string, id int, columns ...string) (map[string]interface{}, error) {
	sql := "SELECT "
	for _, col := range columns {
		sql += "\"" + col + "\","
	}
	sql = sql[:len(sql)-1] + " FROM " + table + " WHERE id = " + strconv.Itoa(id)

	count := len(columns)
	ret := make(map[string]interface{})
	ptrCols := make([]interface{}, count)
	ptrs := make([]interface{}, count)

	for i, _ := range ptrs {
		ptrCols[i] = &(ptrs[i])
	}

	err := Gdb.QueryRow(sql).Scan(ptrCols...)

	if err != nil {
		return ret, err
	}

	for i, v := range ptrCols {
		_v := *(v.(*interface{}))
		switch _v.(type) {
		case int64:
			// For convenient
			// Since we only run on 64bit platform, there is no difference between int and int64
			ret[columns[i]] = int(_v.(int64))
		case []uint8:
			// []uint8 -> []byte -> string
			ret[columns[i]] = string(_v.([]uint8))
		default:
			ret[columns[i]] = _v
		}
	}

	return ret, nil
}

type TableRow struct {
	Columns     []string
	ColumnTypes []string
	ColumnName  []string
}

func ReadTableDirect(table string, page int, whereStat string) ([]string, []TableRow, int) {
	ret := make([]TableRow, 0)
	columnNames := make([]string, 0)

	if whereStat != "" {
		whereStat = " WHERE " + whereStat
	}

	_app := conf.GlobalServerConfig.ArticlesPerPage
	_start := _app * (page - 1)

	var count, rowCount int

	cols, err := Gdb.Query("SELECT column_name FROM information_schema.columns WHERE table_name = '" + table + "'")
	if err != nil {
		return columnNames, ret, 0
	}

	defer cols.Close()

	for cols.Next() {
		var cn string
		cols.Scan(&cn)
		columnNames = append(columnNames, cn)
	}

	count = len(columnNames)

	if Gdb.QueryRow(`
        SELECT
            COUNT(id)
        FROM `+table+
		whereStat).Scan(&rowCount) != nil {
		return columnNames, ret, 0
	}

	rows, err := Gdb.Query(`
        SELECT
            *
        FROM ` + table +
		whereStat + `
        ORDER BY id DESC 
        OFFSET ` + strconv.Itoa(_start) + " LIMIT " + strconv.Itoa(_app))
	if err != nil {
		return columnNames, ret, 0
	}

	defer rows.Close()

	ptrCols := make([]interface{}, count)
	ptrs := make([]interface{}, count)
	for i, _ := range ptrs {
		ptrCols[i] = &(ptrs[i])
	}

	for rows.Next() {
		rows.Scan(ptrCols...)
		row := TableRow{}
		row.ColumnTypes = make([]string, 0)
		row.Columns = make([]string, 0)

		for _, v := range ptrCols {
			_v := *(v.(*interface{}))
			row.ColumnTypes = append(row.ColumnTypes, reflect.ValueOf(_v).String())

			s := ""

			switch _v.(type) {
			case []uint8:
				s = string(_v.([]byte))
			case int64:
				s = strconv.Itoa(int(_v.(int64)))
			case bool:
				s = strconv.FormatBool(_v.(bool))
			case time.Time:
				ts := (_v.(time.Time)).Unix()
				s = "ts:" + strconv.Itoa(int(ts))
			default:
				// log.Println("unknown type", reflect.ValueOf(_v).String())
			}

			if len(s) > 48 {
				s = s[:48] + "..."
			}
			row.Columns = append(row.Columns, s)
		}
		ret = append(ret, row)
	}

	return columnNames, ret, rowCount
}

func DeleteRowsDirect(table string, ids []int) string {
	sql := `delete from ` + table + ` where `
	for i, v := range ids {
		sql += "id = " + strconv.Itoa(v)
		if i < len(ids)-1 {
			sql += " or "
		}
	}

	if table == "images" {
		sql2 := `select image from ` + table + ` where `
		for i, v := range ids {
			sql2 += "id = " + strconv.Itoa(v)
			if i < len(ids)-1 {
				sql2 += " or "
			}
		}

		rows, err := Gdb.Query(sql2)
		if err == nil {
			defer rows.Close()

			for rows.Next() {
				var img string
				rows.Scan(&img)
				os.Remove("./images/" + img)
				os.Remove("./thumbs/" + img)
			}
		}
	}

	_, err := Gdb.Exec(sql)

	if err == nil {
		return "ok"
	} else {
		glog.Errorln("Database:", err)
		return "Err::DB::General_Failure"
	}
}
