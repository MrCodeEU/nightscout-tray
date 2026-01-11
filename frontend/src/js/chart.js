// Nightscout Tray - Chart Module
// Custom canvas-based chart implementation (no external dependencies)

export class Chart {
    constructor(canvas, settings) {
        this.canvas = canvas;
        this.ctx = canvas.getContext('2d');
        this.settings = settings || {};
        this.data = null;
        this.padding = { top: 30, right: 20, bottom: 50, left: 50 };
        
        // Handle high DPI displays
        this.dpr = window.devicePixelRatio || 1;
        
        // Setup resize observer
        this.resizeObserver = new ResizeObserver(() => this.resize());
        this.resizeObserver.observe(canvas.parentElement);
        
        this.resize();
    }
    
    resize() {
        const parent = this.canvas.parentElement;
        const width = parent.clientWidth;
        const height = parent.clientHeight;
        
        // Only resize if dimensions changed significantly
        if (Math.abs(this.width - width) < 2 && Math.abs(this.height - height) < 2) {
            return;
        }

        // Set canvas resolution (buffer size)
        this.canvas.width = width * this.dpr;
        this.canvas.height = height * this.dpr;
        
        this.ctx.scale(this.dpr, this.dpr);
        
        this.width = width;
        this.height = height;
        
        if (this.data) {
            this.render();
        }
    }
    
    update(data, settings) {
        this.data = data;
        this.settings = settings || this.settings;
        this.render();
    }
    
    render() {
        if (!this.data || !this.data.entries || this.data.entries.length === 0) {
            this.renderEmpty();
            return;
        }
        
        const ctx = this.ctx;
        const { width, height, padding } = this;
        
        // Clear canvas
        ctx.clearRect(0, 0, width, height);
        
        // Calculate chart area
        const chartWidth = width - padding.left - padding.right;
        const chartHeight = height - padding.top - padding.bottom;
        
        // Get data bounds
        const entries = this.data.entries;
        const times = entries.map(e => e.time);
        const values = entries.map(e => e.value);
        
        const minTime = Math.min(...times);
        const maxTime = Math.max(...times);
        
        // Determine value range based on unit
        const isMMol = this.data.unit === 'mmol/L';
        let minValue, maxValue;
        
        if (isMMol) {
            minValue = Math.min(2, Math.min(...values) - 1);
            maxValue = Math.max(20, Math.max(...values) + 1);
        } else {
            minValue = Math.min(40, Math.min(...values) - 20);
            maxValue = Math.max(300, Math.max(...values) + 20);
        }
        
        // Scale functions
        const scaleX = (time) => padding.left + ((time - minTime) / (maxTime - minTime)) * chartWidth;
        const scaleY = (value) => padding.top + (1 - (value - minValue) / (maxValue - minValue)) * chartHeight;
        
        // Draw target range band
        if (this.settings.chartShowTarget !== false) {
            this.drawTargetBand(ctx, scaleY, chartWidth, isMMol);
        }
        
        // Draw grid lines
        this.drawGrid(ctx, chartWidth, chartHeight, minTime, maxTime, minValue, maxValue, scaleX, scaleY, isMMol);
        
        // Draw threshold lines
        this.drawThresholdLines(ctx, scaleY, chartWidth, isMMol);
        
        // Draw data
        const style = this.settings.chartStyle || 'both';
        
        if (style === 'line' || style === 'both') {
            this.drawLine(ctx, entries, scaleX, scaleY);
        }
        
        if (style === 'points' || style === 'both') {
            this.drawPoints(ctx, entries, scaleX, scaleY);
        }
        
        // Draw current time marker
        if (this.settings.chartShowNow !== false && maxTime >= Date.now() - 60000) {
            this.drawNowMarker(ctx, scaleX, chartHeight);
        }
    }
    
    renderEmpty() {
        const ctx = this.ctx;
        ctx.clearRect(0, 0, this.width, this.height);
        
        ctx.fillStyle = '#64748b';
        ctx.font = '14px -apple-system, BlinkMacSystemFont, sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText('No data available', this.width / 2, this.height / 2);
    }
    
    drawTargetBand(ctx, scaleY, chartWidth, isMMol) {
        let targetLow = this.data.targetLow;
        let targetHigh = this.data.targetHigh;
        
        if (isMMol) {
            targetLow = targetLow / 18.0182;
            targetHigh = targetHigh / 18.0182;
        }
        
        const y1 = scaleY(targetHigh);
        const y2 = scaleY(targetLow);
        
        ctx.fillStyle = 'rgba(74, 222, 128, 0.1)';
        ctx.fillRect(this.padding.left, y1, chartWidth, y2 - y1);
    }
    
