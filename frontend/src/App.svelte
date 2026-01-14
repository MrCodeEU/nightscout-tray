<script lang="ts">
    import { onMount } from 'svelte';
    import { Events } from "@wailsio/runtime";
    import { NightscoutService } from "../bindings/github.com/mrcode/nightscout-tray/internal/app";
    import Chart from './lib/Chart.svelte';
    import Tray from './Tray.svelte';

    // Hash-based routing for the tray window
    let isTray = window.location.hash === '#/tray';
    
    // State
    let status = null;
    let settings = null;
    let chartData = null;
    let error = null;
    let activeTab = 'dashboard';
    let saving = false;

    // Load data on mount
    onMount(async () => {
        if (isTray) return;

        try {
            settings = await NightscoutService.GetSettings();
            status = await NightscoutService.GetCurrentStatus();
            await refreshChart();
        } catch (err) {
            error = "Failed to load initial data: " + err;
        }

        // Listen for backend events
        Events.On('glucose:update', (event) => {
            status = event.data;
            refreshChart();
        });

        Events.On('glucose:error', (event) => {
            error = event.data;
        });
    });

    async function refreshChart() {
        if (settings) {
            try {
                chartData = await NightscoutService.GetChartData(settings.chartTimeRange, 0);
            } catch (err) {
                console.error("Chart refresh error:", err);
            }
        }
    }

    async function saveSettings() {
        saving = true;
        try {
            await NightscoutService.SaveSettings(settings);
            activeTab = 'dashboard';
            await refreshChart();
        } catch (err) {
            error = "Failed to save settings: " + err;
        } finally {
            saving = false;
        }
    }

    // Helpers
    const getStatusColor = (s) => {
        if (!s) return 'var(--color-gray)';
        switch (s.status) {
            case 'urgent_low': case 'urgent_high': return 'var(--color-red)';
            case 'low': return 'var(--color-orange)';
            case 'high': return 'var(--color-yellow)';
            default: return 'var(--color-green)';
        }
    };

    const formatTime = (date) => {
        if (!date) return '--:--';
        return new Date(date).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    };
</script>

