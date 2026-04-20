!include "LogicLib.nsh"

!macro customInstall
  ; Ask user to download firmware
  MessageBox MB_YESNO|MB_ICONQUESTION "Download firmware for offline use?$\n$\nRequires internet connection." IDNO SkipFirmware
  
  DetailPrint "Starting firmware download..."
  
  ; Run the downloader script
  ExecWait 'node "$INSTDIR\resources\firmware-installer.js"' $0
  
  ; Check exit code (0 = success)
  ${If} $0 != 0
    MessageBox MB_OK|MB_ICONEXCLAMATION "Firmware download failed.$\nCheck your connection or try again later in the app."
  ${Else}
    MessageBox MB_OK|MB_ICONINFORMATION "Firmware downloaded successfully!$\nAutoRoot is ready."
  ${EndIf}
  
  SkipFirmware:
!macroend