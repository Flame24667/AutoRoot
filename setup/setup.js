#!/usr/bin/env node

const fs = require('fs');
const path = require('path');
const https = require('https');
const http = require('http');
const { execSync } = require('child_process');

const FIRMWARE_DB = path.join(__dirname, 'firmware-db.json');
const FIRMWARE_DIR = path.join(process.cwd(), 'firmware');

// Colors for console
const colors = {
  reset: '\x1b[0m',
  red: '\x1b[31m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  blue: '\x1b[34m',
  cyan: '\x1b[36m',
};

function log(msg, color = 'reset') {
  console.log(`${colors[color]}${msg}${colors.reset}`);
}

async function downloadFile(url, dest, onProgress) {
  return new Promise((resolve, reject) => {
    const protocol = url.startsWith('https') ? https : http;
    
    protocol.get(url, (res) => {
      const totalSize = parseInt(res.headers['content-length'], 10);
      let downloaded = 0;
      
      const file = fs.createWriteStream(dest);
      
      res.on('data', (chunk) => {
        downloaded += chunk.length;
        if (onProgress) {
          const percent = ((downloaded / totalSize) * 100).toFixed(1);
          const mb = (downloaded / 1024 / 1024).toFixed(1);
          const totalMB = (totalSize / 1024 / 1024).toFixed(1);
          onProgress(percent, mb, totalMB);
        }
      });
      
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
  log('\n🔓 AutoRoot Setup Wizard', 'cyan');
  log('══════════════════════════════════\n', 'cyan');
  
  // Ensure firmware directory exists
  if (!fs.existsSync(FIRMWARE_DIR)) {
    fs.mkdirSync(FIRMWARE_DIR, { recursive: true });
    log(`📁 Created firmware directory: ${FIRMWARE_DIR}`, 'green');
  }
  
  // Load firmware database
  let db;
  try {
    db = JSON.parse(fs.readFileSync(FIRMWARE_DB, 'utf8'));
  } catch (err) {
    log('❌ Error: firmware-db.json not found or invalid', 'red');
    process.exit(1);
  }
  
  log(`📱 Found ${db.devices.length} devices in database\n`, 'yellow');
  
  // Display device list
  db.devices.forEach((device, index) => {
    log(`${index + 1}. ${device.displayName} (${device.brand} ${device.model})`, 'blue');
  });
  
  // Get user selection
  const readline = require('readline').createInterface({
    input: process.stdin,
    output: process.stdout,
  });
  
  const question = (query) => new Promise((resolve) => readline.question(query, resolve));
  
  const selection = await question('\n🔢 Select device number (or "all"): ');
  readline.close();
  
  let selectedDevices = [];
  if (selection.toLowerCase() === 'all') {
    selectedDevices = db.devices;
  } else {
    const indices = selection.split(',').map(s => parseInt(s.trim()) - 1);
    selectedDevices = indices.map(i => db.devices[i]).filter(Boolean);
  }
  
  if (selectedDevices.length === 0) {
    log('❌ No valid devices selected', 'red');
    process.exit(1);
  }
  
  // Download firmware for each device
  for (const device of selectedDevices) {
    log(`\n📥 Downloading firmware for ${device.displayName}...`, 'cyan');
    
    for (const fw of device.firmware) {
      const destPath = path.join(FIRMWARE_DIR, fw.filename);
      
      // Skip if already exists
      if (fs.existsSync(destPath)) {
        log(`  ⏭️  ${fw.filename} already exists, skipping`, 'yellow');
        continue;
      }
      
      log(`  📦 ${fw.filename} (${fw.size})`, 'blue');
      log(`  🌐 ${fw.url}`, 'gray');
      
      try {
        await downloadFile(fw.url, destPath, (percent, mb, totalMB) => {
          process.stdout.write(`\r  ⬇️  Downloading: ${percent}% (${mb}MB / ${totalMB}MB)   `);
        });
        
        log(`\n  ✅ Download complete!`, 'green');
      } catch (err) {
        log(`\n  ❌ Download failed: ${err.message}`, 'red');
      }
    }
  }
  
  log('\n══════════════════════════════════', 'cyan');
  log('✅ Setup complete!', 'green');
  log(`📂 Firmware saved to: ${FIRMWARE_DIR}`, 'yellow');
  log('\nYou can now run AutoRoot offline.', 'cyan');
  log('══════════════════════════════════\n', 'cyan');
}

main().catch((err) => {
  log(`\n❌ Setup failed: ${err.message}`, 'red');
  process.exit(1);
});