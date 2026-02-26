# Initial Setup and Architecture

## Context
This prompt defines the initial architecture and setup for the Unify Interface project.

## Requirements

### Project Goals
- Create a unified interface system that can consolidate multiple API/SDK interfaces
- Provide a consistent, type-safe interface layer
- Support easy integration of new interface types
- Enable efficient code generation and interface management

### Architecture Principles
1. **Modularity**: Each component should be independently testable and maintainable
2. **Type Safety**: Leverage Go's strong typing for interface definitions
3. **Extensibility**: Design patterns that allow easy addition of new interface types
4. **Performance**: Minimal overhead for interface operations

### Core Components

#### 1. Interface Definition Layer
- Location: `internal/core/interface.go`
- Define base interfaces and types
- Support for method signatures, parameters, and return types

#### 2. Registry System
- Location: `internal/core/registry.go`
- Register and manage interface definitions
- Support for interface discovery and lookup

#### 3. Generator System
- Location: `internal/core/generator.go`
- Generate Go code from interface definitions
- Support for various output formats and templates

#### 4. Validation System
- Location: `internal/core/validator.go`
- Validate interface definitions
- Check for type consistency and method compatibility

### Directory Structure Reference
```
uniface/
├── cmd/uniface/          # CLI application
├── internal/
│   ├── core/            # Core business logic
│   ├── parser/          # Parse interface definitions
│   └── generator/       # Code generation
├── pkg/                 # Public APIs
├── prompts/             # AI prompts (this file)
├── docs/                # Documentation
└── test/                # Test files
```

## Dependencies
- Go 1.21 or higher
- Standard library packages
- Consider adding: go/parser for parsing Go code

## Next Steps
1. Implement the core interface types
2. Create the registry system
3. Build the basic CLI commands
4. Add comprehensive tests
5. Write documentation