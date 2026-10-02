# Homebrew formula for WP Performance Tools (wp-perf).
#
# This is a binary formula — it installs pre-built release binaries rather
# than compiling from source. No Go toolchain or Xcode CLT required.
#
# Published in the 'homebrew-tap' repository under the Aura-Plugins GitHub
# organisation. Users install via:
#
#   brew tap aura-plugins/tap
#   brew install wp-perf
#
# On every release:
#   1. Update `version` to the new tag (without the leading "v").
#   2. Update the sha256 values below from the release's checksums.txt:
#      gh release download vX.Y.Z -R Aura-Plugins/wp-performance-tools -p checksums.txt -O -
#   3. Copy this file to Formula/wp-perf.rb in Aura-Plugins/homebrew-tap.
#
class WpPerf < Formula
  desc "Measure why a WordPress site is slow, via WP-CLI, with nothing installed"
  homepage "https://github.com/Aura-Plugins/wp-performance-tools"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/Aura-Plugins/wp-performance-tools/releases/download/v#{version}/wp-perf-darwin-arm64"
      sha256 "964d69c71f14413734029d4be74e84c2b713cc3773c03e862703f4855f011aa9"
    end

    on_intel do
      url "https://github.com/Aura-Plugins/wp-performance-tools/releases/download/v#{version}/wp-perf-darwin-amd64"
      sha256 "d17ed67fa252fd303c8ffe35b5adf3f6b60b4ee156249bbc2168eafacd83eaa9"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/Aura-Plugins/wp-performance-tools/releases/download/v#{version}/wp-perf-linux-arm64"
      sha256 "97264b93b9577a94fbc902d957aba331fbd1b4306b40e7866847bc8333ad2fbc"
    end

    on_intel do
      url "https://github.com/Aura-Plugins/wp-performance-tools/releases/download/v#{version}/wp-perf-linux-amd64"
      sha256 "e14dde3e18a499554f4e4088f892bbbbd37534e56ba99f4c2b39cc2e584dd3f9"
    end
  end

  def install
    bin.install Dir["wp-perf-*"].first => "wp-perf"
  end

  test do
    assert_match "wp-perf v#{version}", shell_output("#{bin}/wp-perf version")
  end
end
