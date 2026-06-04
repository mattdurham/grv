// Package treeformat converts grv JSON AST nodes to and from compact indented
// tree notation — a more token-efficient, agent-readable alternative to JSON.
//
// Format rules:
//
//	KindName scalar=val scalar2=val2    ← node header with inline scalar attrs
//	  childField                        ← single-object child block (no suffix)
//	    KindName ...
//	  listField[]                       ← object-array block ([] suffix)
//	    KindName ...
//	    KindName ...
//	  scalarList=[a,b,c]               ← scalar array inline
//	  emptyList=[]                      ← empty array inline
//
// The [] suffix on list fields is the key discriminator: it lets Unmarshal
// distinguish a single-item array (lhs[] → [{...}]) from a single object
// (body → {...}) without needing schema knowledge.
package treeformat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// Marshal converts a JSON AST node tree into compact indented tree notation.
// Returns the raw bytes unchanged on any parse error.
func Marshal(raw json.RawMessage) ([]byte, error) {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw, err
	}
	var buf bytes.Buffer
	writeValue(&buf, v, 0)
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Unmarshal parses tree notation back into a JSON AST node tree.
func Unmarshal(src []byte) (json.RawMessage, error) {
	lines := strings.Split(string(src), "\n")
	val, _, err := parseBlock(lines, 0, 0)
	if err != nil {
		return nil, err
	}
	if val == nil {
		return json.RawMessage("null"), nil
	}
	b, err := json.Marshal(val)
	return json.RawMessage(b), err
}

// writeValue writes any JSON value at the given indent depth.
func writeValue(buf *bytes.Buffer, v interface{}, depth int) {
	prefix := strings.Repeat("  ", depth)
	switch val := v.(type) {
	case map[string]interface{}:
		kind, hasKind := val["kind"].(string)
		if !hasKind {
			// Plain object (no kind discriminator): render as key=val lines.
			writePlainObject(buf, val, depth)
			return
		}
		fmt.Fprintf(buf, "%s%s\n", prefix, buildHeader(kind, val))
		writeChildren(buf, val, depth+1)
	case []interface{}:
		for _, item := range val {
			writeValue(buf, item, depth)
		}
	default:
		b, _ := json.Marshal(val)
		buf.WriteString(prefix)
		buf.Write(b)
		buf.WriteByte('\n')
	}
}

// writePlainObject renders a map without a "kind" field as key=val lines.
// Scalar values are inline; objects and arrays become indented child blocks.
func writePlainObject(buf *bytes.Buffer, obj map[string]interface{}, depth int) {
	prefix := strings.Repeat("  ", depth)
	for _, k := range sortedKeys(obj) {
		v := obj[k]
		if isScalar(v) {
			fmt.Fprintf(buf, "%s%s=%s\n", prefix, k, formatScalar(v))
			continue
		}
		switch cv := v.(type) {
		case map[string]interface{}:
			fmt.Fprintf(buf, "%s%s\n", prefix, k)
			writeValue(buf, cv, depth+1)
		case []interface{}:
			if len(cv) == 0 {
				fmt.Fprintf(buf, "%s%s=[]\n", prefix, k)
			} else if allScalars(cv) {
				parts := make([]string, len(cv))
				for i, item := range cv {
					parts[i] = formatScalar(item)
				}
				fmt.Fprintf(buf, "%s%s=[%s]\n", prefix, k, strings.Join(parts, ","))
			} else {
				fmt.Fprintf(buf, "%s%s[]\n", prefix, k)
				for _, item := range cv {
					writeValue(buf, item, depth+1)
				}
			}
		}
	}
}

// buildHeader builds "KindName scalar=val ..." from a node object.
func buildHeader(kind string, obj map[string]interface{}) string {
	parts := []string{kind}
	for _, k := range sortedKeys(obj) {
		if k == "kind" {
			continue
		}
		if isScalar(obj[k]) {
			parts = append(parts, k+"="+formatScalar(obj[k]))
		}
	}
	return strings.Join(parts, " ")
}

