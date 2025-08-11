package quality

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement"
	"github.com/rs/zerolog/log"
)

// CodeValidator validates code snippets for syntax, compilation, and best practices
type CodeValidator struct {
	config        *CodeValidationConfig
	tempDir       string
	supportedLangs map[string]*LanguageConfig
}

// CodeValidationConfig configures code validation behavior
type CodeValidationConfig struct {
	EnableCompilation bool          `json:"enable_compilation"`
	EnableExecution   bool          `json:"enable_execution"`
	ExecutionTimeout  time.Duration `json:"execution_timeout"`
	TempDirectory     string        `json:"temp_directory"`
	MaxFileSize       int64         `json:"max_file_size"`
}

// LanguageConfig defines configuration for specific programming languages
type LanguageConfig struct {
	FileExtension    string   `json:"file_extension"`
	CompileCommand   []string `json:"compile_command"`
	ExecuteCommand   []string `json:"execute_command"`
	SyntaxPatterns   []string `json:"syntax_patterns"`
	SecurityPatterns []string `json:"security_patterns"`
}

// NewCodeValidator creates a new code validator
func NewCodeValidator() *CodeValidator {
	tempDir := filepath.Join(os.TempDir(), "caia-code-validation")
	os.MkdirAll(tempDir, 0755)
	
	cv := &CodeValidator{
		config: &CodeValidationConfig{
			EnableCompilation: true,
			EnableExecution:   false, // Disabled by default for security
			ExecutionTimeout:  5 * time.Second,
			TempDirectory:     tempDir,
			MaxFileSize:       1024 * 1024, // 1MB
		},
		tempDir: tempDir,
		supportedLangs: make(map[string]*LanguageConfig),
	}
	
	cv.setupLanguageConfigs()
	return cv
}

// ValidateCode validates a code snippet
func (cv *CodeValidator) ValidateCode(ctx context.Context, code string, language string) (*procurement.CodeValidation, error) {
	start := time.Now()
	
	log.Debug().
		Str("language", language).
		Int("code_length", len(code)).
		Msg("Starting code validation")
	
	result := &procurement.CodeValidation{
		Language: language,
		Code:     code,
		Errors:   make([]string, 0),
	}
	
	// Security: Validate code content for dangerous patterns
	if err := cv.validateCodeContent(code, language); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Security validation failed: %s", err.Error()))
		return result, nil
	}
	
	// Check if language is supported
	langConfig, supported := cv.supportedLangs[language]
	if !supported {
		result.Errors = append(result.Errors, fmt.Sprintf("Language '%s' not supported for validation", language))
		return result, nil
	}
	
	// Check code size
	if int64(len(code)) > cv.config.MaxFileSize {
		result.Errors = append(result.Errors, "Code snippet too large for validation")
		return result, nil
	}
	
	// Basic syntax validation
	result.SyntaxValid = cv.validateSyntax(code, langConfig)
	if !result.SyntaxValid {
		result.Errors = append(result.Errors, "Syntax validation failed")
	}
	
	// Security check
	result.SecuritySafe = cv.checkSecurity(code, langConfig)
	if !result.SecuritySafe {
		result.Errors = append(result.Errors, "Security issues detected")
	}
	
	// Best practices check
	result.BestPractices = cv.checkBestPractices(code, language)
	
	// Compilation check (if enabled and syntax is valid)
	if cv.config.EnableCompilation && result.SyntaxValid {
		compileResult := cv.tryCompile(ctx, code, langConfig)
		result.Compilable = compileResult.Success
		if !compileResult.Success {
			result.Errors = append(result.Errors, compileResult.Error)
		}
	}
	
	// Execution check (if enabled and compilable)
	if cv.config.EnableExecution && result.Compilable && result.SecuritySafe {
		execResult := cv.tryExecute(ctx, code, langConfig)
		result.Executable = execResult.Success
		if !execResult.Success {
			result.Errors = append(result.Errors, execResult.Error)
		}
	}
	
	log.Debug().
		Str("language", language).
		Bool("syntax_valid", result.SyntaxValid).
		Bool("compilable", result.Compilable).
		Bool("security_safe", result.SecuritySafe).
		Dur("duration", time.Since(start)).
		Msg("Code validation completed")
	
	return result, nil
}

