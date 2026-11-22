package main

const sessionEntry = `[Desktop Entry]
Name=Umbriel
Comment=A Wayland compositor with scrolling, dwindle and master layouts
Exec=/usr/bin/start-umbriel
Type=Application
DesktopNames=Umbriel`

func runSession() error {
	_, err := stdout.WriteString(sessionEntry + "\n")
	return err
}