// writeChildren writes non-scalar fields of a node as labeled child blocks.
func writeChildren(buf *bytes.Buffer, obj map[string]interface{}, depth int) {
	prefix := strings.Repeat("  ", depth)
	for _, k := range sortedKeys(obj) {
		if k == "kind" {
			continue
		}
		v := obj[k]
		if isScalar(v) {
			continue
		}
		switch cv := v.(type) {
		case map[string]interface{}:
			// Single object child — no suffix.
			fmt.Fprintf(buf, "%s%s\n", prefix, k)
			writeValue(buf, cv, depth+1)
		case []interface{}:
			if len(cv) == 0 {
				// Empty array inline.
				fmt.Fprintf(buf, "%s%s=[]\n", prefix, k)
			} else if allScalars(cv) {
				// Scalar array inline.
				parts := make([]string, len(cv))
				for i, item := range cv {
					parts[i] = formatScalar(item)
				}
				fmt.Fprintf(buf, "%s%s=[%s]\n", prefix, k, strings.Join(parts, ","))
			} else {
				// Object array — "[]" suffix discriminates from single-object child.
				fmt.Fprintf(buf, "%s%s[]\n", prefix, k)
				for _, item := range cv {
					writeValue(buf, item, depth+1)
				}
			}
		}
	}
}

// parseBlock collects node items at `depth` until a shallower line is found.
// Returns: single node → that node, multiple nodes → []interface{}, none → nil.
func parseBlock(lines []string, lineIdx int, depth int) (interface{}, int, error) {
	var items []interface{}
	for lineIdx < len(lines) {
		line := lines[lineIdx]
		if strings.TrimSpace(line) == "" {
			lineIdx++
			continue
		}
		d := indentDepth(line)
		if d < depth {
			break
		}
		if d > depth {
			lineIdx++
			continue
		}
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" {
			lineIdx++
			continue
		}
		if !unicode.IsUpper(rune(trimmed[0])) {
			lineIdx++
			continue
		}
		node, next, err := parseNode(lines, lineIdx, depth)
		if err != nil {
			return nil, next, err
		}
		items = append(items, node)
		lineIdx = next
	}
	switch len(items) {
	case 0:
		return nil, lineIdx, nil
	case 1:
		return items[0], lineIdx, nil
	default:
		return items, lineIdx, nil
	}
}

// parseArrayBlock is like parseBlock but always wraps the result in []interface{}.
// Used for "label[]" blocks so single-item arrays round-trip correctly.
func parseArrayBlock(lines []string, lineIdx int, depth int) ([]interface{}, int, error) {
	val, next, err := parseBlock(lines, lineIdx, depth)
	if err != nil {
		return nil, next, err
	}
	if val == nil {
		return []interface{}{}, next, nil
	}
	if arr, ok := val.([]interface{}); ok {
		return arr, next, nil
	}
	return []interface{}{val}, next, nil
}

// parseNode parses one node header line and its children.
func parseNode(lines []string, lineIdx int, depth int) (map[string]interface{}, int, error) {
	trimmed := strings.TrimLeft(lines[lineIdx], " ")
	lineIdx++
	node := map[string]interface{}{}
	parts := splitHeader(trimmed)
	if len(parts) == 0 {
		return node, lineIdx, nil
	}
	node["kind"] = parts[0]
	for _, kv := range parts[1:] {
		k, v, _ := strings.Cut(kv, "=")
		node[k] = parseScalar(v)
	}
	var err error
	lineIdx, err = collectChildren(lines, lineIdx, depth+1, node)
	return node, lineIdx, err
}

