.PHONY: build test test-live fuzz cover lint vuln snapshot screenshots demo run

build:
	go build -o brewery ./cmd/brewery

test:
	go test ./...

test-live:
	go test -tags live -count=1 ./internal/brew/

# Fuzzes the command line's tokenizer; a failing input lands in
# internal/ui/testdata/fuzz, where go test then replays it.
FUZZTIME ?= 1m
fuzz:
	go test -run='^$$' -fuzz='^FuzzSplitArgs$$' -fuzztime=$(FUZZTIME) ./internal/ui/

# Each package's statement coverage, then the total.
cover:
	go test -count=1 -coverprofile=cover.out ./... && go tool cover -func=cover.out | tail -1 && rm -f cover.out

# golangci-lint v2: brew install golangci-lint
lint:
	golangci-lint run ./...

# Known vulnerabilities the code reaches, in dependencies and the standard library.
vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

# Builds the release locally into dist/ without publishing or signing:
# brew install goreleaser
snapshot:
	goreleaser release --snapshot --clean --skip=sign

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

# The demo video and the README's GIF of it, from the same made-up machine,
# driven by a script (internal/ui/demo_test.go). VHS records it, Swift draws
# the captions (docs/captions.swift) and ffmpeg lays them under the video.
# Needs VHS and ffmpeg (brew install vhs ffmpeg), Xcode's command line tools
# for swift, and the font the screenshots use.
CAPTION_FONT ?= $(HOME)/Library/Fonts/JetBrainsMonoNLNerdFont-Bold.ttf
demo:
	go test -c -tags demo -o bin/brewery-demo ./internal/ui
	vhs docs/demo.tape
	rm -rf bin/captions
	swift docs/captions.swift bin/captions.tsv bin/captions \
		"$$(ffprobe -v error -select_streams v:0 -show_entries stream=width -of csv=p=0 bin/demo-raw.mp4)" \
		"$$(ffprobe -v error -show_entries format=duration -of csv=p=0 bin/demo-raw.mp4)" "$(CAPTION_FONT)"
	ffmpeg -v error -y -i bin/demo-raw.mp4 $$(for f in bin/captions/*.png; do printf -- '-i %s ' "$$f"; done) \
		-/filter_complex bin/captions/filter.txt -map '[out]' \
		-c:v libx264 -pix_fmt yuv420p -crf 20 -movflags +faststart docs/demo.mp4
	ffmpeg -v error -y -i docs/demo.mp4 -vf "fps=12,scale=1200:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=64:stats_mode=diff[p];[b][p]paletteuse=dither=none:diff_mode=rectangle" docs/demo.gif

run: build
	./brewery
