# ruby-extension-go

> Build Ruby native extensions in pure Go - no Ruby required during installation!

[![CI](https://github.com/contriboss/ruby-extension-go/actions/workflows/ci.yml/badge.svg)](https://github.com/contriboss/ruby-extension-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/contriboss/ruby-extension-go.svg)](https://pkg.go.dev/github.com/contriboss/ruby-extension-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/contriboss/ruby-extension-go)](https://goreportcard.com/report/github.com/contriboss/ruby-extension-go)

## Overview

This library provides native extension compilation support for Ruby gems in pure Go. It supports multiple build systems commonly used by Ruby gems, making it possible to build native extensions without requiring Ruby during the installation phase.

Ruby equivalent: `Gem::Ext::Builder`

## Supported Build Systems

- **extconf.rb** - The most common Ruby extension build system
- **Rakefile** - Rake-based builds (rake compile)
- **CMake** - CMake-based extensions
- **Cargo** - Rust-based Ruby extensions
- **configure** - Autotools-style configure scripts

## Why This Exists

Part of the [ORE](https://github.com/contriboss/ore) ecosystem, this library enables:
- Building Ruby native extensions without Ruby installed
- Cross-compilation support
- Faster parallel builds
- Better error reporting
- Consistent build environment

## Quick Start

```bash
go get github.com/contriboss/ruby-extension-go
```

## Usage

### Basic Extension Building

```go
package main

import (
    "context"
    "log"

    rubyext "github.com/contriboss/ruby-extension-go"
)

func main() {
    // Create builder factory
    factory := rubyext.NewBuilderFactory()

    // Configure build
    config := &rubyext.BuildConfig{
        GemDir:       "/path/to/extracted/gem",
        ExtensionDir: "/path/to/ext",
        DestPath:     "/path/to/install",
        RubyPath:     "/usr/bin/ruby",
        RubyVersion:  "3.4.0",
        Verbose:      true,
    }

    // Build all extensions
    extensions := []string{"ext/myext/extconf.rb"}
    results, err := factory.BuildAllExtensions(context.Background(), config, extensions)
    if err != nil {
        log.Fatal(err)
    }

    for _, result := range results {
        if result.Success {
            log.Printf("Built: %v", result.Extensions)
        } else {
            log.Printf("Failed: %v", result.Error)
        }
    }
}
```

### Building Specific Extension Types

```go
// For extconf.rb extensions
builder := &rubyext.ExtConfBuilder{}
if builder.CanBuild("extconf.rb") {
    result, err := builder.Build(ctx, config, "ext/myext/extconf.rb")
}

// For Rust extensions
cargoBuilder := &rubyext.CargoBuilder{}
if cargoBuilder.CanBuild("Cargo.toml") {
    result, err := cargoBuilder.Build(ctx, config, "ext/rust_ext/Cargo.toml")
}
```

## Build Systems Details

### ExtConf Builder
Handles traditional Ruby C extensions using `extconf.rb`:
- Generates Makefile via Ruby
- Supports custom build flags
- Handles platform-specific configurations

### Rake Builder
For gems using Rake for compilation:
- Executes `rake compile`
- Supports custom rake tasks
- Handles multi-stage builds

### CMake Builder
For modern C++ extensions:
- Cross-platform support
- Out-of-source builds
- Advanced dependency management

### Cargo Builder
For Rust-based extensions:
- Handles Cargo.toml projects
- Supports release/debug builds
- Manages Rust toolchain

### Configure Builder
For Autotools-based extensions:
- Standard ./configure && make
- Cross-compilation support
- Platform detection

## Features

- **Parallel Builds** - Use multiple CPU cores for faster compilation
- **Cross-Compilation** - Build for different architectures
- **Build Caching** - Avoid rebuilding unchanged extensions
- **Error Recovery** - Detailed error messages for debugging
- **Platform Detection** - Automatic platform-specific adjustments

## Architecture

```
ORE
 ↓
ruby-extension-go
 ↓
BuilderFactory
 ↓
├── ExtConfBuilder
├── RakeBuilder
├── CMakeBuilder
├── CargoBuilder
└── ConfigureBuilder
```

## Building

We use [Mage](https://magefile.org) for builds:

```bash
# Install Mage
go install github.com/magefile/mage@latest

# Run tests
mage test

# Run linter
mage lint

# Build
mage build

# Run CI checks
mage ci
```

## Testing

```bash
# Run tests with coverage
mage test

# Run tests with race detector
mage testrace

# Run integration tests
go test -tags=integration ./...
```

## Performance

- Parallel compilation with make -j
- Efficient resource usage
- Build output caching
- Minimal overhead vs native builds

## Platform Support

- **Linux** - Full support for all build systems
- **macOS** - Full support, handles SDK paths
- **Windows** - Limited support (MinGW/MSYS2)
- **Cross-compilation** - Via proper toolchain configuration

## License

MIT

## Related Projects

- [ORE](https://github.com/contriboss/ore) - The fast Ruby gem installer
- [gemfile-go](https://github.com/contriboss/gemfile-go) - Parse Gemfile and Gemfile.lock
- [rubygems-client-go](https://github.com/contriboss/rubygems-client-go) - RubyGems.org API client

---

Made by [@contriboss](https://github.com/contriboss) - Building the future of Ruby dependency management