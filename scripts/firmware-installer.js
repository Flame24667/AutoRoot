#!/usr/bin/env node
/**
 * Firmware Downloader - Runs during AutoRoot installation
 * Downloads firmware files based on user selection
 */

const fs = require('fs');
const path = require('path');
const https = require('https');
const http = require('http');
const { execSync } = require('child_process');

// Firmware database (simplified for installer)
const FIRMWARE_DB = {
  devices: [
    {
      brand: "ASUS",
      model: "ASUS_AI2401",
      displayName: "ROG Phone 8",
      firmware: [{
        region: "WW",
        version: "32.0810.0810.80",
        url: "https://dl.asus.com/rogphone/ASUS_AI2401_WW_user.zip",
        filename: "ASUS_AI2401_WW_32.0810.0810.80.zip",
        size: "4.2 GB"
      }]
    },
    {
      brand: "Samsung", 
      model: "SM-S921B",
      displayName: "Galaxy S24",
      firmware: [{
        region: "XSG",
        version: "S921BXXU1AWL1",
        url: "https://example.com/SM-S921B_XSG.tar.md5",
        filename: "SM-S921B_XSG_S921BXXU1AWL1.tar.md5",
        size: "6.8 GB"
      }]
    }
    // Add more devices as needed
  ]
};

// Determine where to save firmware (AppData for per-user install)
function getFirmwareDir() {
  const appData = process.env.APPDATA || path.join(process.env.HOME, 'AppData', 'Roaming');
  const dir = path.join(appData, 'AutoRoot', 'firmware');
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }
  return dir;
}

async function downloadFile(url, dest) {
  return new Promise((resolve, reject) => {
    const protocol = url.startsWith('https') ? https : http;
    
    protocol.get(url, (res) => {
      if (res.statusCode === 302 || res.statusCode === 301) {
        // Follow redirect
        downloadFile(res.headers.location, dest).then(resolve).catch(reject);
        return;
      }
      
      const file = fs.createWriteStream(dest);
      res.pipe(file);
      
      file.on('finish', () => {
        file.close();
        resolve(dest);
      });
    }).on('error', (err) => {
      fs.unlink(dest, () => {});
      reject(err);
    });
  });
}

async function main() {
  const args = process.argv.slice(2);
  const installDir = args[0] || process.cwd();
  const firmwareDir = getFirmwareDir();
  
  console.log(`🔓 AutoRoot Firmware Installer`);
  console.log(`📂 Saving to: ${firmwareDir}\n`);
  
  // For now, download ALL firmware (in production, add CLI args for selection)
  for (const device of FIRMWARE_DB.devices) {
    for (const fw of device.firmware) {
      const dest = path.join(firmwareDir, fw.filename);
      
      if (fs.existsSync(dest)) {
        console.log(`⏭️  ${fw.filename} already exists`);
        continue;
      }
      
      console.log(`📥 Downloading ${fw.filename} (${fw.size})...`);
      
      try {
        await downloadFile(fw.url, dest);
        console.log(`✅ ${fw.filename} downloaded successfully`);
      } catch (err) {
        console.error(`❌ Failed to download ${fw.filename}: ${err.message}`);
      }
    }
  }
  
  console.log('\n✨ Firmware installation complete!');
  console.log(`📁 Files saved to: ${firmwareDir}`);
}

main().catch(console.error);