    drawGrid(ctx, chartWidth, chartHeight, minTime, maxTime, minValue, maxValue, scaleX, scaleY, isMMol) {
        ctx.strokeStyle = '#334155';
        ctx.lineWidth = 1;
        ctx.font = '11px -apple-system, BlinkMacSystemFont, sans-serif';
        ctx.fillStyle = '#64748b';
        
        // Horizontal grid lines (values)
        const valueStep = isMMol ? 2 : 50;
        const startValue = Math.ceil(minValue / valueStep) * valueStep;
        
        for (let v = startValue; v <= maxValue; v += valueStep) {
            const y = scaleY(v);
            
            ctx.beginPath();
            ctx.moveTo(this.padding.left, y);
            ctx.lineTo(this.padding.left + chartWidth, y);
            ctx.stroke();
            
            // Value label
            ctx.textAlign = 'right';
            ctx.textBaseline = 'middle';
            ctx.fillText(v.toString(), this.padding.left - 8, y);
        }
        
        // Vertical grid lines (time)
        const timeRange = maxTime - minTime;
        const hourMs = 60 * 60 * 1000;
        let timeStep = hourMs;
        
        if (timeRange > 12 * hourMs) {
            timeStep = 2 * hourMs;
        }
        if (timeRange > 24 * hourMs) {
            timeStep = 4 * hourMs;
        }
        
        const startTime = Math.ceil(minTime / timeStep) * timeStep;
        
        for (let t = startTime; t <= maxTime; t += timeStep) {
            const x = scaleX(t);
            
            ctx.beginPath();
            ctx.moveTo(x, this.padding.top);
            ctx.lineTo(x, this.padding.top + chartHeight);
            ctx.stroke();
            
            // Time label
            const date = new Date(t);
            const hours = date.getHours().toString().padStart(2, '0');
            const minutes = date.getMinutes().toString().padStart(2, '0');
            
            ctx.textAlign = 'center';
            ctx.textBaseline = 'top';
            ctx.fillText(`${hours}:${minutes}`, x, this.padding.top + chartHeight + 8);
        }
    }
    
    drawThresholdLines(ctx, scaleY, chartWidth, isMMol) {
        const convert = (v) => isMMol ? v / 18.0182 : v;
        
        // Urgent thresholds (dashed)
        ctx.setLineDash([5, 5]);
        
        // Urgent high
        ctx.strokeStyle = this.settings.chartColorUrgent || '#ef4444';
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.moveTo(this.padding.left, scaleY(convert(this.data.urgentHigh)));
        ctx.lineTo(this.padding.left + chartWidth, scaleY(convert(this.data.urgentHigh)));
        ctx.stroke();
        
        // Urgent low
        ctx.beginPath();
        ctx.moveTo(this.padding.left, scaleY(convert(this.data.urgentLow)));
        ctx.lineTo(this.padding.left + chartWidth, scaleY(convert(this.data.urgentLow)));
        ctx.stroke();
        
        // Target thresholds (solid)
        ctx.setLineDash([]);
        ctx.lineWidth = 1;
        
        // Target high
        ctx.strokeStyle = this.settings.chartColorHigh || '#facc15';
        ctx.globalAlpha = 0.5;
        ctx.beginPath();
        ctx.moveTo(this.padding.left, scaleY(convert(this.data.targetHigh)));
        ctx.lineTo(this.padding.left + chartWidth, scaleY(convert(this.data.targetHigh)));
        ctx.stroke();
        
        // Target low
        ctx.strokeStyle = this.settings.chartColorLow || '#f97316';
        ctx.beginPath();
        ctx.moveTo(this.padding.left, scaleY(convert(this.data.targetLow)));
        ctx.lineTo(this.padding.left + chartWidth, scaleY(convert(this.data.targetLow)));
        ctx.stroke();
        
        ctx.globalAlpha = 1;
    }
    
    drawLine(ctx, entries, scaleX, scaleY) {
        if (entries.length < 2) return;
        
        ctx.lineWidth = 2;
        ctx.lineCap = 'round';
        ctx.lineJoin = 'round';
        
        // Draw segments with different colors based on status
        for (let i = 1; i < entries.length; i++) {
            const prev = entries[i - 1];
            const curr = entries[i];
            
            ctx.beginPath();
            ctx.strokeStyle = this.getStatusColor(curr.status);
            ctx.moveTo(scaleX(prev.time), scaleY(prev.value));
            ctx.lineTo(scaleX(curr.time), scaleY(curr.value));
            ctx.stroke();
        }
    }
    
    drawPoints(ctx, entries, scaleX, scaleY) {
        const radius = 4;
        
        entries.forEach(entry => {
            const x = scaleX(entry.time);
            const y = scaleY(entry.value);
            
            // Outer circle (dark border)
            ctx.beginPath();
            ctx.arc(x, y, radius + 1, 0, Math.PI * 2);
            ctx.fillStyle = '#1e293b';
            ctx.fill();
            
            // Inner circle (colored)
            ctx.beginPath();
            ctx.arc(x, y, radius, 0, Math.PI * 2);
            ctx.fillStyle = this.getStatusColor(entry.status);
            ctx.fill();
        });
    }
    
    drawNowMarker(ctx, scaleX, chartHeight) {
        const x = scaleX(Date.now());
        
        ctx.setLineDash([3, 3]);
        ctx.strokeStyle = '#f8fafc';
        ctx.lineWidth = 1;
        ctx.globalAlpha = 0.5;
        
        ctx.beginPath();
        ctx.moveTo(x, this.padding.top);
        ctx.lineTo(x, this.padding.top + chartHeight);
        ctx.stroke();
        
        ctx.setLineDash([]);
        ctx.globalAlpha = 1;
        
        // "Now" label
        ctx.fillStyle = '#f8fafc';
        ctx.font = '10px -apple-system, BlinkMacSystemFont, sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText('Now', x, this.padding.top - 8);
    }
    
    getStatusColor(status) {
        switch (status) {
            case 'urgent_low':
            case 'urgent_high':
                return this.settings.chartColorUrgent || '#ef4444';
            case 'low':
                return this.settings.chartColorLow || '#f97316';
            case 'high':
                return this.settings.chartColorHigh || '#facc15';
            default:
                return this.settings.chartColorInRange || '#4ade80';
        }
    }
    
    destroy() {
        if (this.resizeObserver) {
            this.resizeObserver.disconnect();
        }
    }
}
