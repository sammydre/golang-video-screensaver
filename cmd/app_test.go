package main

import "testing"

func TestParseCommandLineArgs(t *testing.T) {
	var invalidArgs = [][]string{
		[]string{},
		[]string{"-a", "0"},
		[]string{"/a", "ab0099"},
		[]string{"-a"},
		[]string{"/a"},
		[]string{"/p", "not a number"},
	}

	for _, args := range invalidArgs {
		if parseCommandLineArgs(args).ctype != InvalidCommand {
			t.Errorf("Testing '%v' does not return invalid command", args)
		}

	}

	if parseCommandLineArgs([]string{"/s"}).ctype != RunScreenSaver {
		t.Error("RunScreenSaver not parsing")
	}

	if parseCommandLineArgs([]string{"/c"}).ctype != ConfigureScreenSaver {
		t.Error("ConfigureScreenSaver not parsing")
	}

	var expectedPreview = Command{ctype: PreviewScreenSaver, hwnd: 0x200}
	if parseCommandLineArgs([]string{"/p", "0x200"}) != expectedPreview {
		t.Error("PreviewScreenSaver not parsing")
	}
}