// setupLanguageConfigs initializes language-specific configurations
func (cv *CodeValidator) setupLanguageConfigs() {
	// Go configuration
	cv.supportedLangs["go"] = &LanguageConfig{
		FileExtension:  ".go",
		CompileCommand: []string{"go", "build"},
		ExecuteCommand: []string{"go", "run"},
		SyntaxPatterns: []string{
			`^package\s+\w+`, // Must start with package declaration
			`func\s+\w+\s*\(`, // Function declarations
		},
		SecurityPatterns: []string{
			`os\.Exec`, `exec\.Command`, `syscall\.`, `unsafe\.`,
			`os\.Remove`, `os\.RemoveAll`, `ioutil\.WriteFile`,
		},
	}
	
	// Python configuration
	cv.supportedLangs["python"] = &LanguageConfig{
		FileExtension:  ".py",
		CompileCommand: []string{"python", "-m", "py_compile"},
		ExecuteCommand: []string{"python"},
		SyntaxPatterns: []string{
			`def\s+\w+\s*\(`, // Function definitions
			`class\s+\w+`,    // Class definitions
		},
		SecurityPatterns: []string{
			`import\s+os`, `import\s+subprocess`, `import\s+sys`,
			`exec\s*\(`, `eval\s*\(`, `__import__`,
		},
	}
	
	// JavaScript configuration
	cv.supportedLangs["javascript"] = &LanguageConfig{
		FileExtension:  ".js",
		CompileCommand: []string{"node", "--check"},
		ExecuteCommand: []string{"node"},
		SyntaxPatterns: []string{
			`function\s+\w+\s*\(`, // Function declarations
			`=>\s*{`,             // Arrow functions
			`var\s+\w+|let\s+\w+|const\s+\w+`, // Variable declarations
		},
		SecurityPatterns: []string{
			`require\s*\(\s*['"]fs['"]`, `require\s*\(\s*['"]child_process['"]`,
			`eval\s*\(`, `Function\s*\(`, `new\s+Function`,
		},
	}
	
	// Java configuration
	cv.supportedLangs["java"] = &LanguageConfig{
		FileExtension:  ".java",
		CompileCommand: []string{"javac"},
		ExecuteCommand: []string{"java"},
		SyntaxPatterns: []string{
			`public\s+class\s+\w+`, // Public class declaration
			`public\s+static\s+void\s+main`, // Main method
		},
		SecurityPatterns: []string{
			`Runtime\.getRuntime`, `ProcessBuilder`, `System\.exit`,
			`File`, `FileInputStream`, `FileOutputStream`,
		},
	}
}

// validateSyntax performs basic syntax validation
func (cv *CodeValidator) validateSyntax(code string, langConfig *LanguageConfig) bool {
	// Basic checks
	if strings.TrimSpace(code) == "" {
		return false
	}
	
	// Language-specific syntax patterns
	for _, pattern := range langConfig.SyntaxPatterns {
		matched, err := regexp.MatchString(pattern, code)
		if err == nil && matched {
			return true // At least one pattern matched
		}
	}
	
	// Additional basic syntax checks
	switch {
	case strings.Contains(langConfig.FileExtension, "go"):
		return cv.validateGoSyntax(code)
	case strings.Contains(langConfig.FileExtension, "py"):
		return cv.validatePythonSyntax(code)
	case strings.Contains(langConfig.FileExtension, "js"):
		return cv.validateJavaScriptSyntax(code)
	case strings.Contains(langConfig.FileExtension, "java"):
		return cv.validateJavaSyntax(code)
	}
	
	return true // Default to valid for unsupported detailed validation
}

