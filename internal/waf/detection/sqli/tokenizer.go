package sqli

import (
	"strings"
	"unicode"
)

// TokenType represents the type of a SQL token.
type TokenType int

const (
	TokenString    TokenType = iota // 'value', "value"
	TokenNumber                     // 123, 0x1A
	TokenKeyword                    // SELECT, UNION, OR, AND
	TokenOperator                   // =, <>, !=, >=, LIKE
	TokenFunction                   // COUNT(), SLEEP()
	TokenComment                    // --, /* */, #
	TokenParen                      // (, )
	TokenComma                      // ,
	TokenSemicolon                  // ;
	TokenWildcard                   // *
	TokenOther                      // anything else
)

// Token represents a single SQL token.
type Token struct {
	Type  TokenType
	Value string
}

// Tokenize tokenizes an input string into SQL tokens.
func Tokenize(input string) []Token {
	var tokens []Token
	runes := []rune(input)
	i := 0

	for i < len(runes) {
		ch := runes[i]

		// Skip whitespace
		if unicode.IsSpace(ch) {
			i++
			continue
		}

		// Single-line comment: -- or #
		if ch == '-' && i+1 < len(runes) && runes[i+1] == '-' {
			end := i + 2
			for end < len(runes) && runes[end] != '\n' {
				end++
			}
			tokens = append(tokens, Token{Type: TokenComment, Value: string(runes[i:end])})
			i = end
			continue
		}
		if ch == '#' {
			end := i + 1
			for end < len(runes) && runes[end] != '\n' {
				end++
			}
			tokens = append(tokens, Token{Type: TokenComment, Value: string(runes[i:end])})
			i = end
			continue
		}

		// Multi-line comment: /* */
		if ch == '/' && i+1 < len(runes) && runes[i+1] == '*' {
			end := i + 2
			for end+1 < len(runes) {
				if runes[end] == '*' && runes[end+1] == '/' {
					end += 2
					break
				}
				end++
			}
			if end >= len(runes) {
				end = len(runes)
			}
			tokens = append(tokens, Token{Type: TokenComment, Value: string(runes[i:end])})
			i = end
			continue
		}

		// String literals: 'value' or "value"
		if ch == '\'' || ch == '"' {
			end := i + 1
			quote := ch
			for end < len(runes) {
				if runes[end] == quote {
					// Check for escaped quote ('')
					if end+1 < len(runes) && runes[end+1] == quote {
						end += 2
						continue
					}
					end++
					break
				}
				if runes[end] == '\\' && end+1 < len(runes) {
					end += 2
					continue
				}
				end++
			}
			tokens = append(tokens, Token{Type: TokenString, Value: string(runes[i:end])})
			i = end
			continue
		}

		// Backtick (identifier quoting in MySQL)
		if ch == '`' {
			end := i + 1
			for end < len(runes) && runes[end] != '`' {
				end++
			}
			if end < len(runes) {
				end++
			}
			tokens = append(tokens, Token{Type: TokenOther, Value: string(runes[i:end])})
			i = end
			continue
		}

		// Numbers (including hex: 0x...)
		if unicode.IsDigit(ch) || (ch == '0' && i+1 < len(runes) && (runes[i+1] == 'x' || runes[i+1] == 'X')) {
			end := i
			if ch == '0' && end+1 < len(runes) && (runes[end+1] == 'x' || runes[end+1] == 'X') {
				end += 2
				for end < len(runes) && isHexDigit(runes[end]) {
					end++
				}
			} else {
				for end < len(runes) && (unicode.IsDigit(runes[end]) || runes[end] == '.') {
					end++
				}
			}
			tokens = append(tokens, Token{Type: TokenNumber, Value: string(runes[i:end])})
			i = end
			continue
		}

		// Identifiers and keywords
		if unicode.IsLetter(ch) || ch == '_' {
			end := i
			for end < len(runes) && (unicode.IsLetter(runes[end]) || unicode.IsDigit(runes[end]) || runes[end] == '_') {
				end++
			}
			word := string(runes[i:end])
			upper := strings.ToUpper(word)

			// Check if it's a function call (followed by parenthesis)
			nextNonSpace := end
			for nextNonSpace < len(runes) && unicode.IsSpace(runes[nextNonSpace]) {
				nextNonSpace++
			}
			if nextNonSpace < len(runes) && runes[nextNonSpace] == '(' {
				if _, ok := dangerousFunctions[upper]; ok {
					tokens = append(tokens, Token{Type: TokenFunction, Value: upper})
				} else if sqlKeywords[upper] {
					tokens = append(tokens, Token{Type: TokenKeyword, Value: upper})
				} else {
					tokens = append(tokens, Token{Type: TokenFunction, Value: upper})
				}
			} else if sqlKeywords[upper] {
				tokens = append(tokens, Token{Type: TokenKeyword, Value: upper})
			} else {
				tokens = append(tokens, Token{Type: TokenOther, Value: word})
			}
			i = end
			continue
		}

		// Operators
		if ch == '=' || ch == '<' || ch == '>' || ch == '!' {
			end := i + 1
			if end < len(runes) && (runes[end] == '=' || runes[end] == '>') {
				end++
			}
			tokens = append(tokens, Token{Type: TokenOperator, Value: string(runes[i:end])})
			i = end
			continue
		}

		// Special characters
		switch ch {
		case '(':
			tokens = append(tokens, Token{Type: TokenParen, Value: "("})
			i++
		case ')':
			tokens = append(tokens, Token{Type: TokenParen, Value: ")"})
			i++
		case ',':
			tokens = append(tokens, Token{Type: TokenComma, Value: ","})
			i++
		case ';':
			tokens = append(tokens, Token{Type: TokenSemicolon, Value: ";"})
			i++
		case '*':
			tokens = append(tokens, Token{Type: TokenWildcard, Value: "*"})
			i++
		case '|':
			if i+1 < len(runes) && runes[i+1] == '|' {
				tokens = append(tokens, Token{Type: TokenOperator, Value: "||"})
				i += 2
			} else {
				tokens = append(tokens, Token{Type: TokenOther, Value: "|"})
				i++
			}
		default:
			tokens = append(tokens, Token{Type: TokenOther, Value: string(ch)})
			i++
		}
	}

	return tokens
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}
