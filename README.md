# golang-video-screensaver

A screensaver that plays a random video from a configured path on each screen. When each video finishes, it selects another at random to play.

Implemented in Golang.

# Building

```
powershell build.ps1
```

# Installing

A basic installer is built that will install the dependencies into a folder and write a couple of registry entries.

```
out/VideoGalleryInstaller.exe
```

# Running

To test it:

```
out/VideoScreensaver.scr /S
```

To configure it:

```
out/VideoScreensaver.scr /C
```

Screensavers are regular executables that need to handle a small number of command-line arguments and have the correct semantics to exit on interaction. We do not use [the functions in scrnsave.lib](https://docs.microsoft.com/en-us/windows/win32/api/scrnsave/nf-scrnsave-screensaverconfiguredialog), but handle the [command-line arguments](https://docs.microsoft.com/en-us/troubleshoot/windows/win32/screen-saver-command-line) instead.

The only awkward case to handle is being given an `HWND` to use as a parent window for previewing the screensaver. This works but makes the code somewhat uglier in the current implementation.