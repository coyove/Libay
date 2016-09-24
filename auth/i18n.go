package auth

import (
	"encoding/json"
	"io/ioutil"
	"regexp"
	"strings"
)

func TranslateTemplate(file string, langFile string) map[string]string {
	ret := make(map[string]string)

	buf, err := ioutil.ReadFile(file)
	if err != nil {
		return ret
	}
	text := string(buf)

	langBuf, err := ioutil.ReadFile(langFile)
	if err != nil {
		return ret
	}
	i18n := make(map[string]interface{})
	err = json.Unmarshal(langBuf, &i18n)
	if err != nil {
		return ret
	}

	re := regexp.MustCompile(`\[\[(.+?)\]\]`)

	for k, im := range i18n {
		m := im.(map[string]interface{})

		ret[k] = re.ReplaceAllStringFunc(text, func(in string) string {
			backup := in[2 : len(in)-2]
			t := strings.ToLower(backup)

			if tr, e := m[t]; e {
				return tr.(string)
			} else {
				return backup
			}
		})
	}

	return ret
}