{#if isTray}
    <Tray />
{:else}
    <div class="shell">
        <aside class="sidebar">
            <div class="brand">
                <div class="brand-icon">⚡</div>
                <h1>Nightscout</h1>
            </div>
            
            <nav>
                <button class:active={activeTab === 'dashboard'} on:click={() => activeTab = 'dashboard'}>
                    <span class="icon">📊</span> Dashboard
                </button>
                <button class:active={activeTab === 'settings'} on:click={() => activeTab = 'settings'}>
                    <span class="icon">⚙️</span> Settings
                </button>
            </nav>

            <div class="version">v3.0.0</div>
        </aside>

        <main class="main-content">
            {#if activeTab === 'dashboard'}
                <div class="view dashboard">
                    <div class="glance-card">
                        <div class="glucose-circle" style="border-color: {getStatusColor(status)}">
                            <span class="value" style="color: {getStatusColor(status)}">
                                {status ? (settings?.unit === 'mmol/L' ? status.valueMmol.toFixed(1) : status.value) : '--'}
                            </span>
                            <span class="trend">{status?.trend || ''}</span>
                        </div>
                        <div class="info">
                            <div class="time">Updated {status ? formatTime(status.time) : '--:--'}</div>
                            <div class="delta">{status && status.delta ? (status.delta > 0 ? '+' + status.delta : status.delta) : ''}</div>
                        </div>
                    </div>

                    <div class="chart-panel">
                        {#if chartData}
                            <Chart data={chartData} {settings} />
                        {:else}
                            <div class="loading">Loading Chart...</div>
                        {/if}
                    </div>
                </div>
            {/if}

            {#if activeTab === 'settings'}
                <div class="view settings">
                    <h2>Configuration</h2>
                    {#if settings}
                        <div class="form-grid">
                            <section>
                                <h3>Connection</h3>
                                <label>
                                    <span>Nightscout URL</span>
                                    <input type="text" bind:value={settings.nightscoutUrl} placeholder="https://..." />
                                </label>
                                <label>
                                    <span>API Secret / Token</span>
                                    <input type="password" bind:value={settings.apiSecret} />
                                </label>
                                <label class="checkbox">
                                    <input type="checkbox" bind:checked={settings.useToken} />
                                    <span>Use Token Authentication</span>
                                </label>
                            </section>

                            <section>
                                <h3>Display</h3>
                                <label>
                                    <span>Unit</span>
                                    <select bind:value={settings.unit}>
                                        <option value="mg/dL">mg/dL</option>
                                        <option value="mmol/L">mmol/L</option>
                                    </select>
                                </label>
                                <label>
                                    <span>Refresh Rate (sec)</span>
                                    <input type="number" bind:value={settings.refreshInterval} min="30" />
                                </label>
                            </section>

                            <section>
                                <h3>Thresholds</h3>
                                <div class="row">
                                    <label><span>Low</span><input type="number" bind:value={settings.targetLow} /></label>
                                    <label><span>High</span><input type="number" bind:value={settings.targetHigh} /></label>
                                </div>
                                <div class="row">
                                    <label><span>U. Low</span><input type="number" bind:value={settings.urgentLow} /></label>
                                    <label><span>U. High</span><input type="number" bind:value={settings.urgentHigh} /></label>
                                </div>
                            </section>

                            <section>
                                <h3>Notifications</h3>
                                <label class="checkbox">
                                    <input type="checkbox" bind:checked={settings.enableHighAlert} />
                                    <span>High Alert</span>
                                </label>
                                <label class="checkbox">
                                    <input type="checkbox" bind:checked={settings.enableLowAlert} />
                                    <span>Low Alert</span>
                                </label>
                                <label class="checkbox">
                                    <input type="checkbox" bind:checked={settings.enableUrgentHighAlert} />
                                    <span>Urgent High Alert</span>
                                </label>
                                <label class="checkbox">
                                    <input type="checkbox" bind:checked={settings.enableUrgentLowAlert} />
                                    <span>Urgent Low Alert</span>
                                </label>
                                <label class="checkbox">
                                    <input type="checkbox" bind:checked={settings.enableSoundAlerts} />
                                    <span>Sound Alerts</span>
                                </label>
                                <label>
                                    <span>Repeat Alert (min, 0=off)</span>
                                    <input type="number" bind:value={settings.repeatAlertMinutes} min="0" />
                                </label>
                            </section>

                            <section>
                                <h3>Chart Colors</h3>
                                <div class="color-row">
                                    <label class="color-picker">
                                        <input type="color" bind:value={settings.chartColorInRange} />
                                        <span>In Range</span>
                                    </label>
                                    <label class="color-picker">
                                        <input type="color" bind:value={settings.chartColorHigh} />
                                        <span>High</span>
                                    </label>
                                </div>
                                <div class="color-row">
                                    <label class="color-picker">
                                        <input type="color" bind:value={settings.chartColorLow} />
                                        <span>Low</span>
                                    </label>
                                    <label class="color-picker">
                                        <input type="color" bind:value={settings.chartColorUrgent} />
                                        <span>Urgent</span>
                                    </label>
                                </div>
                            </section>

                            <section>
                                <h3>Chart Options</h3>
                                <label>
                                    <span>Time Range (hours)</span>
                                    <input type="number" bind:value={settings.chartTimeRange} min="1" max="48" />
                                </label>
                                <label>
                                    <span>Style</span>
                                    <select bind:value={settings.chartStyle}>
                                        <option value="line">Line</option>
                                        <option value="points">Points</option>
                                        <option value="both">Both</option>
                                    </select>
                                </label>
                                <label class="checkbox">
                                    <input type="checkbox" bind:checked={settings.chartShowTarget} />
                                    <span>Show Target Range</span>
                                </label>
                                <label class="checkbox">
                                    <input type="checkbox" bind:checked={settings.chartShowNow} />
                                    <span>Show "Now" Marker</span>
                                </label>
                            </section>

                            <section>
                                <h3>System</h3>
                                <label class="checkbox">
                                    <input type="checkbox" bind:checked={settings.autoStart} />
                                    <span>Start with Windows</span>
                                </label>
                                <label class="checkbox">
                                    <input type="checkbox" bind:checked={settings.startMinimized} />
                                    <span>Start Minimized</span>
                                </label>
                            </section>
                        </div>
                        <div class="actions">
                            <button class="save-btn" on:click={saveSettings} disabled={saving}>
                                {saving ? 'Saving...' : 'Save Changes'}
                            </button>
                        </div>
                    {/if}
                </div>
            {/if}

            {#if error}
                <button class="toast" on:click={() => error = null}>{error}</button>
            {/if}
        </main>
    </div>
{/if}

<style>
    /* Design Tokens */
    :global(:root) {
        --bg-dark: #0f172a;
        --bg-panel: #1e293b;
        --bg-input: #020617;
        --text-main: #f1f5f9;
        --text-dim: #94a3b8;
        --color-green: #4ade80;
        --color-yellow: #facc15;
        --color-orange: #fb923c;
        --color-red: #ef4444;
        --color-blue: #3b82f6;
        --color-gray: #64748b;
    }

    :global(body) {
        margin: 0;
        background: var(--bg-dark);
        color: var(--text-main);
        font-family: 'Segoe UI', sans-serif;
        overflow: hidden;
    }

    .shell {
        display: flex;
        height: 100vh;
    }

    /* Sidebar */
    .sidebar {
        width: 220px;
        background: var(--bg-panel);
        padding: 20px;
        display: flex;
        flex-direction: column;
        border-right: 1px solid #334155;
    }

    .brand {
        display: flex;
        align-items: center;
        gap: 10px;
        margin-bottom: 40px;
    }

    .brand-icon {
        background: var(--color-blue);
        width: 32px;
        height: 32px;
        border-radius: 8px;
        display: flex;
        align-items: center;
        justify-content: center;
        font-weight: bold;
    }

    .brand h1 {
        font-size: 1.2rem;
        margin: 0;
    }

    nav button {
        width: 100%;
        text-align: left;
        background: none;
        border: none;
        color: var(--text-dim);
        padding: 12px;
        cursor: pointer;
        border-radius: 8px;
        font-size: 1rem;
        display: flex;
        gap: 10px;
        transition: 0.2s;
    }

    nav button:hover {
        background: #334155;
        color: white;
    }

    nav button.active {
        background: var(--color-blue);
        color: white;
    }

    .version {
        margin-top: auto;
        text-align: center;
        color: var(--text-dim);
        font-size: 0.8rem;
    }

    /* Main Content */
    .main-content {
        flex: 1;
        padding: 30px;
        overflow-y: auto;
    }

    .view {
        max-width: 900px;
        margin: 0 auto;
        display: flex;
        flex-direction: column;
        gap: 20px;
    }

    /* Dashboard */
    .glance-card {
        background: var(--bg-panel);
        border-radius: 16px;
        padding: 30px;
        display: flex;
        align-items: center;
        justify-content: space-around;
        border: 1px solid #334155;
    }

    .glucose-circle {
        width: 150px;
        height: 150px;
        border-radius: 50%;
        border: 8px solid var(--color-gray);
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        position: relative;
    }

    .glucose-circle .value {
        font-size: 3rem;
        font-weight: bold;
        line-height: 1;
    }

    .glucose-circle .trend {
        font-size: 1.5rem;
        color: var(--text-dim);
    }

    .info {
        text-align: center;
    }

    .info .time {
        font-size: 1.2rem;
        color: var(--text-dim);
    }

    .info .delta {
        font-size: 1.5rem;
        font-weight: bold;
        color: var(--text-main);
    }

    .chart-panel {
        background: var(--bg-panel);
        border-radius: 16px;
        padding: 20px;
        height: 350px;
        border: 1px solid #334155;
    }

    /* Settings */
    .form-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
        gap: 20px;
    }

    section {
        background: var(--bg-panel);
        padding: 20px;
        border-radius: 12px;
        border: 1px solid #334155;
    }

    section h3 {
        margin-top: 0;
        color: var(--color-blue);
        font-size: 1.1rem;
        margin-bottom: 15px;
    }

    label {
        display: block;
        margin-bottom: 12px;
    }

    label span {
        display: block;
        font-size: 0.9rem;
        color: var(--text-dim);
        margin-bottom: 5px;
    }

    input[type="text"], input[type="password"], input[type="number"], select {
        width: 100%;
        background: var(--bg-input);
        border: 1px solid #475569;
        color: white;
        padding: 10px;
        border-radius: 6px;
        box-sizing: border-box;
    }

    .row {
        display: flex;
        gap: 10px;
    }

    .checkbox {
        flex-direction: row;
        align-items: center;
        gap: 10px;
        cursor: pointer;
    }
    
    .checkbox span { margin: 0; }
    .checkbox input { width: auto; }

    .color-row {
        display: flex;
        gap: 15px;
        margin-bottom: 10px;
    }

    .color-picker {
        display: flex;
        align-items: center;
        gap: 8px;
        flex: 1;
    }

    .color-picker input[type="color"] {
        width: 40px;
        height: 30px;
        padding: 0;
        border: 1px solid #475569;
        border-radius: 4px;
        cursor: pointer;
        background: transparent;
    }

    .color-picker span {
        margin: 0;
        font-size: 0.85rem;
    }

    .actions {
        display: flex;
        justify-content: flex-end;
    }

    .save-btn {
        background: var(--color-blue);
        color: white;
        border: none;
        padding: 12px 30px;
        border-radius: 8px;
        font-size: 1rem;
        cursor: pointer;
    }

    .save-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .toast {
        position: fixed;
        bottom: 20px;
        right: 20px;
        background: var(--color-red);
        color: white;
        padding: 15px 25px;
        border-radius: 8px;
        cursor: pointer;
        box-shadow: 0 5px 15px rgba(0,0,0,0.3);
    }

    .loading {
        height: 100%;
        display: flex;
        align-items: center;
        justify-content: center;
        color: var(--text-dim);
    }
</style>