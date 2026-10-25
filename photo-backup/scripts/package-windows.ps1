param(
  [string]$ModelPath = "yolo11n.pt"
)

$ErrorActionPreference = "Stop"
$project = Split-Path -Parent $PSScriptRoot
Set-Location $project

if (-not (Test-Path ".venv-yolo\Scripts\pyinstaller.exe")) {
  .\.venv-yolo\Scripts\python.exe -m pip install pyinstaller
  if ($LASTEXITCODE -ne 0) {
    throw "Could not install PyInstaller. Check network access and run again."
  }
}

if (-not (Test-Path $ModelPath)) {
  throw "YOLO model not found: $ModelPath"
}

& .\.venv-yolo\Scripts\pyinstaller.exe --noconfirm --clean --onefile `
  --name yolo_worker `
  --collect-binaries torchvision `
  --collect-data torchvision `
  --hidden-import torchvision._C_stable `
  --hidden-import torchvision._meta_registrations `
  --add-data "$ModelPath;." `
  tools\yolo_worker.py
if ($LASTEXITCODE -ne 0) {
  throw "PyInstaller failed"
}

if (-not (Test-Path "build\bin\photo-backup.exe")) {
  throw "Run wails build successfully first"
}

New-Item -ItemType Directory -Force "tools\bin\windows" | Out-Null
New-Item -ItemType Directory -Force "models" | Out-Null
Copy-Item -Force "dist\yolo_worker.exe" "tools\bin\windows\yolo_worker.exe"
Copy-Item -Force $ModelPath "models\yolo11n.pt"
Write-Host "Portable YOLO assets are in tools\bin\windows and models"

$blipModelPath = "models\blip-image-captioning-base"
if (Test-Path $blipModelPath) {
  if (-not (Test-Path ".venv-blip\Scripts\pyinstaller.exe")) {
    throw "BLIP model exists but .venv-blip with PyInstaller was not found"
  }
  & .\.venv-blip\Scripts\pyinstaller.exe --noconfirm --clean --onefile `
    --name caption_worker `
    --collect-all transformers `
    --collect-all torch `
    --collect-all PIL `
    --hidden-import torchvision `
    tools\caption_worker.py
  if ($LASTEXITCODE -ne 0) { throw "BLIP PyInstaller failed" }
  Copy-Item -Force "dist\caption_worker.exe" "tools\bin\windows\caption_worker.exe"
  Write-Host "Portable BLIP assets are in tools\bin\windows and models\blip-image-captioning-base"
}
