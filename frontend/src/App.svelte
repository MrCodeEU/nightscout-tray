<script lang="ts">
    import { onMount } from 'svelte';
    import { Events } from "@wailsio/runtime";
    import { NightscoutService } from "../bindings/github.com/mrcode/nightscout-tray/internal/app";
    import Chart from './lib/Chart.svelte';
    import Tray from './Tray.svelte';

    // Hash-based routing for the tray window
    let isTray = window.location.hash === '#/tray';

    // State - use 'any' for Wails-generated types to avoid class vs interface conflicts
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let status: any = null;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let settings: any = null;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let chartData: any = null;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let predictionData: any = null;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let diabetesParams: any = null;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let calculationProgress: any = null;
    let error: string | null = null;
    let activeTab = 'dashboard';
    let saving = false;
    let showLongTermPrediction = false;
    let isCalculating = false;
    let calculationDays = 30;
    let progressInterval: ReturnType<typeof setInterval> | null = null;

    // Load data on mount
    onMount(async () => {
        if (isTray) return;

        try {
            settings = await NightscoutService.GetSettings();
            status = await NightscoutService.GetCurrentStatus();
            diabetesParams = await NightscoutService.GetPredictionParameters();
            await refreshChart();
            await refreshPrediction();
        } catch (err) {
            error = "Failed to load initial data: " + err;
        }

        // Listen for backend events
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        Events.On('glucose:update', (event: any) => {
            status = event.data;
            refreshChart();
            refreshPrediction();
        });

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        Events.On('glucose:error', (event: any) => {
            error = event.data;
        });
    });

    async function refreshChart(): Promise<void> {
        if (settings) {
            try {
                chartData = await NightscoutService.GetChartData(settings.chartTimeRange || 4, 0);
            } catch (err) {
                console.error("Chart refresh error:", err);
            }
        }
    }

    async function refreshPrediction(): Promise<void> {
        try {
            predictionData = await NightscoutService.GetChartPredictionData(showLongTermPrediction);
        } catch (err) {
            console.error("Prediction refresh error:", err);
        }
    }

    async function startCalculation(): Promise<void> {
        isCalculating = true;
        calculationProgress = { stage: 'Starting...', progress: 0 };
        
        try {
            await NightscoutService.StartParameterCalculation(calculationDays);
            
            // Poll for progress
            progressInterval = setInterval(async () => {
                try {
                    const stillCalc = await NightscoutService.IsCalculating();
                    if (stillCalc) {
                        calculationProgress = await NightscoutService.GetCalculationProgress();
                    } else {
                        // Calculation complete
                        if (progressInterval) clearInterval(progressInterval);
                        progressInterval = null;
                        isCalculating = false;
                        diabetesParams = await NightscoutService.GetPredictionParameters();
                        await refreshPrediction();
                    }
                } catch (e) {
                    console.error("Progress poll error:", e);
                }
            }, 500);
        } catch (err) {
            error = "Failed to start calculation: " + err;
            isCalculating = false;
        }
    }

    async function cancelCalculation(): Promise<void> {
        try {
            await NightscoutService.CancelCalculation();
            if (progressInterval) clearInterval(progressInterval);
            progressInterval = null;
            isCalculating = false;
        } catch (err) {
            console.error("Cancel error:", err);
        }
    }

    async function toggleLongTerm(): Promise<void> {
        showLongTermPrediction = !showLongTermPrediction;
        await refreshPrediction();
    }

    async function saveSettings(): Promise<void> {
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
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const getStatusColor = (s: any): string => {
        if (!s) return 'var(--color-gray)';
        switch (s.status) {
            case 'urgent_low': case 'urgent_high': return 'var(--color-red)';
            case 'low': return 'var(--color-orange)';
            case 'high': return 'var(--color-yellow)';
            default: return 'var(--color-green)';
        }
    };

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const formatTime = (date: any): string => {
        if (!date) return '--:--';
        return new Date(date).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    };

    const formatNumber = (n: number | undefined, decimals: number = 1): string => {
        if (n === undefined || n === null) return '--';
        return n.toFixed(decimals);
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
                <button class:active={activeTab === 'predictions'} on:click={() => activeTab = 'predictions'}>
                    <span class="icon">🔮</span> Predictions
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
                        {#if predictionData}
                            <div class="iob-cob">
                                <span class="iob" title="Insulin on Board">💉 IOB: {formatNumber(predictionData.iob, 2)}u</span>
                                <span class="cob" title="Carbs on Board">🍞 COB: {formatNumber(predictionData.cob, 0)}g</span>
                            </div>
                        {/if}
                    </div>

                    <div class="chart-controls">
                        <label class="toggle">
                            <input type="checkbox" bind:checked={showLongTermPrediction} on:change={toggleLongTerm} />
                            <span>Show Long-term Predictions (6h)</span>
                        </label>
                    </div>

                    <div class="chart-panel">
                        {#if chartData}
                            <Chart data={chartData} {settings} {predictionData} {showLongTermPrediction} />
                        {:else}
                            <div class="loading">Loading Chart...</div>
                        {/if}
                    </div>
                </div>
            {/if}

            {#if activeTab === 'predictions'}
                <div class="view predictions">
                    <h2>AI Predictions & Diabetes Parameters</h2>
                    
                    <section class="calc-section">
                        <h3>Calculate Parameters</h3>
                        <p class="description">Analyze your historical data to calculate personalized diabetes parameters. This process learns from your glucose readings, insulin doses, and carb intake.</p>
                        
                        <div class="calc-controls">
                            <label>
                                <span>Analyze last</span>
                                <select bind:value={calculationDays} disabled={isCalculating}>
                                    <option value={7}>7 days</option>
                                    <option value={14}>14 days</option>
                                    <option value={30}>30 days</option>
                                    <option value={60}>60 days</option>
                                    <option value={90}>90 days</option>
                                </select>
                            </label>
                            
                            {#if !isCalculating}
                                <button class="calc-btn" on:click={startCalculation}>
                                    🧮 Calculate Parameters
                                </button>
                            {:else}
                                <button class="cancel-btn" on:click={cancelCalculation}>
                                    ❌ Cancel
                                </button>
                            {/if}
                        </div>

                        {#if isCalculating && calculationProgress}
                            <div class="progress-section">
                                <div class="progress-bar">
                                    <div class="progress-fill" style="width: {calculationProgress.progress}%"></div>
                                </div>
                                <div class="progress-info">
                                    <span class="stage">{calculationProgress.stage}</span>
                                    <span class="percent">{calculationProgress.progress?.toFixed(0)}%</span>
                                    {#if calculationProgress.estimatedTimeRemaining > 0}
                                        <span class="eta">~{Math.ceil(calculationProgress.estimatedTimeRemaining)}s remaining</span>
                                    {/if}
                                </div>
                            </div>
                        {/if}
                    </section>

                    {#if diabetesParams}
                        <section class="params-section">
                            <h3>Calculated Parameters</h3>
                            <div class="params-grid">
                                <div class="param-card">
                                    <div class="param-label">Insulin Sensitivity Factor (ISF)</div>
                                    <div class="param-value">{formatNumber(diabetesParams.isf, 0)} <span class="unit">mg/dL per unit</span></div>
                                    <div class="param-desc">1 unit of insulin lowers your blood glucose by this amount</div>
                                    <div class="confidence" style="--conf: {diabetesParams.isfConfidence}%">
                                        Confidence: {formatNumber(diabetesParams.isfConfidence, 0)}%
                                    </div>
                                </div>

                                <div class="param-card">
                                    <div class="param-label">Insulin-to-Carb Ratio (ICR)</div>
                                    <div class="param-value">1:{formatNumber(diabetesParams.icr, 0)} <span class="unit">unit per {formatNumber(diabetesParams.icr, 0)}g carbs</span></div>
                                    <div class="param-desc">Grams of carbohydrates covered by 1 unit of insulin</div>
                                    <div class="confidence" style="--conf: {diabetesParams.icrConfidence}%">
                                        Confidence: {formatNumber(diabetesParams.icrConfidence, 0)}%
                                    </div>
                                </div>

                                <div class="param-card">
                                    <div class="param-label">Duration of Insulin Action (DIA)</div>
                                    <div class="param-value">{formatNumber(diabetesParams.dia, 1)} <span class="unit">hours</span></div>
                                    <div class="param-desc">How long insulin remains active in your body</div>
                                    <div class="confidence" style="--conf: {diabetesParams.diaConfidence}%">
                                        Confidence: {formatNumber(diabetesParams.diaConfidence, 0)}%
                                    </div>
                                </div>

                                <div class="param-card">
                                    <div class="param-label">Carb Absorption Rate</div>
                                    <div class="param-value">{formatNumber(diabetesParams.carbAbsorptionRate, 0)} <span class="unit">g/hour</span></div>
                                    <div class="param-desc">Average rate at which carbs are absorbed</div>
                                </div>
                            </div>
                        </section>

                        <section class="time-of-day-section">
                            <h3>Time-of-Day Variations</h3>
                            <div class="tod-grid">
                                <div class="tod-card">
                                    <div class="tod-label">🌅 Morning (6-11)</div>
                                    <div class="tod-values">
                                        <span>ISF: {formatNumber(diabetesParams.isfByTimeOfDay?.morning, 0)}</span>
                                        <span>ICR: 1:{formatNumber(diabetesParams.icrByTimeOfDay?.morning, 0)}</span>
                                    </div>
                                </div>
                                <div class="tod-card">
                                    <div class="tod-label">☀️ Midday (11-17)</div>
                                    <div class="tod-values">
                                        <span>ISF: {formatNumber(diabetesParams.isfByTimeOfDay?.midday, 0)}</span>
                                        <span>ICR: 1:{formatNumber(diabetesParams.icrByTimeOfDay?.midday, 0)}</span>
                                    </div>
                                </div>
                                <div class="tod-card">
                                    <div class="tod-label">🌆 Evening (17-22)</div>
                                    <div class="tod-values">
                                        <span>ISF: {formatNumber(diabetesParams.isfByTimeOfDay?.evening, 0)}</span>
                                        <span>ICR: 1:{formatNumber(diabetesParams.icrByTimeOfDay?.evening, 0)}</span>
                                    </div>
                                </div>
                                <div class="tod-card">
                                    <div class="tod-label">🌙 Night (22-6)</div>
                                    <div class="tod-values">
                                        <span>ISF: {formatNumber(diabetesParams.isfByTimeOfDay?.night, 0)}</span>
                                        <span>ICR: 1:{formatNumber(diabetesParams.icrByTimeOfDay?.night, 0)}</span>
                                    </div>
                                </div>
                            </div>
                        </section>

                        <section class="stats-section">
                            <h3>Glucose Statistics</h3>
                            <div class="stats-grid">
                                <div class="stat-item">
                                    <span class="stat-label">Average Glucose</span>
                                    <span class="stat-value">{formatNumber(diabetesParams.averageGlucose, 0)} mg/dL</span>
                                </div>
                                <div class="stat-item">
                                    <span class="stat-label">GMI (est. A1C)</span>
                                    <span class="stat-value">{formatNumber(diabetesParams.gmi, 1)}%</span>
                                </div>
                                <div class="stat-item">
                                    <span class="stat-label">Time in Range</span>
                                    <span class="stat-value tir">{formatNumber(diabetesParams.timeInRange, 0)}%</span>
                                </div>
                                <div class="stat-item">
                                    <span class="stat-label">Time Below Range</span>
                                    <span class="stat-value tbr">{formatNumber(diabetesParams.timeBelowRange, 1)}%</span>
                                </div>
                                <div class="stat-item">
                                    <span class="stat-label">Time Above Range</span>
                                    <span class="stat-value tar">{formatNumber(diabetesParams.timeAboveRange, 0)}%</span>
                                </div>
                                <div class="stat-item">
                                    <span class="stat-label">Coefficient of Variation</span>
                                    <span class="stat-value">{formatNumber(diabetesParams.coefficientOfVariation, 1)}%</span>
                                </div>
                                <div class="stat-item">
                                    <span class="stat-label">Daily Insulin (avg)</span>
                                    <span class="stat-value">{formatNumber(diabetesParams.totalDailyInsulin, 1)} units</span>
                                </div>
                                <div class="stat-item">
                                    <span class="stat-label">Daily Carbs (avg)</span>
                                    <span class="stat-value">{formatNumber(diabetesParams.totalDailyCarbs, 0)}g</span>
                                </div>
                            </div>
                            
                            <div class="data-info">
                                <span>Based on {diabetesParams.dataDays} days of data</span>
                                <span>•</span>
                                <span>{diabetesParams.entriesAnalyzed} glucose readings</span>
                                <span>•</span>
                                <span>{diabetesParams.treatmentsAnalyzed} treatments</span>
                            </div>
                        </section>
                    {:else}
                        <section class="no-params">
                            <p>No calculated parameters yet. Click "Calculate Parameters" to analyze your data.</p>
                        </section>
                    {/if}
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

    /* IOB/COB Display */
    .iob-cob {
        display: flex;
        gap: 20px;
        margin-top: 15px;
        font-size: 0.9rem;
        color: var(--text-dim);
    }

    .iob-cob span {
        padding: 5px 10px;
        background: rgba(59, 130, 246, 0.1);
        border-radius: 6px;
        border: 1px solid rgba(59, 130, 246, 0.3);
    }

    /* Chart Controls */
    .chart-controls {
        display: flex;
        justify-content: flex-end;
        margin-bottom: 10px;
    }

    .chart-controls .toggle {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 0.85rem;
        color: var(--text-dim);
        cursor: pointer;
    }

    .chart-controls .toggle input {
        width: auto;
    }

    /* Predictions Tab Styles */
    .predictions {
        padding: 20px;
        overflow-y: auto;
    }

    .predictions h2 {
        margin-bottom: 25px;
    }

    .predictions h3 {
        color: var(--text-main);
        margin-bottom: 15px;
        font-size: 1.1rem;
    }

    .predictions section {
        background: var(--bg-panel);
        border-radius: 12px;
        padding: 20px;
        margin-bottom: 20px;
    }

    .description {
        color: var(--text-dim);
        margin-bottom: 20px;
        line-height: 1.6;
    }

    /* Calculation Controls */
    .calc-controls {
        display: flex;
        align-items: center;
        gap: 20px;
        flex-wrap: wrap;
    }

    .calc-controls label {
        display: flex;
        align-items: center;
        gap: 10px;
    }

    .calc-controls select {
        width: auto;
        padding: 8px 15px;
    }

    .calc-btn, .cancel-btn {
        padding: 10px 25px;
        border-radius: 8px;
        border: none;
        font-size: 1rem;
        cursor: pointer;
        transition: all 0.2s;
    }

    .calc-btn {
        background: var(--color-blue);
        color: white;
    }

    .calc-btn:hover {
        background: #2563eb;
    }

    .cancel-btn {
        background: var(--color-red);
        color: white;
    }

    /* Progress Section */
    .progress-section {
        margin-top: 20px;
    }

    .progress-bar {
        height: 8px;
        background: var(--bg-input);
        border-radius: 4px;
        overflow: hidden;
    }

    .progress-fill {
        height: 100%;
        background: linear-gradient(90deg, var(--color-blue), var(--color-green));
        transition: width 0.3s ease;
    }

    .progress-info {
        display: flex;
        gap: 20px;
        margin-top: 10px;
        font-size: 0.9rem;
        color: var(--text-dim);
    }

    .progress-info .stage {
        color: var(--text-main);
    }

    .progress-info .percent {
        color: var(--color-blue);
        font-weight: bold;
    }

    /* Parameter Cards */
    .params-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
        gap: 20px;
    }

    .param-card {
        background: var(--bg-dark);
        border-radius: 10px;
        padding: 20px;
        border: 1px solid #334155;
    }

    .param-label {
        font-size: 0.85rem;
        color: var(--text-dim);
        margin-bottom: 8px;
    }

    .param-value {
        font-size: 1.8rem;
        font-weight: bold;
        color: var(--color-green);
        margin-bottom: 8px;
    }

    .param-value .unit {
        font-size: 0.8rem;
        font-weight: normal;
        color: var(--text-dim);
    }

    .param-desc {
        font-size: 0.8rem;
        color: var(--text-dim);
        margin-bottom: 12px;
    }

    .confidence {
        font-size: 0.75rem;
        color: var(--text-dim);
        padding: 4px 8px;
        background: rgba(74, 222, 128, calc(var(--conf) / 100 * 0.2));
        border-radius: 4px;
        display: inline-block;
    }

    /* Time of Day Grid */
    .tod-grid {
        display: grid;
        grid-template-columns: repeat(4, 1fr);
        gap: 15px;
    }

    .tod-card {
        background: var(--bg-dark);
        border-radius: 10px;
        padding: 15px;
        text-align: center;
        border: 1px solid #334155;
    }

    .tod-label {
        font-size: 0.9rem;
        margin-bottom: 10px;
    }

    .tod-values {
        display: flex;
        flex-direction: column;
        gap: 5px;
        font-size: 0.85rem;
        color: var(--text-dim);
    }

    /* Stats Grid */
    .stats-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
        gap: 15px;
        margin-bottom: 20px;
    }

    .stat-item {
        display: flex;
        flex-direction: column;
        gap: 5px;
        padding: 12px;
        background: var(--bg-dark);
        border-radius: 8px;
    }

    .stat-label {
        font-size: 0.8rem;
        color: var(--text-dim);
    }

    .stat-value {
        font-size: 1.2rem;
        font-weight: bold;
    }

    .stat-value.tir { color: var(--color-green); }
    .stat-value.tbr { color: var(--color-red); }
    .stat-value.tar { color: var(--color-yellow); }

    .data-info {
        display: flex;
        gap: 10px;
        font-size: 0.8rem;
        color: var(--text-dim);
        justify-content: center;
        padding-top: 10px;
        border-top: 1px solid #334155;
    }

    .no-params {
        text-align: center;
        color: var(--text-dim);
        padding: 40px;
    }

    @media (max-width: 900px) {
        .tod-grid {
            grid-template-columns: repeat(2, 1fr);
        }
    }
</style>
