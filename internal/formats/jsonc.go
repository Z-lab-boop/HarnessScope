package formats

import (
	"bytes"
	"fmt"
)

// NormalizeJSONC removes JSONC comments and trailing commas without touching
// comment-like text inside quoted strings. Newlines are preserved for useful
// downstream syntax locations.
func NormalizeJSONC(input []byte) ([]byte, error) {
	withoutComments := make([]byte, 0, len(input))
	inString := false
	escaped := false
	for index := 0; index < len(input); index++ {
		current := input[index]
		if inString {
			withoutComments = append(withoutComments, current)
			if escaped {
				escaped = false
				continue
			}
			if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			withoutComments = append(withoutComments, current)
			continue
		}
		if current == '/' && index+1 < len(input) {
			switch input[index+1] {
			case '/':
				index += 2
				for index < len(input) && input[index] != '\n' {
					index++
				}
				if index < len(input) {
					withoutComments = append(withoutComments, '\n')
				}
				continue
			case '*':
				index += 2
				closed := false
				for index < len(input) {
					if input[index] == '\n' {
						withoutComments = append(withoutComments, '\n')
					}
					if input[index] == '*' && index+1 < len(input) && input[index+1] == '/' {
						index++
						closed = true
						break
					}
					index++
				}
				if !closed {
					return nil, fmt.Errorf("unterminated block comment")
				}
				continue
			}
		}
		withoutComments = append(withoutComments, current)
	}
	if inString {
		return nil, fmt.Errorf("unterminated string")
	}

	result := make([]byte, 0, len(withoutComments))
	inString = false
	escaped = false
	for index, current := range withoutComments {
		if inString {
			result = append(result, current)
			if escaped {
				escaped = false
			} else if current == '\\' {
				escaped = true
			} else if current == '"' {
				inString = false
			}
			continue
		}
		if current == '"' {
			inString = true
			result = append(result, current)
			continue
		}
		if current == ',' {
			next := bytes.TrimLeft(withoutComments[index+1:], " \t\r\n")
			if len(next) > 0 && (next[0] == '}' || next[0] == ']') {
				continue
			}
		}
		result = append(result, current)
	}
	return result, nil
}
