package goway

import "strings"

// splitSQLStatements divides a raw SQL script into individual statements on the
// semicolon delimiter. It is aware of the lexical constructs that can legally
// contain a semicolon: single quoted strings, double quoted identifiers, line
// and block comments, and, for PostgreSQL, dollar quoted strings. For SQLite it
// additionally recognizes backtick and square bracket quoted identifiers and
// tracks BEGIN, CASE and END so that the inner statements of a trigger body are
// not split apart.
//
// Comments and whitespace are preserved inside each returned statement, but
// statements that are empty after trimming are discarded. The dialect argument
// selects the database specific behavior.
func splitSQLStatements(script, dialect string) ([]string, error) {
	dollarQuoting := dialect == dialectPostgres
	sqlite := dialect == dialectSQLite

	var statements []string
	var current strings.Builder
	blockDepth := 0
	length := len(script)
	index := 0

	flush := func() {
		statement := strings.TrimSpace(current.String())
		if statement != "" {
			statements = append(statements, statement)
		}
		current.Reset()
	}

	for index < length {
		character := script[index]
		switch {
		case character == '-' && index+1 < length && script[index+1] == '-':
			// Line comment: copy to the end of the line.
			current.WriteByte('-')
			current.WriteByte('-')
			index += 2
			for index < length && script[index] != '\n' {
				current.WriteByte(script[index])
				index++
			}

		case character == '/' && index+1 < length && script[index+1] == '*':
			// Block comment, supporting nesting as PostgreSQL does.
			nesting := 1
			current.WriteByte('/')
			current.WriteByte('*')
			index += 2
			for index < length && nesting > 0 {
				if script[index] == '/' && index+1 < length && script[index+1] == '*' {
					nesting++
					current.WriteByte('/')
					current.WriteByte('*')
					index += 2
					continue
				}
				if script[index] == '*' && index+1 < length && script[index+1] == '/' {
					nesting--
					current.WriteByte('*')
					current.WriteByte('/')
					index += 2
					continue
				}
				current.WriteByte(script[index])
				index++
			}

		case character == '\'':
			// A single quote immediately preceded by a standalone E introduces a
			// PostgreSQL escape string, in which a backslash escapes the following
			// character, including a quote.
			escapeString := index > 0 &&
				(script[index-1] == 'E' || script[index-1] == 'e') &&
				(index < 2 || !isWordByte(script[index-2]))
			index = copySingleQuoted(script, index, escapeString, &current)

		case character == '"':
			index = copyQuoted(script, index, '"', &current)

		case sqlite && character == '`':
			index = copyQuoted(script, index, '`', &current)

		case sqlite && character == '[':
			index = copyBracketQuoted(script, index, &current)

		case dollarQuoting && character == '$':
			if tag, ok := readDollarTag(script, index); ok {
				current.WriteString(tag)
				index += len(tag)
				closing := strings.Index(script[index:], tag)
				if closing < 0 {
					current.WriteString(script[index:])
					index = length
				} else {
					current.WriteString(script[index : index+closing])
					current.WriteString(tag)
					index += closing + len(tag)
				}
			} else {
				current.WriteByte(character)
				index++
			}

		case character == ';' && blockDepth == 0:
			current.WriteByte(';')
			index++
			flush()

		default:
			if sqlite && isWordStartByte(character) && (index == 0 || !isWordByte(script[index-1])) {
				word, advance := readWord(script, index)
				switch strings.ToUpper(word) {
				case "BEGIN", "CASE":
					blockDepth++
				case "END":
					if blockDepth > 0 {
						blockDepth--
					}
				}
				current.WriteString(word)
				index += advance
			} else {
				current.WriteByte(character)
				index++
			}
		}
	}

	flush()
	return statements, nil
}

// copyQuoted copies a quoted run that starts at the opening quote located at
// index. A doubled quote inside the run is treated as an escaped quote rather
// than a terminator. The returned index points just past the closing quote.
func copyQuoted(script string, index int, quote byte, current *strings.Builder) int {
	length := len(script)
	current.WriteByte(quote)
	index++
	for index < length {
		if script[index] == quote {
			if index+1 < length && script[index+1] == quote {
				current.WriteByte(quote)
				current.WriteByte(quote)
				index += 2
				continue
			}
			current.WriteByte(quote)
			index++
			break
		}
		current.WriteByte(script[index])
		index++
	}
	return index
}

// copySingleQuoted copies a single quoted string. A doubled quote is always an
// escaped quote. When escapeString is true, a backslash also escapes the
// following character, matching PostgreSQL escape strings. The returned index
// points just past the closing quote.
func copySingleQuoted(script string, index int, escapeString bool, current *strings.Builder) int {
	length := len(script)
	current.WriteByte('\'')
	index++
	for index < length {
		character := script[index]
		if escapeString && character == '\\' && index+1 < length {
			current.WriteByte(character)
			current.WriteByte(script[index+1])
			index += 2
			continue
		}
		if character == '\'' {
			if index+1 < length && script[index+1] == '\'' {
				current.WriteByte('\'')
				current.WriteByte('\'')
				index += 2
				continue
			}
			current.WriteByte('\'')
			index++
			break
		}
		current.WriteByte(character)
		index++
	}
	return index
}

// copyBracketQuoted copies a SQLite square bracket quoted identifier. The
// identifier runs from the opening bracket to the first closing bracket; SQLite
// does not support nested or escaped brackets. The returned index points just
// past the closing bracket.
func copyBracketQuoted(script string, index int, current *strings.Builder) int {
	length := len(script)
	current.WriteByte('[')
	index++
	for index < length {
		character := script[index]
		current.WriteByte(character)
		index++
		if character == ']' {
			break
		}
	}
	return index
}

// readDollarTag attempts to read a PostgreSQL dollar quote tag beginning at the
// dollar sign located at index. A tag is a dollar sign, an optional identifier,
// and a closing dollar sign, for example "$$" or "$function$". The identifier,
// when present, must begin with a letter or underscore, so a sequence such as
// "$1$" is not treated as a dollar quote. The full tag, including both dollar
// signs, is returned when one is present.
func readDollarTag(script string, index int) (string, bool) {
	cursor := index + 1
	if cursor < len(script) && script[cursor] != '$' {
		if !isWordStartByte(script[cursor]) {
			return "", false
		}
		cursor++
		for cursor < len(script) && isWordByte(script[cursor]) {
			cursor++
		}
	}
	if cursor < len(script) && script[cursor] == '$' {
		return script[index : cursor+1], true
	}
	return "", false
}

// readWord reads a maximal run of identifier bytes beginning at index and
// returns the run together with its length.
func readWord(script string, index int) (string, int) {
	cursor := index
	for cursor < len(script) && isWordByte(script[cursor]) {
		cursor++
	}
	return script[index:cursor], cursor - index
}

// isWordByte reports whether a byte can appear within an identifier.
func isWordByte(character byte) bool {
	return character == '_' ||
		(character >= 'a' && character <= 'z') ||
		(character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9')
}

// isWordStartByte reports whether a byte can begin an identifier.
func isWordStartByte(character byte) bool {
	return character == '_' ||
		(character >= 'a' && character <= 'z') ||
		(character >= 'A' && character <= 'Z')
}
