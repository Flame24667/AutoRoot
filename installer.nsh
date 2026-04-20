!macro customInstall
  ; This macro runs automatically at the end of the installation
  
  ; Ask the user if they want to download firmware
  MessageBox MB_YESNO|MB_ICONQUESTION "Download Firmware?" \
    "AutoRoot requires device-specific firmware files to work offline.$\n$\nWould you like to download them now?$\n$\n(Requires Internet connection)" \
    IDNO SkipFirmware

  ; If user clicked Yes
  DetailPrint "🔓 Starting firmware download..."
  
  ; Run the firmware installer script
  ; Note: This assumes 'node' is in the system PATH.
  ; If the user doesn't have Node.js installed, this will fail silently or show an error.
  ExecWait 'node "$INSTDIR\resources\firmware-installer.js"' $0
  
  ; Check if the script succeeded (Exit code 0 is success)
  ${If} $0 != 0
    MessageBox MB_OK|MB_ICONEXCLAMATION "Firmware download failed (Error Code: $0).$\n$\nPlease check your internet connection or download firmware later from within the app."
  ${Else}
    MessageBox MB_OK|MB_ICONINFORMATION "Firmware download complete!$\n$\nYou can now use AutoRoot offline."
  ${EndIf}

  SkipFirmware:
!macroend