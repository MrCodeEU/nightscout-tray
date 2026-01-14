<script lang="ts">
    import { onMount, onDestroy } from 'svelte';

    export let data = null;
    export let settings = {};

    let canvas;
    let ctx;
    let dpr = window.devicePixelRatio || 1;
    let width, height;
    let resizeObserver;

    // Compute padding based on tray mode
    $: isTrayMode = settings.trayMode === true;
    $: padding = isTrayMode
        ? { top: 8, right: 8, bottom: 20, left: 30 }
        : { top: 30, right: 20, bottom: 50, left: 50 };

    $: if (data && canvas) {
        render();
    }

    onMount(() => {
        ctx = canvas.getContext('2d');
        resizeObserver = new ResizeObserver(() => resize());
        resizeObserver.observe(canvas.parentElement);
        resize();
    });

    onDestroy(() => {
        if (resizeObserver) resizeObserver.disconnect();
    });

    function resize() {
        const parent = canvas.parentElement;
        const w = parent.clientWidth;
        const h = parent.clientHeight;

        if (Math.abs(width - w) < 2 && Math.abs(height - h) < 2) return;

        canvas.width = w * dpr;
        canvas.height = h * dpr;
        ctx.scale(dpr, dpr);
        width = w;
        height = h;

        render();
    }

    function render() {
        if (!ctx) return;
        if (!data || !data.entries || data.entries.length === 0) {
            renderEmpty();
            return;
        }

        ctx.clearRect(0, 0, width, height);

        const chartWidth = width - padding.left - padding.right;
        const chartHeight = height - padding.top - padding.bottom;

        const entries = data.entries;
        const times = entries.map(e => e.time);
        const values = entries.map(e => e.value);

        const minTime = Math.min(...times);
        const maxTime = Math.max(...times);

        const isMMol = data.unit === 'mmol/L';
        let minValue, maxValue;

        if (isMMol) {
            minValue = Math.min(2, Math.min(...values) - 1);
            maxValue = Math.max(20, Math.max(...values) + 1);
        } else {
            minValue = Math.min(40, Math.min(...values) - 20);
            maxValue = Math.max(300, Math.max(...values) + 20);
        }

        const scaleX = (time) => padding.left + ((time - minTime) / (maxTime - minTime)) * chartWidth;
        const scaleY = (value) => padding.top + (1 - (value - minValue) / (maxValue - minValue)) * chartHeight;

        if (settings.chartShowTarget !== false) {
            drawTargetBand(scaleY, chartWidth, isMMol);
        }

        drawGrid(chartWidth, chartHeight, minTime, maxTime, minValue, maxValue, scaleX, scaleY, isMMol);
        drawThresholdLines(scaleY, chartWidth, isMMol);

        const style = settings.chartStyle || 'both';
        if (style === 'line' || style === 'both') drawLine(entries, scaleX, scaleY);
        if (style === 'points' || style === 'both') drawPoints(entries, scaleX, scaleY);

        if (settings.chartShowNow !== false && maxTime >= Date.now() - 3600000) {
            drawNowMarker(scaleX, chartHeight);
        }
    }

    function renderEmpty() {
        ctx.clearRect(0, 0, width, height);
        ctx.fillStyle = '#64748b';
        ctx.font = '14px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText('No data available', width / 2, height / 2);
    }

    function drawTargetBand(scaleY, chartWidth, isMMol) {
        let low = data.targetLow;
        let high = data.targetHigh;
        if (isMMol) { low /= 18.0182; high /= 18.0182; }
        const y1 = scaleY(high);
        const y2 = scaleY(low);
        ctx.fillStyle = 'rgba(74, 222, 128, 0.1)';
        ctx.fillRect(padding.left, y1, chartWidth, y2 - y1);
    }

    function drawGrid(chartWidth, chartHeight, minTime, maxTime, minValue, maxValue, scaleX, scaleY, isMMol) {
        ctx.strokeStyle = '#334155';
        ctx.lineWidth = 1;
        ctx.font = isTrayMode ? '9px sans-serif' : '11px sans-serif';
        ctx.fillStyle = '#64748b';

        // Adjust value step for tray mode (fewer labels)
        let valueStep = isMMol ? 2 : 50;
        if (isTrayMode) valueStep = isMMol ? 4 : 100;

        const startValue = Math.ceil(minValue / valueStep) * valueStep;
        for (let v = startValue; v <= maxValue; v += valueStep) {
            const y = scaleY(v);
            ctx.beginPath(); ctx.moveTo(padding.left, y); ctx.lineTo(padding.left + chartWidth, y); ctx.stroke();
            ctx.textAlign = 'right'; ctx.textBaseline = 'middle';
            ctx.fillText(v.toString(), padding.left - 4, y);
        }

        const timeRange = maxTime - minTime;
        const hourMs = 3600000;
        let timeStep = hourMs;
        if (isTrayMode) {
            // For tray, use 1.5h step for 3h range
            timeStep = 1.5 * hourMs;
        } else {
            if (timeRange > 12 * hourMs) timeStep = 2 * hourMs;
            if (timeRange > 24 * hourMs) timeStep = 4 * hourMs;
        }

        const startTime = Math.ceil(minTime / timeStep) * timeStep;
        for (let t = startTime; t <= maxTime; t += timeStep) {
            const x = scaleX(t);
            ctx.beginPath(); ctx.moveTo(x, padding.top); ctx.lineTo(x, padding.top + chartHeight); ctx.stroke();
            const date = new Date(t);
            const label = `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
            ctx.textAlign = 'center'; ctx.textBaseline = 'top';
            ctx.fillText(label, x, padding.top + chartHeight + 4);
        }
    }

    function drawThresholdLines(scaleY, chartWidth, isMMol) {
        const convert = (v) => isMMol ? v / 18.0182 : v;
        ctx.setLineDash([5, 5]);
        ctx.strokeStyle = settings.chartColorUrgent || '#ef4444';
        ctx.beginPath(); ctx.moveTo(padding.left, scaleY(convert(data.urgentHigh))); ctx.lineTo(padding.left+chartWidth, scaleY(convert(data.urgentHigh))); ctx.stroke();
        ctx.beginPath(); ctx.moveTo(padding.left, scaleY(convert(data.urgentLow))); ctx.lineTo(padding.left+chartWidth, scaleY(convert(data.urgentLow))); ctx.stroke();
        ctx.setLineDash([]);
        ctx.globalAlpha = 0.5;
        ctx.strokeStyle = settings.chartColorHigh || '#facc15';
        ctx.beginPath(); ctx.moveTo(padding.left, scaleY(convert(data.targetHigh))); ctx.lineTo(padding.left+chartWidth, scaleY(convert(data.targetHigh))); ctx.stroke();
        ctx.strokeStyle = settings.chartColorLow || '#f97316';
        ctx.beginPath(); ctx.moveTo(padding.left, scaleY(convert(data.targetLow))); ctx.lineTo(padding.left+chartWidth, scaleY(convert(data.targetLow))); ctx.stroke();
        ctx.globalAlpha = 1.0;
    }

    function drawLine(entries, scaleX, scaleY) {
        ctx.lineWidth = 2;
        for (let i = 1; i < entries.length; i++) {
            ctx.beginPath();
            ctx.strokeStyle = getStatusColor(entries[i].status);
            ctx.moveTo(scaleX(entries[i-1].time), scaleY(entries[i-1].value));
            ctx.lineTo(scaleX(entries[i].time), scaleY(entries[i].value));
            ctx.stroke();
        }
    }

    function drawPoints(entries, scaleX, scaleY) {
        const outerRadius = isTrayMode ? 3 : 5;
        const innerRadius = isTrayMode ? 2.5 : 4;
        entries.forEach(e => {
            const x = scaleX(e.time), y = scaleY(e.value);
            ctx.beginPath(); ctx.arc(x, y, outerRadius, 0, Math.PI * 2); ctx.fillStyle = '#1e293b'; ctx.fill();
            ctx.beginPath(); ctx.arc(x, y, innerRadius, 0, Math.PI * 2); ctx.fillStyle = getStatusColor(e.status); ctx.fill();
        });
    }

    function drawNowMarker(scaleX, chartHeight) {
        const x = scaleX(Date.now());
        ctx.setLineDash([3, 3]); ctx.strokeStyle = '#f8fafc'; ctx.globalAlpha = 0.5;
        ctx.beginPath(); ctx.moveTo(x, padding.top); ctx.lineTo(x, padding.top + chartHeight); ctx.stroke();
        ctx.setLineDash([]); ctx.globalAlpha = 1.0; ctx.fillStyle = '#f8fafc'; ctx.font = '10px sans-serif'; ctx.textAlign = 'center';
        ctx.fillText('Now', x, padding.top - 8);
    }

    function getStatusColor(status) {
        switch (status) {
            case 'urgent_low': case 'urgent_high': return settings.chartColorUrgent || '#ef4444';
            case 'low': return settings.chartColorLow || '#f97316';
            case 'high': return settings.chartColorHigh || '#facc15';
            default: return settings.chartColorInRange || '#4ade80';
        }
    }
</script>

<canvas bind:this={canvas} style="width: 100%; height: 100%;"></canvas>
