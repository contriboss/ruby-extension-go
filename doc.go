// Package rubyext provides native extension compilation support for Ruby gems.
//
// This package is the Go equivalent of Ruby's Gem::Ext::Builder and supports
// multiple build systems commonly used by Ruby native extensions.
//
// # Supported Build Systems
//
// The package includes builders for:
//   - extconf.rb - Traditional Ruby C extensions (most common)
//   - Rakefile - Rake-based compilation workflows
//   - CMakeLists.txt - CMake-based C/C++ extensions
//   - Cargo.toml - Rust-based extensions via Cargo
//   - configure - Autotools-style configure scripts
//
// # Basic Usage
//
// Create a builder factory and use it to build extensions:
//
//	factory := rubyext.NewBuilderFactory()
//
//	config := &rubyext.BuildConfig{
//	    GemDir:       "/path/to/gem",
//	    ExtensionDir: "/path/to/ext",
//	    DestPath:     "/path/to/install",
//	    RubyPath:     "/usr/bin/ruby",
//	    Verbose:      true,
//	}
//
//	extensions := []string{"ext/myext/extconf.rb"}
//	results, err := factory.BuildAllExtensions(ctx, config, extensions)
//
// # Architecture
//
// The package uses a factory pattern with registered builders:
//
//	BuilderFactory
//	├── ExtConfBuilder (extconf.rb)
//	├── RakeBuilder (Rakefile, mkrf_conf.rb)
//	├── CMakeBuilder (CMakeLists.txt)
//	├── CargoBuilder (Cargo.toml)
//	└── ConfigureBuilder (configure, configure.sh)
//
// Each builder implements the Builder interface and can:
//   - Detect if it can handle a given extension file
//   - Build the extension with proper error handling
//   - Clean build artifacts
//
// # Requirements
//
// Requires Go 1.25 or later.
//
// # Platform Support
//
// Full support on Linux and macOS. Limited Windows support (MinGW/MSYS2).
// Cross-compilation is supported with proper toolchain configuration.
package rubyext
