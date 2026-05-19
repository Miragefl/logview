#!/bin/bash
set -euo pipefail

# nightly.sh - build, upload to GitHub nightly pre-release, update homebrew formula
# Usage: ./scripts/nightly.sh

VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u '+%Y-%m-%d')
TIME=$(date -u '+%H%M%S')
BUILT=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
FORMULA_VERSION="${VERSION#v}-nightly-${DATE//-/}-${TIME}"
LDFLAGS="-s -w -X main.version=${VERSION}-nightly -X main.commit=${COMMIT} -X main.date=${BUILT}"

echo "=== Building nightly ${VERSION} (${COMMIT}) ==="

# cross-compile
GOOS=darwin GOARCH=amd64 go build -ldflags "$LDFLAGS" -o logview-darwin_amd64 .
GOOS=darwin GOARCH=arm64 go build -ldflags "$LDFLAGS" -o logview-darwin_arm64 .
GOOS=linux GOARCH=amd64 go build -ldflags "$LDFLAGS" -o logview-linux_amd64 .
GOOS=linux GOARCH=arm64 go build -ldflags "$LDFLAGS" -o logview-linux_arm64 .

# verify
./logview-darwin_arm64 version

# package
tar czf logview-darwin_amd64.tar.gz logview-darwin_amd64
tar czf logview-darwin_arm64.tar.gz logview-darwin_arm64
tar czf logview-linux_amd64.tar.gz logview-linux_amd64
tar czf logview-linux_arm64.tar.gz logview-linux_arm64

# compute sha256
SHA_DARWIN_AMD=$(sha256sum logview-darwin_amd64.tar.gz | cut -d' ' -f1)
SHA_DARWIN_ARM=$(sha256sum logview-darwin_arm64.tar.gz | cut -d' ' -f1)
SHA_LINUX_AMD=$(sha256sum logview-linux_amd64.tar.gz | cut -d' ' -f1)
SHA_LINUX_ARM=$(sha256sum logview-linux_arm64.tar.gz | cut -d' ' -f1)

echo "=== SHA256 ==="
echo "darwin_amd64: $SHA_DARWIN_AMD"
echo "darwin_arm64: $SHA_DARWIN_ARM"
echo "linux_amd64:  $SHA_LINUX_AMD"
echo "linux_arm64:  $SHA_LINUX_ARM"

# delete old nightly release if exists, then create new one
gh release delete nightly --yes 2>/dev/null || true
gh release create nightly \
  logview-darwin_amd64.tar.gz \
  logview-darwin_arm64.tar.gz \
  logview-linux_amd64.tar.gz \
  logview-linux_arm64.tar.gz \
  --title "Nightly (${DATE})" \
  --notes "Automated nightly build from ${COMMIT}" \
  --prerelease

echo "=== Updating homebrew formula ==="

FORMULA_DIR="${HOME}/.cache/logview-nightly-formula"
FORMULA_REPO="${FORMULA_DIR}/homebrew-logview"
TAP_URL="https://github.com/Miragefl/homebrew-logview.git"

if [ ! -d "$FORMULA_REPO" ]; then
  mkdir -p "$FORMULA_DIR"
  git clone "$TAP_URL" "$FORMULA_REPO"
fi

cd "$FORMULA_REPO"
git pull origin master

# update formula
cat > Formula/logview-nightly.rb << RUBY
# typed: false
# frozen_string_literal: true

class LogviewNightly < Formula
  desc "Terminal log viewer (nightly build)"
  homepage "https://github.com/Miragefl/logview"
  version "${FORMULA_VERSION}"

  on_macos do
    if Hardware::CPU.intel?
      url "https://github.com/Miragefl/logview/releases/download/nightly/logview-darwin_amd64.tar.gz"
      sha256 "${SHA_DARWIN_AMD}"

      define_method(:install) do
        bin.install "logview-darwin_amd64" => "logview"
      end
    end
    if Hardware::CPU.arm?
      url "https://github.com/Miragefl/logview/releases/download/nightly/logview-darwin_arm64.tar.gz"
      sha256 "${SHA_DARWIN_ARM}"

      define_method(:install) do
        bin.install "logview-darwin_arm64" => "logview"
      end
    end
  end

  on_linux do
    if Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/Miragefl/logview/releases/download/nightly/logview-linux_amd64.tar.gz"
      sha256 "${SHA_LINUX_AMD}"
      define_method(:install) do
        bin.install "logview-linux_amd64" => "logview"
      end
    end
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/Miragefl/logview/releases/download/nightly/logview-linux_arm64.tar.gz"
      sha256 "${SHA_LINUX_ARM}"
      define_method(:install) do
        bin.install "logview-linux_arm64" => "logview"
      end
    end
  end

  test do
    system "#{bin}/logview", "--help"
  end
end
RUBY

git add Formula/logview-nightly.rb
git commit -m "nightly: ${FORMULA_VERSION}"
git push origin master

echo "=== Done! ==="
echo "Install: brew install Miragefl/logview/logview-nightly"
echo "Stable:  brew install logview (or logview-nightly, both coexist)"

cd -

# cleanup
rm -f logview-darwin_amd64 logview-darwin_arm64 logview-linux_amd64 logview-linux_arm64
rm -f logview-darwin_amd64.tar.gz logview-darwin_arm64.tar.gz logview-linux_amd64.tar.gz logview-linux_arm64.tar.gz
