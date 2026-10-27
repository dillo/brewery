# brewery's demo video. `make demo` builds the demo binary (a test under the
# demo tag, see internal/ui/demo_test.go) and records it here. The demo
# drives itself, so this only starts it, waits for the closing caption and
# holds on it. The recording stops before the demo quits.

Output docs/demo.mp4

Set Shell zsh
Set FontFamily "JetBrainsMonoNL Nerd Font"
Set FontSize 15
Set Width 1600
Set Height 1000
Set Padding 24
Set Theme "Catppuccin Macchiato"
Set Framerate 30
Set TypingSpeed 0

Hide
Type "cd internal/ui && clear && ../../bin/brewery-demo -test.run '^TestDemo$'"
Enter
Sleep 300ms
Show
Wait+Screen@120s /github.com\/dillo\/brewery/
Sleep 4s
