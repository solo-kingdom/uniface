# Feature Example: Authentication System

## Description
This prompt is a template for implementing new features in the uniface project. It demonstrates the structure and content expected when requesting feature implementation.

## Context
The uniface project aims to unify interfaces across different systems. As we add new features, we need to maintain consistency in code quality, architecture, and documentation.

## Requirements

### Functional Requirements
1. Implement a JWT-based authentication system
2. Support user registration and login
3. Provide middleware for protecting routes
4. Include token refresh mechanism
5. Store user credentials securely (hashed passwords)

### Technical Requirements
- Use `golang.org/x/crypto/bcrypt` for password hashing
- Use `github.com/golang-jwt/jwt/v5` for JWT tokens
- Follow existing code structure in `internal/core`
- Add appropriate error handling and logging
- Write unit tests with >80% coverage
- Update documentation

### Non-Functional Requirements
- Response time < 100ms for authentication operations
- Secure storage of sensitive data
- Clean and maintainable code
- Proper separation of concerns

## Implementation Details

### Files to Create/Modify
- `internal/core/auth/` - New package for authentication logic
- `internal/core/middleware/auth.go` - Authentication middleware
- `pkg/auth/types.go` - Public auth types and interfaces
- `test/auth_test.go` - Unit tests

### Dependencies
- Ensure compatibility with existing Go 1.21
- Update `go.mod` and `go.sum`

### Success Criteria
- [ ] All unit tests pass
- [ ] Code follows project conventions
- [ ] Documentation is updated
- [ ] No security vulnerabilities
- [ ] Performance benchmarks meet requirements

## Notes
- This is an example template
- Replace content with actual feature requirements when implementing real features
- Reference this structure for consistent feature development prompts