<script lang="ts">
    import { onMount } from 'svelte';
    import { Events } from "@wailsio/runtime";
    import { NightscoutService } from "../bindings/github.com/mrcode/nightscout-tray/internal/app";
    import Chart from './lib/Chart.svelte';

    let status = null;
    let settings = null;
    let chartData = null;
    let loaded = false;

    onMount(async () => {
        try {
            settings = await NightscoutService.GetSettings();
            status = await NightscoutService.GetCurrentStatus();

            if (settings) {
                // Simplified chart for tray - no labels, minimal padding
                const traySettings = {
                    ...settings,
                    chartShowTarget: true,
                    chartShowNow: false,
                    // Use compact mode for tray
                    trayMode: true
                };
                settings = traySettings;
                chartData = await NightscoutService.GetChartData(3, 0);
            }
            loaded = true;
        } catch (err) {
            console.error(err);
        }

        Events.On('glucose:update', (event) => {
            status = event.data;
            if (settings) {
                NightscoutService.GetChartData(3, 0).then(data => chartData = data);
            }
        });
    });

    const getStatusColor = (s) => {
        if (!s) return '#64748b';
        switch (s.status) {
            case 'urgent_low': case 'urgent_high': return '#ef4444';
            case 'low': return '#fb923c';
            case 'high': return '#facc15';
            default: return '#4ade80';
        }
    };

    const getStatusText = (s) => {
        if (!s) return '';
        switch (s.status) {
            case 'urgent_low': return 'URGENT LOW';
            case 'urgent_high': return 'URGENT HIGH';
            case 'low': return 'Low';
            case 'high': return 'High';
            default: return 'In Range';
        }
    };

    const formatTime = (date) => {
        if (!date) return '--:--';
        return new Date(date).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    };

    const getTimeSince = (date) => {
        if (!date) return '';
        const mins = Math.floor((Date.now() - new Date(date).getTime()) / 60000);
        if (mins < 1) return 'just now';
        if (mins === 1) return '1 min ago';
        if (mins < 60) return `${mins} min ago`;
        const hours = Math.floor(mins / 60);
        return `${hours}h ago`;
    };
</script>

<div class="tray-popup">
    {#if loaded && status}
        <!-- Header with glucose value -->
        <div class="header">
            <div class="glucose-display">
                <span class="value" style="color: {getStatusColor(status)}">
                    {settings?.unit === 'mmol/L' ? status.valueMmol.toFixed(1) : status.value}
                </span>
                <span class="unit">{settings?.unit || 'mg/dL'}</span>
                <span class="trend" style="color: {getStatusColor(status)}">{status.trend}</span>
            </div>
            <div class="meta">
                <span class="status-badge" style="background: {getStatusColor(status)}20; color: {getStatusColor(status)}">
                    {getStatusText(status)}
                </span>
                <span class="time" class:stale={status.isStale}>
                    {#if status.isStale}⚠️ {/if}{getTimeSince(status.time)}
                </span>
            </div>
        </div>

        <!-- Chart takes up most of the space -->
        <div class="chart-container">
            {#if chartData}
                <Chart data={chartData} settings={settings} />
            {:else}
                <div class="chart-loading">Loading...</div>
            {/if}
        </div>
    {:else}
        <div class="loading">
            <div class="spinner"></div>
            <span>Loading...</span>
        </div>
    {/if}
</div>

<style>
    :global(body) {
        margin: 0;
        padding: 0;
        background: transparent;
        font-family: 'Segoe UI', -apple-system, sans-serif;
        overflow: hidden;
    }

    .tray-popup {
        width: 100vw;
        height: 100vh;
        background: linear-gradient(180deg, #1e293b 0%, #0f172a 100%);
        color: white;
        display: flex;
        flex-direction: column;
        box-sizing: border-box;
        border: 1px solid #334155;
        border-radius: 8px;
        overflow: hidden;
    }

    .header {
        padding: 12px 14px 8px;
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        border-bottom: 1px solid #334155;
        background: rgba(30, 41, 59, 0.5);
    }

    .glucose-display {
        display: flex;
        align-items: baseline;
        gap: 4px;
    }

    .value {
        font-size: 36px;
        font-weight: 700;
        line-height: 1;
        letter-spacing: -1px;
    }

    .unit {
        font-size: 12px;
        color: #64748b;
        margin-left: 2px;
    }

    .trend {
        font-size: 24px;
        margin-left: 6px;
    }

    .meta {
        display: flex;
        flex-direction: column;
        align-items: flex-end;
        gap: 4px;
    }

    .status-badge {
        font-size: 10px;
        font-weight: 600;
        padding: 3px 8px;
        border-radius: 4px;
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }

    .time {
        font-size: 11px;
        color: #94a3b8;
    }

    .time.stale {
        color: #f97316;
    }

    .chart-container {
        flex: 1;
        min-height: 0;
        padding: 8px;
        padding-top: 4px;
    }

    .chart-loading {
        height: 100%;
        display: flex;
        align-items: center;
        justify-content: center;
        color: #64748b;
        font-size: 12px;
    }

    .loading {
        display: flex;
        flex-direction: column;
        align-items: center;
        justify-content: center;
        height: 100%;
        gap: 12px;
        color: #64748b;
    }

    .spinner {
        width: 24px;
        height: 24px;
        border: 2px solid #334155;
        border-top-color: #3b82f6;
        border-radius: 50%;
        animation: spin 1s linear infinite;
    }

    @keyframes spin {
        to { transform: rotate(360deg); }
    }
</style>
