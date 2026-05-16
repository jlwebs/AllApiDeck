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
  [string]$InstalledExeName = "allapideck.exe",
  [string]$UpgradeCode = "39E946D3-FE1D-419F-B018-0C042390D1B9",
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

function Convert-PlainTextToRtf([string]$Value) {
  $normalized = $Value -replace "`r`n", "`n" -replace "`r", "`n"
  $escaped = $normalized.Replace('\', '\\').Replace('{', '\{').Replace('}', '\}')
  $lines = $escaped -split "`n"
  $body = ($lines | ForEach-Object { "$_\par" }) -join "`r`n"
  return "{\rtf1\ansi\deff0{\fonttbl{\f0 Segoe UI;}}`r`n\fs18 $body`r`n}"
}

$inputExePath = Resolve-AbsolutePath $InputExe
$iconFullPath = Resolve-AbsolutePath $IconPath
$outputFullPath = [System.IO.Path]::GetFullPath($OutputFile)
$outputDir = Split-Path -Parent $outputFullPath
$wixWorkDir = Join-Path $outputDir "wix"
$wxsPath = Join-Path $wixWorkDir "product.wxs"
$licenseSourcePath = Resolve-AbsolutePath (Join-Path $PSScriptRoot "..\..\LICENSE")
$licenseRtfPath = Join-Path $wixWorkDir "license.rtf"
$msiVersion = Normalize-MsiVersion $Version

New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
New-Item -ItemType Directory -Path $wixWorkDir -Force | Out-Null

$productNameEscaped = Escape-Wix $ProductName
$manufacturerEscaped = Escape-Wix $Manufacturer
$installDirNameEscaped = Escape-Wix $InstallDirName
$installedExeNameEscaped = Escape-Wix $InstalledExeName
$inputExePathEscaped = Escape-Wix $inputExePath
$iconFullPathEscaped = Escape-Wix $iconFullPath
$licenseRtfPathEscaped = Escape-Wix $licenseRtfPath

$licenseText = Get-Content -LiteralPath $licenseSourcePath -Raw -Encoding UTF8
$licenseRtfContent = Convert-PlainTextToRtf $licenseText
Set-Content -LiteralPath $licenseRtfPath -Value $licenseRtfContent -Encoding ASCII

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
      InstallScope="perUser"
      InstallPrivileges="limited"
      Platform="x64" />
    <MajorUpgrade
      AllowSameVersionUpgrades="yes"
      DowngradeErrorMessage="A newer version of [ProductName] is already installed." />
    <MediaTemplate EmbedCab="yes" />
    <Icon Id="AppIcon.ico" SourceFile="$iconFullPathEscaped" />
    <Property Id="ARPPRODUCTICON" Value="AppIcon.ico" />
    <Property Id="WIXUI_INSTALLDIR" Value="INSTALLFOLDER" />
    <Property Id="WIXUI_EXITDIALOGOPTIONALCHECKBOXTEXT" Value="Launch $productNameEscaped" />
    <Property Id="WIXUI_EXITDIALOGOPTIONALCHECKBOX" Value="1" />
    <Property Id="WixShellExecTarget" Value="[#MainExecutableFile]" />
    <WixVariable Id="WixUILicenseRtf" Value="$licenseRtfPathEscaped" />
    <CustomAction Id="LaunchApplication" BinaryKey="WixCA" DllEntry="WixShellExec" Impersonate="yes" />
    <UIRef Id="WixUI_InstallDir" />
    <UIRef Id="WixUI_ErrorProgressText" />
    <UI>
      <Publish Dialog="ExitDialog" Control="Finish" Event="DoAction" Value="LaunchApplication">
        WIXUI_EXITDIALOGOPTIONALCHECKBOX = 1 AND NOT REMOVE="ALL"
      </Publish>
    </UI>

    <Directory Id="TARGETDIR" Name="SourceDir">
      <Directory Id="LocalAppDataFolder">
        <Directory Id="INSTALLFOLDER" Name="$installDirNameEscaped">
          <Component Id="MainExecutableComponent" Guid="C57A96F5-E97A-4403-B8F6-7C53E4EB78A2">
            <File Id="MainExecutableFile" Source="$inputExePathEscaped" Name="$installedExeNameEscaped" Checksum="yes" />
            <RegistryValue
              Root="HKCU"
              Key="Software\$manufacturerEscaped\$productNameEscaped"
              Name="InstallDir"
              Type="string"
              Value="[INSTALLFOLDER]"
              KeyPath="yes" />
            <RemoveFile Id="RemoveLegacyExeA" Name="allapideck-windows-amd64.exe" On="both" />
            <RemoveFile Id="RemoveLegacyExeB" Name="all-api-deck.exe" On="both" />
            <RemoveFolder Id="RemoveInstallFolder" On="uninstall" />
          </Component>
        </Directory>
      </Directory>
      <Directory Id="ProgramMenuFolder">
        <Directory Id="ApplicationProgramsFolder" Name="$installDirNameEscaped">
          <Component Id="StartMenuShortcutComponent" Guid="C8E8D213-6A63-4AF4-A03D-F226E289E2DB">
            <Shortcut
              Id="ApplicationStartMenuShortcut"
              Name="$productNameEscaped"
              Description="$productNameEscaped"
              Target="[INSTALLFOLDER]$installedExeNameEscaped"
              WorkingDirectory="INSTALLFOLDER"
              Icon="AppIcon.ico" />
            <RemoveFolder Id="RemoveApplicationProgramsFolder" On="uninstall" />
            <RegistryValue
              Root="HKCU"
              Key="Software\$manufacturerEscaped\$productNameEscaped"
              Name="StartMenuShortcut"
              Type="integer"
              Value="1"
              KeyPath="yes" />
          </Component>
        </Directory>
      </Directory>
    </Directory>

    <Feature Id="MainFeature" Title="$productNameEscaped" Level="1">
      <ComponentRef Id="MainExecutableComponent" />
      <ComponentRef Id="StartMenuShortcutComponent" />
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
  & $candle.Source -nologo "-arch" "x64" "-ext" "WixUIExtension" "-ext" "WixUtilExtension" "-out" "product.wixobj" $wxsPath
  if ($LASTEXITCODE -ne 0) {
    throw "WiX candle compilation failed."
  }

  & $light.Source -nologo "-ext" "WixUIExtension" "-ext" "WixUtilExtension" "-sice:ICE61" "-sice:ICE91" "-out" $outputFullPath "product.wixobj"
  if ($LASTEXITCODE -ne 0) {
    throw "WiX light link failed."
  }
} finally {
  Pop-Location
}

Write-Host "MSI created: $outputFullPath"
