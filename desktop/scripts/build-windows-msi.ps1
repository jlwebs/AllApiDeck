param(
  [Parameter(Mandatory = $true)]
  [string]$InputExe,

  [Parameter(Mandatory = $true)]
  [string]$Version,

  [Parameter(Mandatory = $true)]
  [string]$OutputFile,

  [string]$ProductName = "AllApiDeck",
  [string]$Manufacturer = "ding",
  [string]$InstallDirName = "AllApiDeck",
  [string]$UpgradeCode = "3F4F7E64-88E8-4A41-A0F9-7AB6C676AB3B",
  [string]$IconPath = "build/windows/icon.ico"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Resolve-AbsolutePath([string]$PathValue) {
  $resolved = Resolve-Path -LiteralPath $PathValue -ErrorAction Stop
  return $resolved.Path
}

function Normalize-MsiVersion([string]$RawVersion) {
  $matches = [regex]::Matches($RawVersion, '\d+')
  if ($matches.Count -eq 0) {
    return "0.0.0"
  }

  $parts = @()
  for ($index = 0; $index -lt 3; $index += 1) {
    if ($index -lt $matches.Count) {
      $parts += [int]$matches[$index].Value
    } else {
      $parts += 0
    }
  }

  return ($parts -join '.')
}

function Escape-Wix([string]$Value) {
  return $Value.Replace('&', '&amp;').Replace('<', '&lt;').Replace('>', '&gt;').Replace('"', '&quot;')
}

$inputExePath = Resolve-AbsolutePath $InputExe
$iconFullPath = Resolve-AbsolutePath $IconPath
$outputFullPath = [System.IO.Path]::GetFullPath($OutputFile)
$outputDir = Split-Path -Parent $outputFullPath
$wixWorkDir = Join-Path $outputDir "wix"
$wxsPath = Join-Path $wixWorkDir "product.wxs"
$msiVersion = Normalize-MsiVersion $Version

New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
New-Item -ItemType Directory -Path $wixWorkDir -Force | Out-Null

$productNameEscaped = Escape-Wix $ProductName
$manufacturerEscaped = Escape-Wix $Manufacturer
$installDirNameEscaped = Escape-Wix $InstallDirName
$inputExePathEscaped = Escape-Wix $inputExePath
$iconFullPathEscaped = Escape-Wix $iconFullPath

$wxsContent = @"
<?xml version="1.0" encoding="UTF-8"?>
<Wix xmlns="http://schemas.microsoft.com/wix/2006/wi">
  <Product
    Id="*"
    Name="$productNameEscaped"
    Language="1033"
    Version="$msiVersion"
    Manufacturer="$manufacturerEscaped"
    UpgradeCode="$UpgradeCode">
    <Package
      InstallerVersion="500"
      Compressed="yes"
      InstallScope="perMachine"
      Platform="x64" />
    <MajorUpgrade DowngradeErrorMessage="A newer version of [ProductName] is already installed." />
    <MediaTemplate EmbedCab="yes" />
    <Icon Id="AppIcon.ico" SourceFile="$iconFullPathEscaped" />
    <Property Id="ARPPRODUCTICON" Value="AppIcon.ico" />

    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="ProgramFiles64Folder">
        <Directory Id="INSTALLFOLDER" Name="$installDirNameEscaped">
          <Component Id="MainExecutableComponent" Guid="*">
            <File Id="MainExecutableFile" Source="$inputExePathEscaped" KeyPath="yes" Checksum="yes" />
            <Shortcut
              Id="StartMenuShortcut"
              Directory="ProgramMenuDir"
              Name="$productNameEscaped"
              WorkingDirectory="INSTALLFOLDER"
              Advertise="no"
              Icon="AppIcon.ico"
              IconIndex="0" />
            <RemoveFolder Id="RemoveInstallFolder" On="uninstall" />
            <RemoveFolder Id="RemoveProgramMenuDir" Directory="ProgramMenuDir" On="uninstall" />
            <RegistryValue Root="HKLM" Key="Software\$manufacturerEscaped\$productNameEscaped" Name="Installed" Type="integer" Value="1" KeyPath="no" />
          </Component>
        </Directory>
      </Directory>
      <Directory Id="ProgramMenuFolder">
        <Directory Id="ProgramMenuDir" Name="$productNameEscaped" />
      </Directory>
    </Directory>

    <Feature Id="MainFeature" Title="$productNameEscaped" Level="1">
      <ComponentRef Id="MainExecutableComponent" />
    </Feature>
  </Product>
</Wix>
"@

Set-Content -LiteralPath $wxsPath -Value $wxsContent -Encoding UTF8

$candle = Get-Command candle.exe -ErrorAction SilentlyContinue
$light = Get-Command light.exe -ErrorAction SilentlyContinue
if (-not $candle -or -not $light) {
  throw "WiX Toolset (candle.exe/light.exe) is not available on PATH."
}

Push-Location $wixWorkDir
try {
  & $candle.Source -nologo "-arch" "x64" "-out" "product.wixobj" $wxsPath
  if ($LASTEXITCODE -ne 0) {
    throw "WiX candle compilation failed."
  }

  & $light.Source -nologo "-sice:ICE61" "-out" $outputFullPath "product.wixobj"
  if ($LASTEXITCODE -ne 0) {
    throw "WiX light link failed."
  }
} finally {
  Pop-Location
}

Write-Host "MSI created: $outputFullPath"
