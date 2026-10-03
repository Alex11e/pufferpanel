package utils

import (
	"strings"

	"github.com/spf13/cast"
)

type StringReplaceFunc func(str string, key string, value any) string

var PlainReplace = func(str string, key string, value any) string {
	return strings.ReplaceAll(str, key, cast.ToString(value))
}

var ShellReplace = func(str string, key string, value any) string {
	valueString := cast.ToString(value)
	if valueString == "" || strings.ContainsAny(valueString, " \t\r\n\"'\\$`><|&;()") {
		return strings.ReplaceAll(str, key, shellQuote(valueString))
	}
	return PlainReplace(str, key, valueString)
}

func shellQuote(value string) string {
	if value == "" {
		return "\"\""
	}

	var builder strings.Builder
	builder.WriteByte('"')
	for _, ch := range value {
		switch ch {
		case '\\':
			builder.WriteString("\\\\")
		case '"':
			builder.WriteString("\\\"")
		case '$':
			builder.WriteString("\\$")
		case '`':
			builder.WriteString("\\`")
		case '>', '<', '|', '&', ';', '(', ')':
			builder.WriteByte('\\')
			builder.WriteRune(ch)
		default:
			builder.WriteRune(ch)
		}
	}
	builder.WriteByte('"')
	return builder.String()
}

func ReplaceTokens(msg string, mapping map[string]any, function StringReplaceFunc) string {
	if function == nil {
		function = PlainReplace
	}

	newmsg := msg
	for key, value := range mapping {
		newmsg = function(newmsg, "${"+key+"}", value)
	}
	return newmsg
}

func ReplaceTokensInArr(msg []string, mapping map[string]any) []string {
	newarr := make([]string, len(msg))
	for index, element := range msg {
		newarr[index] = ReplaceTokens(element, mapping, PlainReplace)
	}
	return newarr
}

func ReplaceTokensInMap(msg map[string]string, mapping map[string]any) map[string]string {
	newarr := make(map[string]string, len(msg))
	for index, element := range msg {
		newarr[index] = ReplaceTokens(element, mapping, PlainReplace)
	}
	return newarr
}

func SplitArguments(source string) (cmd string, arguments []string) {
	if source == "" {
		return "", []string{}
	}

	var parts []string
	var current strings.Builder
	quoteMode := false
	for i := 0; i < len(source); i++ {
		switch source[i] {
		case '\\':
			if quoteMode && i+1 < len(source) && (source[i+1] == '\\' || source[i+1] == '"') {
				current.WriteByte(source[i+1])
				i++
				continue
			}
			current.WriteByte(source[i])
		case '"':
			quoteMode = !quoteMode
		case ' ', '\t', '\r', '\n':
			if quoteMode {
				current.WriteByte(source[i])
				continue
			}
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(source[i])
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	if len(parts) == 0 {
		return "", []string{}
	} else if len(parts) == 1 {
		return parts[0], []string{}
	}

	return parts[0], parts[1:]
}

func SplitArgumentsLegacy(source string) (cmd string, arguments []string) {
	if source == "" {
		return "", []string{}
	}

	results := []string{""}

	skip := false //if this is set, the next char is always added to the current string
	inQuote := false
	for _, v := range source {
		if skip {
			skip = false
			results[len(results)-1] += string(v)
			continue
		}
		switch v {
		case '\\':
			{
				skip = true
			}
		case '"':
			{
				inQuote = !inQuote
			}
		case ' ':
			{
				if inQuote {
					results[len(results)-1] += string(v)
				} else {
					results = append(results, "")
				}
			}
		default:
			results[len(results)-1] += string(v)
		}
	}

	//remove any "empty" items
	i := 0 // output index
	for _, x := range results {
		if x != "" {

			results[i] = x
			i++
		}
	}
	results = results[:i]

	cmd = results[0]
	arguments = results[1:]
	return
}

func MergeArguments(arguments []string) string {
	var result string
	for _, v := range arguments {
		if result != "" {
			result += " "
		}
		if strings.Contains(v, " ") && !strings.HasPrefix(v, "\"") {
			result += "\"" + v + "\""
		} else {
			result += v
		}
	}
	return result
}
