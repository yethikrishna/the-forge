// Package errors provides a unified error handling system for Forge,
// combining structured error codes, teachable error messages, and
// intelligent error explanation into one cohesive package.
//
// It merges the former internal/errcode, internal/errteach, and
// internal/errorexplain packages. Use the sub-packages for specific
// functionality:
//
//   - errors/code: Structured error code catalog (FORGE-E001 through FORGE-E999)
//   - errors/teach: Teachable errors with fix suggestions and docs links
//   - errors/explain: Intelligent error interpretation with root cause analysis
//
// "Something went wrong" is the enemy of adoption. Actionable errors build trust.
package errors
