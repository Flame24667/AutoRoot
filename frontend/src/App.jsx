import { useState, useEffect } from 'react';

function App() {
    const [error, setError] = useState(null);
    const [status, setStatus] = useState('loading');

    useEffect(() => {
        // Test if Go backend is available
        const testBackend = async () => {
        try {
            if (window.goAPI) {
            const result = await window.goAPI.call('ping', {});
            setStatus('ready');
            console.log('✅ Go backend responded:', result);
            } else {
            setStatus('no-api');
            }
        } catch (err) {
            setError(err.message);
            setStatus('error');
        }
        };
        testBackend();
    }, []);

    if (status === 'loading') {
        return (
        <div style={{ padding: '2rem', fontFamily: 'system-ui' }}>
            <h1>🚀 Loading AutoRoot...</h1>
        </div>
        );
    }

    if (status === 'no-api') {
        return (
        <div style={{ padding: '2rem', fontFamily: 'system-ui', color: 'orange' }}>
            <h1>⚠️ window.goAPI not found</h1>
            <p>Running outside Electron or preload failed.</p>
        </div>
        );
    }

    if (status === 'error') {
        return (
        <div style={{ padding: '2rem', fontFamily: 'system-ui', color: 'red' }}>
            <h1>❌ Error</h1>
            <p>{error}</p>
        </div>
        );
    }

    return (
        <div style={{ padding: '2rem', fontFamily: 'system-ui' }}>
        <h1>✅ AutoRoot is Ready!</h1>
        <p>Your React + Electron + Go app is working.</p>
        <button 
            onClick={async () => {
            const result = await window.goAPI.call('ping', {});
            alert('Go says: ' + result);
            }}
            style={{ padding: '0.5rem 1rem', cursor: 'pointer', marginTop: '1rem' }}
        >
            Test Go Backend
        </button>
        </div>
    );
}

export default App;