param(
    [switch]$DownloadOnly,
    [switch]$DontCompress
)

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
function Get-And-Extract-Zip {
    param(
        [String]$Destination="",
        [String]$Uri
    )

    Write-Output "Downloading $Uri"

    $tmp = New-TemporaryFile | Rename-Item -NewName { $_ -replace 'tmp$', 'zip' } -PassThru
    Invoke-WebRequest -Uri $Uri -OutFile $tmp
    Expand-Archive -Path $tmp -DestinationPath "$PSScriptRoot\out\$Destination"
    Remove-Item -Path $tmp
}

$ignored=New-Item -Path .\out -ItemType Directory -Force

if (-not(Test-Path -Path "out\upx-3.96-win64\upx.exe" -PathType Leaf)) {
    Get-And-Extract-Zip -Uri "https://github.com/upx/upx/releases/download/v3.96/upx-3.96-win64.zip"
}

if (-not(Test-Path -Path "out\libvlc-3.0.16\build\x64\libvlc.dll" -PathType Leaf)) {
    Get-And-Extract-Zip -Uri "https://www.nuget.org/api/v2/package/VideoLAN.LibVLC.Windows/3.0.16" -Destination "libvlc-3.0.16"
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

if (-not($DontCompress)) {
    Write-Output "Compressing installer"
    Invoke-NativeCommand "out\upx-3.96-win64\upx.exe" -qq "out\VideoGalleryInstaller.exe"
}