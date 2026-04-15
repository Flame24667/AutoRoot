// Electron Builder Configuration
module.exports = {
  appId: 'com.autoroot.desktop',
  productName: 'AutoRoot Desktop',
  directories: {
    output: 'dist-electron'
  },
  files: [
    'dist/**/*',
    'electron/**/*'
  ],
  win: {
    target: 'portable',
    icon: 'public/favicon.ico'
  },
  linux: {
    target: 'AppImage',
    category: 'Utility'
  },
  mac: {
    target: 'dmg',
    category: 'public.app-category.utilities'
  }
};
