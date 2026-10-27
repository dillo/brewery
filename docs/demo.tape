# brewery's demo video, before its captions. `make demo` builds the demo
# binary (a test under the demo tag, see internal/ui/demo_test.go), records
# it here, then lays the captions it logged under the video. The demo drives
# itself, so this only starts it and waits for it to quit, which stops the
# recording before the shell shows again.

Output bin/demo-raw.mp4

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
Type "cd internal/ui && clear && DEMO_CAPTIONS=../../bin/captions.tsv ../../bin/brewery-demo -test.run '^TestDemo$'"
Enter
Sleep 300ms
Show
Wait+Screen@120s /PASS/
