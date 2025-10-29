package rubyext

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var nativeLibraryExtensions = map[string]struct{}{
	".so":     {},
	".bundle": {},
	".dll":    {},
	".dylib":  {},
}

// finalizeNativeExtensions copies compiled native libraries into the gem's lib directory structure
// and returns their paths relative to the gem root. If no native libraries are present, the original
// build outputs are returned relative to the gem root.
func finalizeNativeExtensions(config *BuildConfig, extensionFile, extensionDir string, built []string) ([]string, error) {
	if len(built) == 0 {
		return nil, nil
	}

	var hasNative bool
	for _, rel := range built {
		if isNativeLibrary(rel) {
			hasNative = true
			break
		}
	}

	if !hasNative {
		return makeGemRelative(config.GemDir, extensionFile, built), nil
	}

	primaryDest, extraDests := installTargets(config)
	if primaryDest == "" {
		return makeGemRelative(config.GemDir, extensionFile, built), nil
	}

	var installed []string

	for _, rel := range built {
		if !isNativeLibrary(rel) {
			continue
		}

		srcPath := filepath.Join(extensionDir, rel)
		if info, err := os.Stat(srcPath); err != nil || !info.Mode().IsRegular() {
			continue
		}

		relDest := determineInstallRelativePath(config.GemDir, extensionFile, rel)
		if relDest == "" {
			relDest = filepath.Base(rel)
		}

		if err := copyFile(srcPath, filepath.Join(primaryDest, relDest)); err != nil {
			return nil, err
		}

		for _, dest := range extraDests {
			if err := copyFile(srcPath, filepath.Join(dest, relDest)); err != nil {
				return nil, err
			}
		}

		if relPath, err := filepath.Rel(config.GemDir, filepath.Join(primaryDest, relDest)); err == nil {
			installed = append(installed, filepath.ToSlash(relPath))
		} else {
			installed = append(installed, filepath.ToSlash(filepath.Join(primaryDest, relDest)))
		}
	}

	return installed, nil
}

func makeGemRelative(gemDir, extensionFile string, built []string) []string {
	var relPaths []string
	baseDir := filepath.Dir(extensionFile)

	for _, rel := range built {
		full := filepath.Join(baseDir, rel)
		if gemDir != "" {
			if cleaned, err := filepath.Rel(gemDir, filepath.Join(gemDir, full)); err == nil {
				relPaths = append(relPaths, filepath.ToSlash(cleaned))
				continue
			}
		}
		relPaths = append(relPaths, filepath.ToSlash(full))
	}

	return relPaths
}

func isNativeLibrary(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := nativeLibraryExtensions[ext]
	return ok
}

func installTargets(config *BuildConfig) (primary string, additional []string) {
	baseDirs := gatherBaseDirectories(config)
	if len(baseDirs) == 0 {
		return "", nil
	}

	versionDir, useVersion := rubyVersionDirectory(config.RubyVersion)

	for i, base := range baseDirs {
		target := base
		if useVersion {
			target = filepath.Join(base, versionDir)
		}

		if i == 0 {
			primary = target
		} else {
			additional = append(additional, target)
		}

		// Also copy to unversioned base for compatibility
		if useVersion {
			additional = append(additional, base)
		}
	}

	additional = uniqueStrings(additional)
	return primary, additional
}

func gatherBaseDirectories(config *BuildConfig) []string {
	var dirs []string

	add := func(dir string) {
		if dir == "" {
			return
		}
		if !filepath.IsAbs(dir) && config.GemDir != "" {
			dir = filepath.Join(config.GemDir, dir)
		}
		dirs = append(dirs, filepath.Clean(dir))
	}

	add(config.DestPath)
	add(config.LibDir)

	if len(dirs) == 0 && config.GemDir != "" {
		add(filepath.Join(config.GemDir, "lib"))
	}

	return uniqueStrings(dirs)
}

func rubyVersionDirectory(version string) (string, bool) {
	major, minor, ok := parseRubyVersion(version)
	if !ok {
		return "", false
	}

	if major > 3 || (major == 3 && minor >= 4) {
		return fmt.Sprintf("%d.%d", major, minor), true
	}

	return "", false
}

func determineInstallRelativePath(gemDir, extensionFile, builtRel string) string {
	suffix := filepath.Ext(builtRel)
	baseName := strings.TrimSuffix(filepath.Base(builtRel), suffix)

	if module := moduleFromCreateMakefile(gemDir, extensionFile); module != "" {
		modulePath := filepath.FromSlash(module)
		if suffix != "" && !strings.HasSuffix(modulePath, suffix) {
			modulePath += suffix
		}
		return safeRelativePath(modulePath)
	}

	if strings.HasSuffix(extensionFile, "extconf.rb") {
		relPath := strings.TrimPrefix(extensionFile, "ext/")
		relPath = strings.TrimSuffix(relPath, "/extconf.rb")
		relPath = strings.TrimSuffix(relPath, filepath.Ext(relPath))
		relPath = strings.Trim(relPath, "/\\")

		if relPath != "" && !strings.HasSuffix(relPath, baseName) {
			relPath = filepath.Join(relPath, baseName)
		}

		if relPath == "" {
			relPath = baseName
		}

		if suffix != "" && !strings.HasSuffix(relPath, suffix) {
			relPath += suffix
		}

		return safeRelativePath(relPath)
	}

	relDir := strings.TrimPrefix(filepath.Dir(extensionFile), "ext/")
	if relDir == "" {
		relDir = baseName
	} else if !strings.HasSuffix(relDir, baseName) {
		relDir = filepath.Join(relDir, baseName)
	}

	if suffix != "" && !strings.HasSuffix(relDir, suffix) {
		relDir += suffix
	}

	return safeRelativePath(relDir)
}

func moduleFromCreateMakefile(gemDir, extensionFile string) string {
	if !strings.HasSuffix(extensionFile, "extconf.rb") {
		return ""
	}

	extconfPath := filepath.Join(gemDir, extensionFile)
	content, err := os.ReadFile(extconfPath)
	if err != nil {
		return ""
	}

	patterns := []string{
		`create_makefile\s*\(\s*['"]([^'"]+)['"]`,
		`create_makefile\s+['"]([^'"]+)['"]`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(string(content)); len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

func copyFile(srcPath, destPath string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	dir := filepath.Dir(destPath)
	if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
		return mkErr
	}

	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}

	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	return out.Close()
}

func safeRelativePath(path string) string {
	clean := filepath.Clean(path)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return filepath.Base(path)
	}
	return clean
}

func parseRubyVersion(version string) (major, minor int, ok bool) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, 0, false
	}

	var err error
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, false
	}

	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, false
	}

	return major, minor, true
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{})
	var result []string

	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}
