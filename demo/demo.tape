Set FontSize 32
Set Width 2400
Set Height 2120
Set Theme TokyoNight
Set Padding 48
Set WindowBar Colorful
Set WindowBarSize 64
Set BorderRadius 24

Type "git diff v0.1.0..v0.2.0 | ./flak"
Enter
Sleep 2s

# Jump to ui.go (second file)
Type "]"
Sleep 1s

# Search for the new help mode constant
Type "/"
Sleep 0.4s
Type "modeHelp"
Enter
Sleep 1.2s

# Scroll up to modeEdit for first comment
Type "k"
Sleep 0.4s
Type "k"
Sleep 0.4s
Type "k"
Sleep 0.8s

# First comment on modeEdit
Type "c"
Sleep 0.8s
Type "modeEdit and modeComment both use the textarea — consider unifying them with a flag for new vs existing."
Sleep 1.5s
Ctrl+S
Sleep 1.2s

# Scroll down to modeHelp (comment line inserted, so 3 steps)
Type "j"
Sleep 0.4s
Type "j"
Sleep 0.4s
Type "j"
Sleep 0.8s

# Second comment on modeHelp
Type "c"
Sleep 0.8s
Type "Add ? as a dismiss binding inside the help panel too, symmetric with how it opens."
Sleep 10s

Screenshot demo/demo.png
