import { useState, useEffect } from 'react';

function App() {
    const [device, setDevice] = useState({
        brand: 'Detecting...',
        model: '...',
        androidVersion: '...',
        rooted: null, // null = unknown, true/false = known
        status: 'connecting', // connecting, ready, error, rooting, success
        message: ''
    });

    const [isDetecting, setIsDetecting] = useState(true);

    // Detect connected device on mount
    useEffect(() => {
        detectDevice();
    }, []);

    const detectDevice = async () => {
        setIsDetecting(true);
        try {
        if (!window.goAPI) {
            throw new Error('Backend not available');
        }
        
        const result = await window.goAPI.call('detectDevice', {});
        setDevice(prev => ({
            ...prev,
            brand: result.brand || 'Unknown',
            model: result.model || 'Unknown',
            androidVersion: result.androidVersion || 'Unknown',
            rooted: result.rooted,
            status: 'ready',
            message: result.rooted ? 'Device is already rooted ✓' : 'Device is not rooted'
        }));
        } catch (err) {
        setDevice(prev => ({
            ...prev,
            status: 'error',
            message: `Failed to detect device: ${err.message}`
        }));
        } finally {
        setIsDetecting(false);
        }
    };

    const handleRoot = async () => {
        if (device.rooted) {
        setDevice(prev => ({ ...prev, message: 'Device is already rooted!' }));
        return;
        }

        setDevice(prev => ({ ...prev, status: 'rooting', message: 'Starting root process...' }));
        
        try {
        const result = await window.goAPI.call('rootDevice', {
            model: device.model,
            androidVersion: device.androidVersion
        });
        
        setDevice(prev => ({
            ...prev,
            rooted: true,
            status: 'success',
            message: result.message || 'Root completed successfully! ✓'
        }));
        
        // Auto-refresh device status after 2 seconds
        setTimeout(detectDevice, 2000);
        } catch (err) {
        setDevice(prev => ({
            ...prev,
            status: 'error',
            message: `Root failed: ${err.message}`
        }));
        }
    };

    // Status badge colors
    const getStatusColor = () => {
        switch (device.status) {
        case 'ready': return device.rooted ? '#22c55e' : '#f59e0b';
        case 'rooting': return '#3b82f6';
        case 'success': return '#22c55e';
        case 'error': return '#ef4444';
        default: return '#6b7280';
        }
    };

    return (
        <div style={styles.container}>
        {/* Header */}
        <header style={styles.header}>
            <h1 style={styles.title}>🔓 AutoRoot</h1>
            <p style={styles.subtitle}>One-click root automation for Android</p>
        </header>

        {/* Device Card */}
        <div style={styles.card}>
            <div style={styles.cardHeader}>
            <h2 style={styles.cardTitle}>Connected Device</h2>
            <span 
                style={{
                ...styles.badge,
                backgroundColor: getStatusColor()
                }}
            >
                {device.status === 'rooting' ? 'Rooting...' : 
                device.rooted ? 'Rooted ✓' : 
                device.status === 'error' ? 'Error' : 'Not Rooted'}
            </span>
            </div>

            {isDetecting ? (
            <div style={styles.loading}>
                <div style={styles.spinner}></div>
                <p>Detecting device...</p>
            </div>
            ) : (
            <div style={styles.deviceInfo}>
                <InfoRow label="Brand" value={device.brand} />
                <InfoRow label="Model" value={device.model} />
                <InfoRow label="Android Version" value={device.androidVersion} />
                <InfoRow 
                label="Root Status" 
                value={device.rooted ? 'Yes ✓' : 'No'} 
                highlight={device.rooted}
                />
            </div>
            )}

            {/* Message */}
            {device.message && (
            <div style={{
                ...styles.message,
                backgroundColor: device.status === 'error' ? '#fef2f2' : '#f0f9ff',
                borderLeft: `4px solid ${device.status === 'error' ? '#ef4444' : '#3b82f6'}`
            }}>
                {device.message}
            </div>
            )}

            {/* Root Button */}
            <button
            onClick={handleRoot}
            disabled={isDetecting || device.status === 'rooting' || device.rooted || device.status === 'error'}
            style={{
                ...styles.rootButton,
                opacity: (isDetecting || device.status === 'rooting' || device.rooted) ? 0.6 : 1,
                cursor: (isDetecting || device.status === 'rooting' || device.rooted) ? 'not-allowed' : 'pointer'
            }}
            >
            {device.rooted ? '✓ Already Rooted' : 
            device.status === 'rooting' ? 'Rooting...' : 
            '🔓 Root Device'}
            </button>

            {/* Retry Button */}
            {device.status === 'error' && (
            <button 
                onClick={detectDevice}
                style={styles.retryButton}
            >
                🔁 Retry Detection
            </button>
            )}
        </div>

        {/* Footer */}
        <footer style={styles.footer}>
            <p>AutoRoot v1.0 • Offline • No data collected</p>
        </footer>
        </div>
    );
}

