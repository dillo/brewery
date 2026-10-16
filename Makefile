.PHONY: build test test-live lint snapshot screenshots run

build:
	go build -o brewery ./cmd/brewery

test:
	go test ./...

test-live:
	go test -tags live -count=1 ./internal/brew/

# golangci-lint v2: brew install golangci-lint
lint:
	golangci-lint run ./...

# Builds the release locally into dist/ without publishing: brew install goreleaser
snapshot:
	goreleaser release --snapshot --clean

# The README's screenshots, from a made-up machine (internal/ui/testdata/demo).
# Needs Charm's freeze (go install github.com/charmbracelet/freeze@v0.2.2; brew's
# "freeze" is a different app) and the JetBrains Mono Nerd Font, whose NL
# (no ligatures) cut shows brew's ==> as typed:
# brew install --cask font-jetbrains-mono-nerd-font
FREEZE ?= freeze
FONT ?= $(HOME)/Library/Fonts/JetBrainsMonoNLNerdFont-Regular.ttf
screenshots:
	go test -tags screenshots -count=1 -run TestScreenshots ./internal/ui
	for f in docs/screenshots/*.ansi; do \
		$(FREEZE) "$$f" --language ansi --window --background '#24273a' \
			--padding 20,24 --margin 0 --border.radius 10 \
			--font.family 'JetBrainsMonoNL Nerd Font' --font.file "$(FONT)" \
			--font.size 14 --line-height 1.2 -o "$${f%.ansi}.png" </dev/null || exit 1; \
	done

run: build
	./brewery
