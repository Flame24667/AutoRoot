import { useState, useRef } from 'react';
import { useAssetStore } from '../store';

export function AssetImporter() {
  const addAsset = useAssetStore((state) => state.addAsset);
  const [dragActive, setDragActive] = useState(false);
  const inputRef = useRef(null);

  const handleDrag = (e) => {
    e.preventDefault();
    e.stopPropagation();
    if (e.type === 'dragenter' || e.type === 'dragover') {
      setDragActive(true);
    } else if (e.type === 'dragleave') {
      setDragActive(false);
    }
  };

  const handleDrop = (e) => {
    e.preventDefault();
    e.stopPropagation();
    setDragActive(false);

    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      handleFiles(e.dataTransfer.files);
    }
  };

  const handleChange = (e) => {
    e.preventDefault();
    if (e.target.files && e.target.files[0]) {
      handleFiles(e.target.files);
    }
  };

  const handleFiles = (files) => {
    Array.from(files).forEach(file => {
      // Validate file type
      const validTypes = ['.tar', '.img', '.zip', '.md5', '.sha'];
      const fileExt = '.' + file.name.split('.').pop().toLowerCase();
      
      if (validTypes.includes(fileExt)) {
        const asset = {
          id: Date.now() + Math.random(),
          name: file.name,
          path: file.path || `/assets/${file.name}`,
          type: fileExt,
          size: file.size,
          addedAt: new Date(),
          compatibleModels: [] // Will be populated by backend analysis
        };
        addAsset(asset);
      } else {
        alert(`Invalid file type: ${fileExt}. Supported: .tar, .img, .zip, .md5, .sha`);
      }
    });
  };

  return (
    <div className="asset-importer">
      <h3>Local Warehouse - Asset Importer</h3>
      <p className="description">
        Drag and drop firmware files (.tar, .img, .zip) or click to browse
      </p>
      
      <div
        className={`drop-zone ${dragActive ? 'active' : ''}`}
        onDragEnter={handleDrag}
        onDragLeave={handleDrag}
        onDragOver={handleDrag}
        onDrop={handleDrop}
        onClick={() => inputRef.current?.click()}
      >
        <div className="drop-content">
          <span className="icon">📁</span>
          <p>Drop files here or click to browse</p>
          <p className="hint">Supported: .tar (Samsung), .img (Fastboot), .zip (Generic)</p>
        </div>
        <input
          ref={inputRef}
          type="file"
          multiple
          accept=".tar,.img,.zip,.md5,.sha"
          onChange={handleChange}
          style={{ display: 'none' }}
        />
      </div>
    </div>
  );
}

// Asset List Display
export function AssetList() {
  const assets = useAssetStore((state) => state.assets);

  return (
    <div className="asset-list">
      <h3>Available Assets ({assets.length})</h3>
      {assets.length === 0 ? (
        <p className="empty-state">No assets imported yet. Use the importer above.</p>
      ) : (
        <table className="asset-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Type</th>
              <th>Size</th>
              <th>Added</th>
              <th>Compatible Models</th>
            </tr>
          </thead>
          <tbody>
            {assets.map((asset) => (
              <tr key={asset.id}>
                <td>{asset.name}</td>
                <td>
                  <span className={`badge ${asset.type.replace('.', '')}`}>
                    {asset.type}
                  </span>
                </td>
                <td>{formatSize(asset.size)}</td>
                <td>{new Date(asset.addedAt).toLocaleDateString()}</td>
                <td>
                  {asset.compatibleModels?.length > 0 
                    ? asset.compatibleModels.join(', ')
                    : 'Auto-detect'}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

function formatSize(bytes) {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const sizes = ['Bytes', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}