// Reusable info row component
function InfoRow({ label, value, highlight = false }) {
    return (
        <div style={styles.infoRow}>
        <span style={styles.infoLabel}>{label}</span>
        <span style={{
            ...styles.infoValue,
            color: highlight ? '#22c55e' : 'inherit',
            fontWeight: highlight ? '600' : '400'
        }}>
            {value}
        </span>
        </div>
    );
}

// Styles (inline for simplicity - move to App.css in production)
const styles = {
    container: {
        minHeight: '100vh',
        background: 'linear-gradient(135deg, #1e293b 0%, #0f172a 100%)',
        color: '#f1f5f9',
        fontFamily: 'system-ui, -apple-system, sans-serif',
        padding: '2rem',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
    },
    header: {
        textAlign: 'center',
        marginBottom: '2rem',
    },
    title: {
        fontSize: '2.5rem',
        fontWeight: '700',
        margin: '0 0 0.5rem 0',
        background: 'linear-gradient(90deg, #60a5fa, #a78bfa)',
        WebkitBackgroundClip: 'text',
        WebkitTextFillColor: 'transparent',
    },
    subtitle: {
        color: '#94a3b8',
        margin: 0,
        fontSize: '1.1rem',
    },
    card: {
        background: '#1e293b',
        borderRadius: '16px',
        padding: '1.5rem',
        width: '100%',
        maxWidth: '500px',
        boxShadow: '0 10px 40px rgba(0, 0, 0, 0.3)',
        border: '1px solid #334155',
    },
    cardHeader: {
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        marginBottom: '1.5rem',
        paddingBottom: '1rem',
        borderBottom: '1px solid #334155',
    },
    cardTitle: {
        margin: 0,
        fontSize: '1.25rem',
        fontWeight: '600',
    },
    badge: {
        padding: '0.35rem 0.75rem',
        borderRadius: '9999px',
        fontSize: '0.875rem',
        fontWeight: '500',
        color: 'white',
    },
    loading: {
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        padding: '2rem 0',
        color: '#94a3b8',
    },
    spinner: {
        width: '32px',
        height: '32px',
        border: '3px solid #334155',
        borderTop: '3px solid #60a5fa',
        borderRadius: '50%',
        animation: 'spin 1s linear infinite',
        marginBottom: '1rem',
    },
    deviceInfo: {
        display: 'flex',
        flexDirection: 'column',
        gap: '0.75rem',
        marginBottom: '1.5rem',
    },
    infoRow: {
        display: 'flex',
        justifyContent: 'space-between',
        padding: '0.5rem 0',
        borderBottom: '1px dashed #334155',
    },
    infoLabel: {
        color: '#94a3b8',
        fontSize: '0.95rem',
    },
    infoValue: {
        fontSize: '0.95rem',
        fontWeight: '500',
    },
    message: {
        padding: '0.75rem 1rem',
        borderRadius: '8px',
        marginBottom: '1.5rem',
        fontSize: '0.9rem',
        color: '#1e40af',
    },
    rootButton: {
        width: '100%',
        padding: '1rem',
        background: 'linear-gradient(135deg, #3b82f6, #6366f1)',
        color: 'white',
        border: 'none',
        borderRadius: '12px',
        fontSize: '1.1rem',
        fontWeight: '600',
        cursor: 'pointer',
        transition: 'transform 0.1s, box-shadow 0.2s',
        boxShadow: '0 4px 14px rgba(59, 130, 246, 0.4)',
    },
    retryButton: {
        width: '100%',
        padding: '0.75rem',
        marginTop: '0.75rem',
        background: 'transparent',
        color: '#94a3b8',
        border: '1px solid #475569',
        borderRadius: '8px',
        fontSize: '0.95rem',
        cursor: 'pointer',
    },
    footer: {
        marginTop: 'auto',
        paddingTop: '2rem',
        color: '#64748b',
        fontSize: '0.875rem',
        textAlign: 'center',
    },
};

// Add CSS animation for spinner
const style = document.createElement('style');
style.textContent = `
    @keyframes spin {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
    }
`;
document.head.appendChild(style);

export default App;