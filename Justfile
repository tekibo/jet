# ==============================================================================
#  Justfile for jet CLI development
# ==============================================================================

# Show this comprehensive list of recipes
default:
    @just --list --unsorted

# Compile the 'jet' binary locally
build:
    go build -ldflags="-s -w" -o jet .
    @echo "✓ Built ./jet binary"

# Compile and install 'jet' to ~/.local/bin
install:
    mkdir -p ~/.local/bin
    go build -ldflags="-s -w" -o ~/.local/bin/jet .
    @echo "✓ Installed 'jet' to ~/.local/bin"

# Run Android environment diagnostics
doctor: build
    ./jet doctor

# Scaffold and verify dry-run build in /tmp/test-app
test-init: build
    rm -rf /tmp/test-app
    ./jet init /tmp/test-app --name "Test App" --package "com.test.app"
    cd /tmp/test-app && ./gradlew --dry-run assembleDebug
    @echo "✓ Test scaffolding succeeded!"

# Cross-compile release binaries for Linux, macOS, and Windows
dist:
    mkdir -p dist
    GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/jet-linux-amd64 .
    GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/jet-linux-arm64 .
    GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/jet-darwin-amd64 .
    GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/jet-darwin-arm64 .
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o dist/jet-windows-amd64.exe .
    @echo "✓ Cross-platform binaries built in dist/"

# Remove all local build artifacts
clean:
    rm -f jet
    rm -rf dist/
    rm -rf /tmp/test-app
    @echo "✓ Cleaned build artifacts"
