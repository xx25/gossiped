package editor

import (
	"strings"
	"unicode"
)

const (
	QuoteStops  = "<\"'-"
	MaxQuoteLen = 40
)

// IsQuoteChar checks if the given character is a quote character
// In our implementation, only '>' is considered a quote character
func IsQuoteChar(char rune) bool {
	return char == '>'
}

// IsQuoteEnhanced performs enhanced quote detection based on GoldED+ is_quote2() algorithm
// This is the only quote detection method we use (no basic detection)
func IsQuoteEnhanced(line string, prevLines []string) bool {
	if len(line) == 0 {
		return false
	}

	// Convert to runes for proper Unicode handling
	runes := []rune(line)
	head := 0
	
	// Skip leading whitespace
	for head < len(runes) && unicode.IsSpace(runes[head]) {
		head++
	}
	
	if head >= len(runes) {
		return false
	}
	
	// Search for first '>' before CR, NUL or other quote stop character
	found := false
	pos := head
	for pos < len(runes) && !found {
		if runes[pos] == '>' {
			found = true
		} else {
			// Check for quote stop characters or control characters
			if strings.ContainsRune(QuoteStops, runes[pos]) || runes[pos] == '\r' || runes[pos] == '\n' {
				return true
			}
		}
		pos++
	}
	
	if !found {
		return false
	}
	
	// Check if line after '>' is also quoted (double quoted)
	if pos < len(runes) {
		remainingLine := string(runes[pos:])
		if IsQuoteBasic(remainingLine) {
			return true
		}
	}
	
	// Pattern: "SPACE*[a-zA-Z]{0,3}>"
	// Count alphabetic characters between head and '>' position
	alphaCount := 0
	for i := head; i < pos-1; i++ {
		if unicode.IsLetter(runes[i]) {
			alphaCount++
		}
	}
	
	// If we have 0-3 alphabetic characters and they make up the entire prefix
	if alphaCount < 4 && alphaCount == (pos-1-head) {
		return true
	}
	
	// Check previous lines for context
	return checkPreviousLinesContext(prevLines)
}

// IsQuoteBasic performs basic quote detection for use within IsQuoteEnhanced
func IsQuoteBasic(line string) bool {
	if len(line) == 0 {
		return false
	}
	
	runes := []rune(line)
	ptr := 0
	endPtr := len(runes)
	if endPtr > 11 {
		endPtr = 11 // Scan limit: 10 chars + 1
	}
	
	// Skip leading whitespace and line feeds
	for ptr < len(runes) && (unicode.IsSpace(runes[ptr]) || runes[ptr] == '\n') {
		ptr++
	}
	
	// Check for empty string or exceeded scan limit
	if ptr >= len(runes) || ptr >= endPtr {
		return false
	}
	
	// Check for immediate quote character after whitespace
	if IsQuoteChar(runes[ptr]) {
		return true
	}
	
	// Scan for quote pattern: alphanumeric sequence followed by quote char
	for ptr < len(runes) && ptr < endPtr {
		if IsQuoteChar(runes[ptr]) {
			return true
		}
		if unicode.IsControl(runes[ptr]) || 
		   strings.ContainsRune(QuoteStops, runes[ptr]) || 
		   unicode.IsSpace(runes[ptr]) {
			break
		}
		ptr++
	}
	
	return ptr < endPtr && ptr < len(runes) && IsQuoteChar(runes[ptr])
}

// checkPreviousLinesContext analyzes previous lines for quote context
func checkPreviousLinesContext(prevLines []string) bool {
	if len(prevLines) == 0 {
		return true
	}
	
	var paragraph *string
	
	for i := len(prevLines) - 1; i >= 0; i-- {
		line := prevLines[i]
		
		// Previous line is quoted?
		if IsQuoteBasic(line) {
			return true
		}
		
		// Begin of paragraph?
		if len(line) == 0 || line[0] == '\n' || line[0] == '\r' {
			if paragraph != nil {
				return true
			} else {
				paragraph = &line
				continue
			}
		}
		
		// Kludge line (starts with CTRL_A)?
		if len(line) > 0 && line[0] == '\x01' {
			return true
		}
		
		// Found begin of citation block?
		lastLT := strings.LastIndex(line, "<")
		if lastLT != -1 {
			// Found both '<' and '>'?
			if strings.Index(line[lastLT:], ">") != -1 {
				return true
			}
			
			// Search for '>' in following lines (up to current)
			for j := i + 1; j < len(prevLines); j++ {
				if strings.Contains(prevLines[j], ">") {
					return true
				}
			}
			
			// Don't quote current line
			return false
		}
	}
	
	return true
}

