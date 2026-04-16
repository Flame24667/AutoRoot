function App() {
    const handleClick = async () => {
        try {
        // window.goAPI is injected by Electron's preload script
        const res = await window.goAPI.call('ping', {});
        alert(`✅ Go responded: ${res}`);
        } catch (err) {
        alert(`❌ Failed: ${err.message}`);
        }
    };

    return (
        <div style={{ padding: '2rem', fontFamily: 'system-ui' }}>
        <h1>React + Electron + Go</h1>
        <p>Click below to call your local Go backend:</p>
        <button 
            onClick={handleClick}
            style={{ padding: '0.5rem 1rem', cursor: 'pointer' }}
        >
            Ping Go Backend
        </button>
        </div>
    );
}

export default App;