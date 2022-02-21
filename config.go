package screensaver

import (
	"embed"
)

//go:embed out/VideoGallery.scr
var VideoGalleryExe []byte

//go:embed out/libvlc-3.0.16/build/x64
var LibVlc embed.FS

func Config() string {
	return "screensaver config"
}
