; Ferry NSIS Installer Script (LZMA Solid)
; Build with: makensis /DVERSION=0.1.0 /DOUTFILE=..\dist\Ferry-0.1.0-windows-amd64-setup.exe installer.nsi

!include "MUI2.nsh"
!include "x64.nsh"

!ifndef VERSION
  !define VERSION "0.1.0-dev"
!endif
!ifndef OUTFILE
  !define OUTFILE "..\dist\Ferry-${VERSION}-windows-amd64-setup.exe"
!endif

Name "Ferry ${VERSION}"
OutFile "${OUTFILE}"
Unicode True
SetCompressor /SOLID lzma
SetCompressorDictSize 32
InstallDir "$PROGRAMFILES64\Ferry"
InstallDirRegKey HKLM "Software\Ferry" "InstallDir"
RequestExecutionLevel admin

!define MUI_ABORTWARNING
!define MUI_ICON "ferry.ico"
!define MUI_UNICON "ferry.ico"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "..\LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "English"

Section "Ferry Core & GUI" SecMain
  SectionIn RO
  SetOutPath "$INSTDIR"

  File "..\bin\Ferry.exe"
  File "..\bin\ferry-core.exe"
  File "..\LICENSE"
  File "..\NOTICE"
  File "..\README.md"

  WriteRegStr HKLM "Software\Ferry" "InstallDir" "$INSTDIR"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Ferry" "DisplayName" "Ferry"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Ferry" "DisplayVersion" "${VERSION}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Ferry" "Publisher" "Ferry Project"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Ferry" "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Ferry" "DisplayIcon" '"$INSTDIR\Ferry.exe"'
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Ferry" "NoModify" 1
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Ferry" "NoRepair" 1

  ; Register ferry:// deep-link protocol handler
  WriteRegStr HKCR "ferry" "" "URL:Ferry Protocol"
  WriteRegStr HKCR "ferry" "URL Protocol" ""
  WriteRegStr HKCR "ferry\DefaultIcon" "" '"$INSTDIR\Ferry.exe",0'
  WriteRegStr HKCR "ferry\shell\open\command" "" '"$INSTDIR\Ferry.exe" "%1"'

  CreateDirectory "$SMPROGRAMS\Ferry"
  CreateShortcut "$SMPROGRAMS\Ferry\Ferry.lnk" "$INSTDIR\Ferry.exe"
  CreateShortcut "$SMPROGRAMS\Ferry\Uninstall Ferry.lnk" "$INSTDIR\Uninstall.exe"

  WriteUninstaller "$INSTDIR\Uninstall.exe"
SectionEnd

Section "Uninstall"
  ; Stop and remove service + ProgramData auth artifacts via ferry-core
  IfFileExists "$INSTDIR\ferry-core.exe" 0 +2
    nsExec::ExecToLog '"$INSTDIR\ferry-core.exe" uninstall-service'
  nsExec::ExecToLog 'sc.exe stop FerryCore'
  nsExec::ExecToLog 'sc.exe delete FerryCore'
  SetShellVarContext all
  RMDir /r "$APPDATA\Ferry\service"
  RMDir /r "$APPDATA\Ferry\service-client"

  Delete "$SMPROGRAMS\Ferry\Ferry.lnk"
  Delete "$SMPROGRAMS\Ferry\Uninstall Ferry.lnk"
  RMDir "$SMPROGRAMS\Ferry"

  Delete "$INSTDIR\Ferry.exe"
  Delete "$INSTDIR\ferry-core.exe"
  Delete "$INSTDIR\LICENSE"
  Delete "$INSTDIR\NOTICE"
  Delete "$INSTDIR\README.md"
  Delete "$INSTDIR\Uninstall.exe"
  RMDir "$INSTDIR"

  DeleteRegKey HKCR "ferry"
  DeleteRegKey HKLM "Software\Ferry"
  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\Ferry"
SectionEnd