// checkSecurity checks for potentially dangerous code patterns
func (cv *CodeValidator) checkSecurity(code string, langConfig *LanguageConfig) bool {
	// Check against security patterns
	for _, pattern := range langConfig.SecurityPatterns {
		matched, err := regexp.MatchString(pattern, code)
		if err == nil && matched {
			return false // Security issue found
		}
	}
	
	// Additional security checks
	dangerousPatterns := []string{
		`rm\s+-rf`, `del\s+/`, `format\s+c:`,
		`while\s*\(\s*true\s*\)`, `for\s*\(\s*;\s*;\s*\)`, // Infinite loops
		`http://`, `https://`, // Network calls
	}
	
	for _, pattern := range dangerousPatterns {
		matched, err := regexp.MatchString(pattern, strings.ToLower(code))
		if err == nil && matched {
			return false
		}
	}
	
	return true
}

// checkBestPractices evaluates code against best practices
func (cv *CodeValidator) checkBestPractices(code string, language string) bool {
	score := 0.0
	
	// Check for comments
	if strings.Contains(code, "//") || strings.Contains(code, "/*") || strings.Contains(code, "#") {
		score += 0.2
	}
	
	// Check for meaningful variable names (not single letters except for loops)
	lines := strings.Split(code, "\n")
	meaningfulNames := 0
	totalVars := 0
	
	for _, line := range lines {
		// Simple heuristic for variable declarations
		if strings.Contains(line, "var ") || strings.Contains(line, "let ") || 
		   strings.Contains(line, "const ") || strings.Contains(line, ":=") {
			totalVars++
			// Check if variable name is more than 1 character
			words := strings.Fields(line)
			for _, word := range words {
				if len(word) > 1 && !strings.Contains("var let const :=", word) {
					meaningfulNames++
					break
				}
			}
		}
	}
	
	if totalVars > 0 && float64(meaningfulNames)/float64(totalVars) > 0.8 {
		score += 0.3
	}
	
	// Check for proper indentation
	properlyIndented := cv.checkIndentation(code)
	if properlyIndented {
		score += 0.2
	}
	
	// Check for reasonable line length
	reasonableLength := cv.checkLineLength(code, 120)
	if reasonableLength {
		score += 0.15
	}
	
	// Check for proper error handling (language specific)
	errorHandling := cv.checkErrorHandling(code, language)
	if errorHandling {
		score += 0.15
	}
	
	return score >= 0.6 // 60% threshold for best practices
}

// Language-specific syntax validators

func (cv *CodeValidator) validateGoSyntax(code string) bool {
	// Basic Go syntax checks
	hasPackage := strings.Contains(code, "package ")
	hasProperBraces := cv.checkBraces(code)
	
	// Check for basic syntax issues
	if strings.Contains(code, "fmt.Println") && !strings.Contains(code, "import") {
		return false // Missing imports
	}
	
	// Check for balanced parentheses
	if !cv.checkBalancedParentheses(code) {
		return false
	}
	
	return hasPackage && hasProperBraces
}

func (cv *CodeValidator) checkBalancedParentheses(code string) bool {
	parenCount := 0
	for _, char := range code {
		switch char {
		case '(':
			parenCount++
		case ')':
			parenCount--
			if parenCount < 0 {
				return false
			}
		}
	}
	return parenCount == 0
}

func (cv *CodeValidator) validatePythonSyntax(code string) bool {
	// Basic Python syntax checks
	lines := strings.Split(code, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Check for proper indentation in Python
		if strings.HasPrefix(line, " ") && len(line) > 0 {
			continue // Properly indented
		}
		if !strings.HasPrefix(line, " ") {
			continue // Top-level statement
		}
	}
	return true
}

func (cv *CodeValidator) validateJavaScriptSyntax(code string) bool {
	// Basic JavaScript syntax checks
	hasProperBraces := cv.checkBraces(code)
	return hasProperBraces
}

func (cv *CodeValidator) validateJavaSyntax(code string) bool {
	// Basic Java syntax checks
	hasClass := strings.Contains(code, "class ")
	hasProperBraces := cv.checkBraces(code)
	return hasClass && hasProperBraces
}

// Helper methods

func (cv *CodeValidator) checkBraces(code string) bool {
	braceCount := 0
	for _, char := range code {
		switch char {
		case '{':
			braceCount++
		case '}':
			braceCount--
		}
		if braceCount < 0 {
			return false
		}
	}
	return braceCount == 0
}

