param([switch]$DownloadOnly)

$ErrorActionPreference = "Stop"

# Credit goes to https://stackoverflow.com/a/11450852/1187411
function Invoke-NativeCommand {
    $command = $args[0]
    $arguments = $args[1..($args.Length)]
    & $command @arguments
    if ($LastExitCode -ne 0) {
        Write-Error "Exit code $LastExitCode while running $command $arguments"
    }
}

# This function name doesn't quite follow the "<ApprovedVerb>-<Prefix><SingularNoun>"
# naming convention... Oh well!
function Download-And-Extract-Zip {
    param([String]$Destination="") 

    $url = $args[0]

    Write-Output "Downloading $url"

    $tmp = New-TemporaryFile | Rename-Item -NewName { $_ -replace 'tmp$', 'zip' } -PassThru
    Invoke-WebRequest $url -OutFile $tmp
    Expand-Archive -Path $tmp -DestinationPath "$PSScriptRoot\out\$Destination"
    Remove-Item -Path $tmp
}

# TODO:
#  - actually write the installer using go:ember

$ignored=New-Item -Path .\out -ItemType Directory -Force

if (-not(Test-Path -Path "out\upx-3.96-win64\upx.exe" -PathType Leaf)) {
    Download-And-Extract-Zip "https://github.com/upx/upx/releases/download/v3.96/upx-3.96-win64.zip"
}

if (-not(Test-Path -Path "out\libvlc-3.0.16\build\x64\libvlc.dll" -PathType Leaf)) {
    Download-And-Extract-Zip "https://www.nuget.org/api/v2/package/VideoLAN.LibVLC.Windows/3.0.16" -Destination "libvlc-3.0.16"
}

if ($DownloadOnly.IsPresent) {
    exit 0
}

# "-H=windowsgui" ensures we don't have a console window popup
Write-Output "Building screensaver"
Invoke-NativeCommand go build -v -ldflags -H=windowsgui -o "out/VideoGallery.scr" github.com/sammydre/golang-video-screensaver/cmd/screensaver

# We'll probably want -ldflags -H=windowsgui in the fullness of time
Write-Output "Building installer"
Invoke-NativeCommand go build -v -o "out/VideoGalleryInstaller.exe" github.com/sammydre/golang-video-screensaver/cmd/installer

Write-Output "Compressing installer"
Invoke-NativeCommand "out\upx-3.96-win64\upx.exe" "out\VideoGalleryInstaller.exe"