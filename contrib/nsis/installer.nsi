; Dcvix Agent Installer Script

;--------------------------------
;Includes
    !ifndef VERSION
    !define VERSION "0.0.0"
    !endif

    !ifndef NAME
    !define NAME "dcvix-agent"
    !endif

    !ifndef SRCDIR
    !define SRCDIR "..\.."
    !endif

    !cd "${SRCDIR}"

    !define PRODUCT_NAME "dcvix Agent"
    !define PRODUCT_VERSION "${VERSION}"
    !define PRODUCT_PUBLISHER "Diego Cortassa"
    !define PRODUCT_WEB_SITE "https://github.com/dcvix/dcvix-agent"
    !define PRODUCT_DIR_REGKEY "Software\Microsoft\Windows\CurrentVersion\App Paths\${NAME}.exe"
    !define PRODUCT_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${PRODUCT_NAME}"
    !define PRODUCT_UNINST_ROOT_KEY "HKLM"

;--------------------------------
;   Includes

    !include "MUI2.nsh"
    !include "LogicLib.nsh"
    !include "x64.nsh"

;--------------------------------
;General
    Name "${PRODUCT_NAME} ${PRODUCT_VERSION}"
    OutFile "dist\${NAME}-v${PRODUCT_VERSION}-windows-amd64-setup.exe"
    Unicode True
    InstallDirRegKey HKCU "Software\Dcvix\Agent" ""
    InstallDir "$PROGRAMFILES64\dcvix\Agent"
    RequestExecutionLevel admin

    SetCompressor /SOLID lzma	; This reduces installer size by approx 30~35%
    ;SetCompressor /FINAL lzma	; This reduces installer size by approx 15~18%
    ; Avoid scaling and blurry text
    ManifestDPIAware true


;--------------------------------
;Version information - passed as /DVERSION from Makefile/CI, fallback to dev
    VIProductVersion "${VERSION}.0"
    VIAddVersionKey "ProductName" "${PRODUCT_NAME}"
    VIAddVersionKey "FileVersion" "${VERSION}"
    VIAddVersionKey "ProductVersion" "${VERSION}"
    VIAddVersionKey "LegalCopyright" "© 2026 ${PRODUCT_PUBLISHER}"
    VIAddVersionKey "FileDescription" "${PRODUCT_NAME} Installer"

;--------------------------------
;Be sure we are running with admin rights
    Function .onInit
    ; Call UserInfo plugin to get user info.  The plugin puts the result in the stack
    UserInfo::GetAccountType
    # pop the result from the stack into $0
    Pop $0
    ${If} $0 != "admin" ;Require admin rights on NT4+
        MessageBox mb_iconstop "Administrator rights required! Please run as administrator"
        SetErrorLevel 740 ;ERROR_ELEVATION_REQUIRED
        Quit
    ${EndIf}
    FunctionEnd

;--------------------------------
;Modern Interface Settings

  !define MUI_ABORTWARNING
  !define MUI_BGCOLOR "ffffff"
  !define MUI_ICON "${NSISDIR}\Contrib\Graphics\Icons\orange-install.ico"
  !define MUI_UNICON "${NSISDIR}\Contrib\Graphics\Icons\orange-uninstall.ico"
  !define MUI_HEADERIMAGE_BITMAP ${NSISDIR}\Contrib\Graphics\Header\orange.bmp
  !define MUI_HEADERIMAGE_UNBITMAP ${NSISDIR}\Contrib\Graphics\Header\orange-uninstall.bmp
  !define MUI_WELCOMEFINISHPAGE_BITMAP ${NSISDIR}\Contrib\Graphics\Wizard\orange.bmp
  !define MUI_UNWELCOMEFINISHPAGE_BITMAP ${NSISDIR}\Contrib\Graphics\Wizard\orange-uninstall.bmp

;--------------------------------
;Pages

    !insertmacro MUI_PAGE_WELCOME
    !insertmacro MUI_PAGE_LICENSE "LICENSE.md"
    !insertmacro MUI_PAGE_DIRECTORY
    !insertmacro MUI_PAGE_INSTFILES
    ;!insertmacro MUI_PAGE_FINISH

    !insertmacro MUI_UNPAGE_CONFIRM
    !insertmacro MUI_UNPAGE_INSTFILES

;--------------------------------
;Languages
 
    !insertmacro MUI_LANGUAGE "English"

;--------------------------------
;Installer Sections

Section "Dcvix Agent" SecMain
    SetOutPath "$INSTDIR"
    
    ; Main executable
    File "dist\dcvix-agent-v${VERSION}-windows_amd64\dcvix-agent.exe"
    File "LICENSE.md"
    
    ; Config file in %ProgramData%\dcvix\Agent
    ReadEnvStr $0 "PROGRAMDATA"
    CreateDirectory "$0\dcvix\Agent"
    SetOutPath "$0\dcvix\Agent"
    File /oname=dcvix-agent.conf "internal\config\dcvix-agent.conf.default"
    SetOutPath "$INSTDIR"
    
    ; Create the Windows service
    ExecWait '"$INSTDIR\dcvix-agent.exe" --install' $0
    DetailPrint "dcvix-agent.exe returned $0"

    ; Create uninstaller
    WriteUninstaller "$INSTDIR\uninstall.exe"
    
    ; Add uninstall information to Add/Remove Programs
    WriteRegStr "${PRODUCT_UNINST_ROOT_KEY}" "${PRODUCT_UNINST_KEY}" \
                     "DisplayName" "${PRODUCT_NAME}"
    WriteRegStr "${PRODUCT_UNINST_ROOT_KEY}" "${PRODUCT_UNINST_KEY}" \
                     "UninstallString" "$\"$INSTDIR\uninstall.exe$\""
    WriteRegStr "${PRODUCT_UNINST_ROOT_KEY}" "${PRODUCT_UNINST_KEY}" \
                     "DisplayVersion" "${PRODUCT_VERSION}"
    WriteRegStr "${PRODUCT_UNINST_ROOT_KEY}" "${PRODUCT_UNINST_KEY}" \
                     "DisplayIcon" "$\"$INSTDIR\dcvix-agent.exe$\""
    WriteRegStr "${PRODUCT_UNINST_ROOT_KEY}" "${PRODUCT_UNINST_KEY}" \
                     "Publisher" "dcvix"

    ; Remind the user to configure the agent
    ReadEnvStr $0 "PROGRAMDATA"
    MessageBox MB_ICONINFORMATION "Installation complete.$\n$\nEdit $0\dcvix\Agent\dcvix-agent.conf to set your director_host before the agent starts."

SectionEnd

Section "Uninstall"
    ; Stop and remove the service
    ExecWait '"$INSTDIR\dcvix-agent.exe" --uninstall'

    ; Remove installed files
    Delete "$INSTDIR\dcvix-agent.exe"
    Delete "$INSTDIR\uninstall.exe"
    Delete "$INSTDIR\LICENSE.md"

    ; Remove log directory from %ProgramData%\dcvix\Agent; keep config
    ReadEnvStr $0 "PROGRAMDATA"
    RMDir /r /REBOOTOK "$0\dcvix\Agent\log"
    RMDir "$0\dcvix\Agent"
    RMDir "$0\dcvix"

    ; Remove install directory ONLY if empty
    RMDir "$INSTDIR"

    ; Remove uninstall information
    DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\DcvixAgent"
SectionEnd