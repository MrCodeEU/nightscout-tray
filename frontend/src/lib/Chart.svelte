<script lang="ts">
    import { onMount, onDestroy } from 'svelte';

    // Use 'any' for data types to avoid Wails class vs interface conflicts
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    export let data: any = null;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    export let settings: any = null;

    let canvas: HTMLCanvasElement;
    let ctx: CanvasRenderingContext2D | null = null;
    let dpr = window.devicePixelRatio || 1;
    let width = 0;
    let height = 0;
    let resizeObserver: ResizeObserver | null = null;

    // Compute padding based on tray mode
    $: isTrayMode = settings?.trayMode === true;
    $: padding = isTrayMode
        ? { top: 8, right: 8, bottom: 20, left: 30 }
        : { top: 30, right: 20, bottom: 50, left: 50 };

    $: if (data && canvas) {
        render();
    }

    onMount(() => {
        ctx = canvas.getContext('2d');
        resizeObserver = new ResizeObserver(() => resize());
        const parent = canvas.parentElement;
        if (parent) {
            resizeObserver.observe(parent);
        }
        resize();
    });

    onDestroy(() => {
        if (resizeObserver) resizeObserver.disconnect();
    });

    function resize(): void {
        const parent = canvas.parentElement;
        if (!parent) return;

        const w = parent.clientWidth;
        const h = parent.clientHeight;

        if (Math.abs(width - w) < 2 && Math.abs(height - h) < 2) return;

        canvas.width = w * dpr;
        canvas.height = h * dpr;
        ctx?.scale(dpr, dpr);
        width = w;
        height = h;

        render();
    }

    function render(): void {
        if (!ctx || !data || !data.entries || data.entries.length === 0) {
            if (ctx) renderEmpty(ctx);
            return;
        }

        const c = ctx;
        const d = data;

        c.clearRect(0, 0, width, height);

        const chartWidth = width - padding.left - padding.right;
        const chartHeight = height - padding.top - padding.bottom;

        const entries = d.entries;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const times = entries.map((e: any) => e.time);
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const values = entries.map((e: any) => e.value);

        const minTime = Math.min(...times);
        const maxTime = Math.max(...times);

        const isMMol = d.unit === 'mmol/L';
        let minValue: number, maxValue: number;

        if (isMMol) {
            minValue = Math.min(2, Math.min(...values) - 1);
            maxValue = Math.max(20, Math.max(...values) + 1);
        } else {
            minValue = Math.min(40, Math.min(...values) - 20);
            maxValue = Math.max(300, Math.max(...values) + 20);
        }

        const scaleX = (time: number): number => padding.left + ((time - minTime) / (maxTime - minTime)) * chartWidth;
        const scaleY = (value: number): number => padding.top + (1 - (value - minValue) / (maxValue - minValue)) * chartHeight;

        if (settings?.chartShowTarget !== false) {
            drawTargetBand(c, d, scaleY, chartWidth, isMMol);
        }

        drawGrid(c, chartWidth, chartHeight, minTime, maxTime, minValue, maxValue, scaleX, scaleY, isMMol);
        drawThresholdLines(c, d, scaleY, chartWidth, isMMol);

        const style = settings?.chartStyle || 'both';
        if (style === 'line' || style === 'both') drawLine(c, entries, scaleX, scaleY);
        if (style === 'points' || style === 'both') drawPoints(c, entries, scaleX, scaleY);

        if (settings?.chartShowNow !== false && maxTime >= Date.now() - 3600000) {
            drawNowMarker(c, scaleX, chartHeight);
        }
    }

    function renderEmpty(c: CanvasRenderingContext2D): void {
        c.clearRect(0, 0, width, height);
        c.fillStyle = '#64748b';
        c.font = '14px sans-serif';
        c.textAlign = 'center';
        c.fillText('No data available', width / 2, height / 2);
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    function drawTargetBand(c: CanvasRenderingContext2D, d: any, scaleY: (v: number) => number, chartWidth: number, isMMol: boolean): void {
        let low = d.targetLow;
        let high = d.targetHigh;
        if (isMMol) { low /= 18.0182; high /= 18.0182; }
        const y1 = scaleY(high);
        const y2 = scaleY(low);
        c.fillStyle = 'rgba(74, 222, 128, 0.1)';
        c.fillRect(padding.left, y1, chartWidth, y2 - y1);
    }

    function drawGrid(c: CanvasRenderingContext2D, chartWidth: number, chartHeight: number, minTime: number, maxTime: number, minValue: number, maxValue: number, scaleX: (t: number) => number, scaleY: (v: number) => number, isMMol: boolean): void {
        c.strokeStyle = '#334155';
        c.lineWidth = 1;
        c.font = isTrayMode ? '9px sans-serif' : '11px sans-serif';
        c.fillStyle = '#64748b';

        let valueStep = isMMol ? 2 : 50;
        if (isTrayMode) valueStep = isMMol ? 4 : 100;

        const startValue = Math.ceil(minValue / valueStep) * valueStep;
        for (let v = startValue; v <= maxValue; v += valueStep) {
            const y = scaleY(v);
            c.beginPath(); c.moveTo(padding.left, y); c.lineTo(padding.left + chartWidth, y); c.stroke();
            c.textAlign = 'right'; c.textBaseline = 'middle';
            c.fillText(v.toString(), padding.left - 4, y);
        }

        const timeRange = maxTime - minTime;
        const hourMs = 3600000;
        let timeStep = hourMs;
        if (isTrayMode) {
            timeStep = 1.5 * hourMs;
        } else {
            if (timeRange > 12 * hourMs) timeStep = 2 * hourMs;
            if (timeRange > 24 * hourMs) timeStep = 4 * hourMs;
        }

        const startTime = Math.ceil(minTime / timeStep) * timeStep;
        for (let t = startTime; t <= maxTime; t += timeStep) {
            const x = scaleX(t);
            c.beginPath(); c.moveTo(x, padding.top); c.lineTo(x, padding.top + chartHeight); c.stroke();
            const date = new Date(t);
            const label = `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`;
            c.textAlign = 'center'; c.textBaseline = 'top';
            c.fillText(label, x, padding.top + chartHeight + 4);
        }
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    function drawThresholdLines(c: CanvasRenderingContext2D, d: any, scaleY: (v: number) => number, chartWidth: number, isMMol: boolean): void {
        const convert = (v: number): number => isMMol ? v / 18.0182 : v;
        c.setLineDash([5, 5]);
        c.strokeStyle = settings?.chartColorUrgent || '#ef4444';
        c.beginPath(); c.moveTo(padding.left, scaleY(convert(d.urgentHigh))); c.lineTo(padding.left+chartWidth, scaleY(convert(d.urgentHigh))); c.stroke();
        c.beginPath(); c.moveTo(padding.left, scaleY(convert(d.urgentLow))); c.lineTo(padding.left+chartWidth, scaleY(convert(d.urgentLow))); c.stroke();
        c.setLineDash([]);
        c.globalAlpha = 0.5;
        c.strokeStyle = settings?.chartColorHigh || '#facc15';
        c.beginPath(); c.moveTo(padding.left, scaleY(convert(d.targetHigh))); c.lineTo(padding.left+chartWidth, scaleY(convert(d.targetHigh))); c.stroke();
        c.strokeStyle = settings?.chartColorLow || '#f97316';
        c.beginPath(); c.moveTo(padding.left, scaleY(convert(d.targetLow))); c.lineTo(padding.left+chartWidth, scaleY(convert(d.targetLow))); c.stroke();
        c.globalAlpha = 1.0;
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    function drawLine(c: CanvasRenderingContext2D, entries: any[], scaleX: (t: number) => number, scaleY: (v: number) => number): void {
        c.lineWidth = 2;
        for (let i = 1; i < entries.length; i++) {
            c.beginPath();
            c.strokeStyle = getStatusColor(entries[i].status);
            c.moveTo(scaleX(entries[i-1].time), scaleY(entries[i-1].value));
            c.lineTo(scaleX(entries[i].time), scaleY(entries[i].value));
            c.stroke();
        }
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    function drawPoints(c: CanvasRenderingContext2D, entries: any[], scaleX: (t: number) => number, scaleY: (v: number) => number): void {
        const outerRadius = isTrayMode ? 3 : 5;
        const innerRadius = isTrayMode ? 2.5 : 4;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        entries.forEach((e: any) => {
            const x = scaleX(e.time), y = scaleY(e.value);
            c.beginPath(); c.arc(x, y, outerRadius, 0, Math.PI * 2); c.fillStyle = '#1e293b'; c.fill();
            c.beginPath(); c.arc(x, y, innerRadius, 0, Math.PI * 2); c.fillStyle = getStatusColor(e.status); c.fill();
        });
    }

    function drawNowMarker(c: CanvasRenderingContext2D, scaleX: (t: number) => number, chartHeight: number): void {
        const x = scaleX(Date.now());
        c.setLineDash([3, 3]); c.strokeStyle = '#f8fafc'; c.globalAlpha = 0.5;
        c.beginPath(); c.moveTo(x, padding.top); c.lineTo(x, padding.top + chartHeight); c.stroke();
        c.setLineDash([]); c.globalAlpha = 1.0; c.fillStyle = '#f8fafc'; c.font = '10px sans-serif'; c.textAlign = 'center';
        c.fillText('Now', x, padding.top - 8);
    }

    function getStatusColor(status: string): string {
        switch (status) {
            case 'urgent_low': case 'urgent_high': return settings?.chartColorUrgent || '#ef4444';
            case 'low': return settings?.chartColorLow || '#f97316';
            case 'high': return settings?.chartColorHigh || '#facc15';
            default: return settings?.chartColorInRange || '#4ade80';
        }
    }
</script>

<canvas bind:this={canvas} style="width: 100%; height: 100%;"></canvas>
