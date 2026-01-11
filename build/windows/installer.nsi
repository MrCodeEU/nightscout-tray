!include "MUI2.nsh"
!include "FileFunc.nsh"

; Basic Info
Name "Nightscout Tray"
OutFile "NightscoutTray-Setup.exe"
InstallDir "$LOCALAPPDATA\NightscoutTray"
InstallDirRegKey HKCU "Software\NightscoutTray" "InstallDir"
RequestExecutionLevel user

; Version Info
VIProductVersion "1.0.0.0"
VIAddVersionKey "ProductName" "Nightscout Tray"
VIAddVersionKey "CompanyName" "mrcode"
VIAddVersionKey "LegalCopyright" "Copyright (c) 2026 mrcode"
VIAddVersionKey "FileDescription" "Nightscout glucose monitoring tray application"
VIAddVersionKey "FileVersion" "1.0.0"
VIAddVersionKey "ProductVersion" "1.0.0"

; MUI Settings
!define MUI_ABORTWARNING
!define MUI_ICON "${NSISDIR}\Contrib\Graphics\Icons\modern-install.ico"
!define MUI_UNICON "${NSISDIR}\Contrib\Graphics\Icons\modern-uninstall.ico"

; Pages
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES
!define MUI_FINISHPAGE_RUN "$INSTDIR\nightscout-tray.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Launch Nightscout Tray"
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; Languages
!insertmacro MUI_LANGUAGE "English"

; Installer Sections
Section "Nightscout Tray (required)" SecMain
    SectionIn RO
    
    SetOutPath "$INSTDIR"
    File "nightscout-tray.exe"
    
    ; Store installation folder
    WriteRegStr HKCU "Software\NightscoutTray" "InstallDir" $INSTDIR
    
    ; Create uninstaller
    WriteUninstaller "$INSTDIR\Uninstall.exe"
    
    ; Add to Programs and Features
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\NightscoutTray" \
                     "DisplayName" "Nightscout Tray"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\NightscoutTray" \
                     "UninstallString" "$\"$INSTDIR\Uninstall.exe$\""
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\NightscoutTray" \
                     "InstallLocation" "$INSTDIR"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\NightscoutTray" \
                     "Publisher" "mrcode"
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\NightscoutTray" \
                     "DisplayVersion" "1.0.0"
    WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\NightscoutTray" \
                      "NoModify" 1
    WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\NightscoutTray" \
                      "NoRepair" 1
    
    ; Calculate size
    ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
    IntFmt $0 "0x%08X" $0
    WriteRegDWORD HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\NightscoutTray" \
                      "EstimatedSize" "$0"
SectionEnd

Section "Start Menu Shortcut" SecStartMenu
    CreateDirectory "$SMPROGRAMS\Nightscout Tray"
    CreateShortcut "$SMPROGRAMS\Nightscout Tray\Nightscout Tray.lnk" "$INSTDIR\nightscout-tray.exe"
    CreateShortcut "$SMPROGRAMS\Nightscout Tray\Uninstall.lnk" "$INSTDIR\Uninstall.exe"
SectionEnd

Section "Desktop Shortcut" SecDesktop
    CreateShortcut "$DESKTOP\Nightscout Tray.lnk" "$INSTDIR\nightscout-tray.exe"
SectionEnd

Section "Start with Windows" SecAutostart
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" \
                     "NightscoutTray" "$INSTDIR\nightscout-tray.exe"
SectionEnd

; Section Descriptions
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
    !insertmacro MUI_DESCRIPTION_TEXT ${SecMain} "The main application files (required)."
    !insertmacro MUI_DESCRIPTION_TEXT ${SecStartMenu} "Create Start Menu shortcuts."
    !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} "Create a Desktop shortcut."
    !insertmacro MUI_DESCRIPTION_TEXT ${SecAutostart} "Start Nightscout Tray automatically when Windows starts."
!insertmacro MUI_FUNCTION_DESCRIPTION_END

; Uninstaller Section
Section "Uninstall"
    ; Kill running process
    nsExec::ExecToLog 'taskkill /F /IM nightscout-tray.exe'
    
    ; Remove files
    Delete "$INSTDIR\nightscout-tray.exe"
    Delete "$INSTDIR\Uninstall.exe"
    RMDir "$INSTDIR"
    
    ; Remove shortcuts
    Delete "$SMPROGRAMS\Nightscout Tray\Nightscout Tray.lnk"
    Delete "$SMPROGRAMS\Nightscout Tray\Uninstall.lnk"
    RMDir "$SMPROGRAMS\Nightscout Tray"
    Delete "$DESKTOP\Nightscout Tray.lnk"
    
    ; Remove registry keys
    DeleteRegKey HKCU "Software\NightscoutTray"
    DeleteRegKey HKCU "Software\Microsoft\Windows\CurrentVersion\Uninstall\NightscoutTray"
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "NightscoutTray"
    
    ; Remove config directory (optional - ask user?)
    RMDir /r "$APPDATA\nightscout-tray"
SectionEnd
