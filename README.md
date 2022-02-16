# golang-video-screensaver

A work in progress Microsoft Windows video screensaver implemented in Go.

Eventually, it should be possible to install as a screensaver. It does not quite implement the right behaviour for that yet.

For now, when run, it will play a random video from the given directory fullscreen on each connected monitor. When each video finishes, it selects another at random to play.

# Building

```
go build github.com/sammydre/golang-video-screensaver/cmd
```

# Running

```
cmd.exe --video-path "C:\Users\bob\Videos\Screensaver\Screensaver Videos"
```