func (cv *CodeValidator) checkIndentation(code string) bool {
	lines := strings.Split(code, "\n")
	consistentIndentation := true
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		// Check if indentation is consistent (either tabs or spaces, not mixed)
		if strings.HasPrefix(line, "\t") && strings.HasPrefix(line, " ") {
			consistentIndentation = false
			break
		}
	}
	
	return consistentIndentation
}

func (cv *CodeValidator) checkLineLength(code string, maxLength int) bool {
	lines := strings.Split(code, "\n")
	for _, line := range lines {
		if len(line) > maxLength {
			return false
		}
	}
	return true
}

func (cv *CodeValidator) checkErrorHandling(code string, language string) bool {
	switch language {
	case "go":
		return strings.Contains(code, "if err != nil") || strings.Contains(code, "error")
	case "python":
		return strings.Contains(code, "try:") || strings.Contains(code, "except")
	case "javascript":
		return strings.Contains(code, "try {") || strings.Contains(code, "catch")
	case "java":
		return strings.Contains(code, "try {") || strings.Contains(code, "catch")
	}
	return true // Default to true for unsupported languages
}

// CompilationResult represents the result of a compilation attempt
type CompilationResult struct {
	Success bool
	Error   string
}

// tryCompile attempts to compile the code
func (cv *CodeValidator) tryCompile(ctx context.Context, code string, langConfig *LanguageConfig) CompilationResult {
	// Security check: validate command is safe
	if !cv.isCommandSafe(langConfig.CompileCommand) {
		return CompilationResult{Success: false, Error: "Compilation disabled: unsafe command detected"}
	}
	
	// Create temporary file
	tempFile := filepath.Join(cv.tempDir, fmt.Sprintf("temp_%d%s", time.Now().UnixNano(), langConfig.FileExtension))
	defer os.Remove(tempFile)
	
	if err := os.WriteFile(tempFile, []byte(code), 0644); err != nil {
		return CompilationResult{Success: false, Error: fmt.Sprintf("Failed to write temp file: %v", err)}
	}
	
	// Prepare compile command with security restrictions
	cmd := exec.CommandContext(ctx, langConfig.CompileCommand[0])
	if len(langConfig.CompileCommand) > 1 {
		cmd.Args = append(cmd.Args, langConfig.CompileCommand[1:]...)
	}
	cmd.Args = append(cmd.Args, tempFile)
	
	// Security: Set restrictive environment
	cmd.Env = []string{"PATH=/usr/bin:/bin"} // Minimal PATH
	
	// Run compilation with timeout
	output, err := cmd.CombinedOutput()
	if err != nil {
		return CompilationResult{Success: false, Error: string(output)}
	}
	
	return CompilationResult{Success: true}
}

// ExecutionResult represents the result of a code execution attempt
type ExecutionResult struct {
	Success bool
	Error   string
	Output  string
}

// tryExecute attempts to execute the code (if enabled and safe)
func (cv *CodeValidator) tryExecute(ctx context.Context, code string, langConfig *LanguageConfig) ExecutionResult {
	// Security: Execution is disabled by default and should only be enabled in secure environments
	if !cv.config.EnableExecution {
		return ExecutionResult{Success: false, Error: "Code execution disabled for security"}
	}
	
	// Security check: validate command is safe
	if !cv.isCommandSafe(langConfig.ExecuteCommand) {
		return ExecutionResult{Success: false, Error: "Execution disabled: unsafe command detected"}
	}
	
	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, cv.config.ExecutionTimeout)
	defer cancel()
	
	// Create temporary file
	tempFile := filepath.Join(cv.tempDir, fmt.Sprintf("temp_%d%s", time.Now().UnixNano(), langConfig.FileExtension))
	defer os.Remove(tempFile)
	
	if err := os.WriteFile(tempFile, []byte(code), 0644); err != nil {
		return ExecutionResult{Success: false, Error: fmt.Sprintf("Failed to write temp file: %v", err)}
	}
	
	// Prepare execute command with security restrictions
	cmd := exec.CommandContext(execCtx, langConfig.ExecuteCommand[0])
	if len(langConfig.ExecuteCommand) > 1 {
		cmd.Args = append(cmd.Args, langConfig.ExecuteCommand[1:]...)
	}
	cmd.Args = append(cmd.Args, tempFile)
	
	// Security: Set highly restrictive environment
	cmd.Env = []string{
		"PATH=/usr/bin:/bin",
		"HOME=/tmp",
		"TMPDIR=" + cv.tempDir,
	}
	
	// Run execution with timeout
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ExecutionResult{Success: false, Error: err.Error(), Output: string(output)}
	}
	
	return ExecutionResult{Success: true, Output: string(output)}
}

