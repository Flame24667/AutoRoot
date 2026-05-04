!include "LogicLib.nsh"

!macro customInstall
  MessageBox MB_YESNO|MB_ICONQUESTION "Download device firmware now?$\n$\nRequires internet. Files saved for offline use." IDNO SkipFirmware

  DetailPrint "📥 Starting firmware download..."

  ; Run PowerShell downloader (native to Windows, no Node required)
  ExecWait 'powershell -ExecutionPolicy Bypass -NoProfile -File "$INSTDIR\resources\firmware-installer.ps1" "$INSTDIR"' $0

  ${If} $0 == 0
    MessageBox MB_OK|MB_ICONINFORMATION "✅ Firmware downloaded successfully!$\n$\nAutoRoot is ready for offline use."
  ${Else}
    MessageBox MB_OK|MB_ICONEXCLAMATION "⚠️ Download failed or skipped.$\n$\nYou can retry later or check your internet connection."
  ${EndIf}

  SkipFirmware:
!macroend