// collectChildren reads child field lines at childDepth and populates node.
func collectChildren(lines []string, lineIdx int, childDepth int, node map[string]interface{}) (int, error) {
	for lineIdx < len(lines) {
		line := lines[lineIdx]
		if strings.TrimSpace(line) == "" {
			lineIdx++
			continue
		}
		d := indentDepth(line)
		if d < childDepth {
			break
		}
		if d > childDepth {
			lineIdx++
			continue
		}
		trimmed := strings.TrimLeft(line, " ")
		if trimmed == "" {
			lineIdx++
			continue
		}
		if unicode.IsUpper(rune(trimmed[0])) {
			break // sibling node
		}
		lineIdx++

		if eqIdx := strings.Index(trimmed, "="); eqIdx != -1 {
			// Scalar or scalar-array field: "key=value" or "key=[a,b,c]"
			k := trimmed[:eqIdx]
			v := trimmed[eqIdx+1:]
			node[k] = parseScalarOrArray(v)
		} else if strings.HasSuffix(trimmed, "[]") {
			// Object-array block: "label[]" — always wraps children in [].
			k := trimmed[:len(trimmed)-2]
			val, next, err := parseArrayBlock(lines, lineIdx, childDepth+1)
			if err != nil {
				return next, err
			}
			node[k] = val
			lineIdx = next
		} else {
			// Single-object child block: "label"
			k := trimmed
			val, next, err := parseBlock(lines, lineIdx, childDepth+1)
			if err != nil {
				return next, err
			}
			node[k] = val
			lineIdx = next
		}
	}
	return lineIdx, nil
}

// parseScalarOrArray parses "[a,b,c]" inline arrays and plain scalars.
func parseScalarOrArray(s string) interface{} {
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		inner := s[1 : len(s)-1]
		if inner == "" {
			return []interface{}{}
		}
		parts := strings.Split(inner, ",")
		result := make([]interface{}, len(parts))
		for i, p := range parts {
			result[i] = parseScalar(strings.TrimSpace(p))
		}
		return result
	}
	return parseScalar(s)
}

// parseScalar converts a tree scalar token to a Go value via JSON unmarshal.
func parseScalar(s string) interface{} {
	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

// indentDepth returns the 2-space indent level of a line.
func indentDepth(line string) int {
	n := 0
	for _, c := range line {
		if c != ' ' {
			break
		}
		n++
	}
	return n / 2
}

// splitHeader tokenizes a header line respecting quoted strings.
func splitHeader(s string) []string {
	var parts []string
	var cur strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
			cur.WriteByte(c)
		case c == ' ' && !inQuote:
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}
	return parts
}

// formatScalar serializes a scalar value for inline tree output.
func formatScalar(v interface{}) string {
	switch val := v.(type) {
	case string:
		if needsQuoting(val) {
			b, _ := json.Marshal(val)
			return string(b)
		}
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// needsQuoting reports whether a string value must be quoted in tree output.
func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '=' || c == '[' || c == ']' || c == '\n' || c == '"' {
			return true
		}
	}
	return false
}

func isScalar(v interface{}) bool {
	switch v.(type) {
	case string, float64, bool, nil:
		return true
	}
	return false
}

func allScalars(arr []interface{}) bool {
	for _, v := range arr {
		if !isScalar(v) {
			return false
		}
	}
	return true
}

// sortedKeys returns map keys in order: scalars first (alpha), then non-scalars (alpha).
// "kind" is excluded (callers prepend it as the node type).
func sortedKeys(obj map[string]interface{}) []string {
	var scalars, nonScalars []string
	for k := range obj {
		if k == "kind" {
			continue
		}
		if isScalar(obj[k]) {
			scalars = append(scalars, k)
		} else {
			nonScalars = append(nonScalars, k)
		}
	}
	insertionSort(scalars)
	insertionSort(nonScalars)
	result := make([]string, 0, len(scalars)+len(nonScalars))
	result = append(result, scalars...)
	result = append(result, nonScalars...)
	return result
}

func insertionSort(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}

	// displayKey and storageKey are identity — field names pass through unchanged.
}

func displayKey(k string) string { return k }
func storageKey(k string) string { return k }