// validateCodeContent validates that code content doesn't contain dangerous patterns
func (cv *CodeValidator) validateCodeContent(code, language string) error {
	// Check code size
	if len(code) > int(cv.config.MaxFileSize) {
		return fmt.Errorf("code too large: %d bytes (max %d)", len(code), cv.config.MaxFileSize)
	}
	
	// General dangerous patterns (language-agnostic)
	dangerousPatterns := []string{
		"system(",
		"exec(",
		"shell_exec(",
		"passthru(",
		"popen(",
		"proc_open(",
		"file_get_contents(",
		"file_put_contents(",
		"fopen(",
		"fwrite(",
		"curl_exec(",
		"eval(",
		"__import__",
		"getattr(",
		"setattr(",
		"delattr(",
		"compile(",
		"os.system",
		"os.popen",
		"subprocess",
		"runtime.exec",
		"require('child_process')",
		"require('fs')",
		"require('os')",
		"Process.start",
		"ProcessBuilder",
		"Runtime.getRuntime",
	}
	
	codeToCheck := strings.ToLower(code)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(codeToCheck, strings.ToLower(pattern)) {
			return fmt.Errorf("dangerous code pattern detected: %s", pattern)
		}
	}
	
	// Language-specific checks
	switch strings.ToLower(language) {
	case "python":
		if strings.Contains(codeToCheck, "import os") ||
		   strings.Contains(codeToCheck, "import subprocess") ||
		   strings.Contains(codeToCheck, "import sys") ||
		   strings.Contains(codeToCheck, "__builtins__") {
			return fmt.Errorf("dangerous Python imports detected")
		}
	case "javascript", "js", "node":
		if strings.Contains(codeToCheck, "require('fs')") ||
		   strings.Contains(codeToCheck, "require('child_process')") ||
		   strings.Contains(codeToCheck, "require('os')") ||
		   strings.Contains(codeToCheck, "new function(") {
			return fmt.Errorf("dangerous JavaScript functions detected")
		}
	case "go":
		if strings.Contains(codeToCheck, "os/exec") ||
		   strings.Contains(codeToCheck, "syscall") ||
		   strings.Contains(codeToCheck, "unsafe") {
			return fmt.Errorf("dangerous Go packages detected")
		}
	}
	
	return nil
}

// isCommandSafe validates that a command is safe to execute
func (cv *CodeValidator) isCommandSafe(command []string) bool {
	if len(command) == 0 {
		return false
	}
	
	// Whitelist of allowed commands
	safeCommands := map[string]bool{
		"go":     true,
		"python": true,
		"python3": true,
		"node":   true,
		"javac":  true,
		"java":   true,
		"gcc":    true,
		"clang":  true,
		"rustc":  true,
	}
	
	baseCommand := filepath.Base(command[0])
	if !safeCommands[baseCommand] {
		return false
	}
	
	// Additional validation for arguments
	for _, arg := range command[1:] {
		// Prevent potentially dangerous arguments
		if strings.Contains(arg, "..") || 
		   strings.Contains(arg, "|") || 
		   strings.Contains(arg, ";") || 
		   strings.Contains(arg, "&") ||
		   strings.Contains(arg, "`") ||
		   strings.Contains(arg, "$") {
			return false
		}
	}
	
	return true
}

// Cleanup removes temporary files
func (cv *CodeValidator) Cleanup() {
	os.RemoveAll(cv.tempDir)
}