// GetQuoteString extracts the quote string from a line
// Returns the quote string and its length
func GetQuoteString(line string) (string, int) {
	if !IsQuoteBasic(line) {
		return "", 0
	}
	
	runes := []rune(line)
	start := 0
	
	// Skip leading whitespace
	for start < len(runes) && (unicode.IsSpace(runes[start]) || runes[start] == '\n') {
		start++
	}
	
	// Find first quote character
	for start < len(runes) && !IsQuoteChar(runes[start]) {
		start++
	}
	
	if start >= len(runes) {
		return "", 0
	}
	
	// Find end of quote string (skip consecutive quote characters)
	end := start
	for end < len(runes) && IsQuoteChar(runes[end]) {
		end++
	}
	
	// Include following space if present
	if end < len(runes) && (unicode.IsSpace(runes[end]) || runes[end] == '\n') {
		end++
	}
	
	// Extract quote string, filtering out line feeds
	var result strings.Builder
	for i := 0; i < end && result.Len() < MaxQuoteLen-1; i++ {
		if runes[i] != '\n' {
			result.WriteRune(runes[i])
		}
	}
	
	quoteStr := result.String()
	return quoteStr, len(quoteStr)
}

// GetQuoteLevel counts the number of '>' characters in a quote string
// Used for alternating quote colors
func GetQuoteLevel(line string) int {
	quoteStr, _ := GetQuoteString(line)
	level := 0
	
	for _, char := range quoteStr {
		if IsQuoteChar(char) {
			level++
		}
	}
	
	return level
}

// ShouldEliminateQuote determines if quote string should be eliminated
// based on cursor position (for Enter key handling)
func ShouldEliminateQuote(line string, cursorPos int) bool {
	_, quoteLen := GetQuoteString(line)
	if quoteLen == 0 {
		return false
	}
	
	// Convert to runes for proper Unicode handling
	runes := []rune(line)
	runeLen := len(runes)
	
	// Eliminate quote string if:
	// - cursor is at end of line (cursorPos >= runeLen)
	// - cursor points to linefeed (line[cursorPos] == '\n')
	// - cursor is inside quote string (cursorPos < quoteLen)
	
	// Check if cursor is at end of line
	if cursorPos >= runeLen {
		return true
	}
	
	// Check if cursor points to linefeed
	if cursorPos < runeLen && runes[cursorPos] == '\n' {
		return true
	}
	
	// Check if cursor is inside quote string
	if cursorPos < quoteLen {
		return true
	}
	
	return false
}

// WordWrapQuoteAware performs word wrapping while preserving quote strings
// This is similar to the existing WordWrap but handles quote prefixes specially
func WordWrapQuoteAware(text string, width int, quotemargin int) (lines []string) {
	if len(text) == 0 {
		return []string{""}
	}
	
	// Split text into lines first
	inputLines := strings.Split(text, "\n")
	
	for _, line := range inputLines {
		if len(line) == 0 {
			lines = append(lines, "")
			continue
		}
		
		// Check if this line is quoted
		quoteStr, quoteLen := GetQuoteString(line)
		
		// Determine margin to use
		margin := width
		if quoteLen > 0 {
			margin = quotemargin
		}
		
		// If line fits within margin, no wrapping needed
		if len(line) <= margin {
			lines = append(lines, line)
			continue
		}
		
		// Extract the text content after the quote string
		contentStart := quoteLen
		content := line[contentStart:]
		
		// Wrap the content
		wrappedContent := wrapTextContent(content, margin - quoteLen)
		
		// Add quote string to each wrapped line
		for i, wrappedLine := range wrappedContent {
			if i == 0 {
				// First line keeps the original quote string
				lines = append(lines, quoteStr + wrappedLine)
			} else {
				// Continuation lines get the same quote string
				lines = append(lines, quoteStr + wrappedLine)
			}
		}
	}
	
	return lines
}

// wrapTextContent wraps text content at word boundaries
func wrapTextContent(content string, maxWidth int) []string {
	if len(content) == 0 {
		return []string{""}
	}
	
	if len(content) <= maxWidth {
		return []string{content}
	}
	
	var result []string
	var currentLine strings.Builder
	words := strings.Fields(content)
	
	for _, word := range words {
		// Check if adding this word would exceed the width
		if currentLine.Len() > 0 && currentLine.Len() + 1 + len(word) > maxWidth {
			// Start a new line
			result = append(result, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		} else {
			// Add word to current line
			if currentLine.Len() > 0 {
				currentLine.WriteString(" ")
			}
			currentLine.WriteString(word)
		}
	}
	
	// Add the last line if it has content
	if currentLine.Len() > 0 {
		result = append(result, currentLine.String())
	}
	
	return result
}

// CanReflowQuotedLines determines if two quoted lines can be reflowed together
// Lines can be reflowed if they have the same quote string
func CanReflowQuotedLines(line1, line2 string) bool {
	quote1, _ := GetQuoteString(line1)
	quote2, _ := GetQuoteString(line2)
	
	return quote1 == quote2
}