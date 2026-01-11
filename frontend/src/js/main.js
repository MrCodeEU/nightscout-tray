// Nightscout Tray - Main JavaScript

import '../css/main.css';
import { Chart } from './chart.js';
import { formatTimeAgo, formatDateTime, debounce } from './utils.js';

// App state
const state = {
    settings: null,
    currentStatus: null,
    chart: null,
    chartHours: 8,
    historyOffset: 0,
    maxHistoryDays: 7,
    isConfigured: false
};

// DOM Elements
const elements = {};

// Initialize the application
async function init() {
    // Wait for Wails runtime to be ready
    if (!window.go) {
        console.log('Waiting for Wails runtime...');
        await new Promise(resolve => {
            const check = setInterval(() => {
                if (window.go) {
                    clearInterval(check);
                    resolve();
                }
            }, 100);
            // Timeout after 2s
            setTimeout(() => {
                clearInterval(check);
                resolve();
            }, 2000);
        });
    }

    cacheElements();
    setupEventListeners();
    await loadSettings();
    await checkConfiguration();
    
    if (state.isConfigured) {
        await refreshData();
    }
    
    setupWailsEvents();

    // Disable zoom
    disableZoom();
}

// Disable browser zooming
function disableZoom() {
    document.addEventListener('keydown', function(e) {
        if ((e.ctrlKey || e.metaKey) && (e.key === '+' || e.key === '-' || e.key === '=')) {
            e.preventDefault();
        }
    });

    document.addEventListener('wheel', function(e) {
        if (e.ctrlKey || e.metaKey) {
            e.preventDefault();
        }
    }, { passive: false });
}

// Cache DOM elements
function cacheElements() {
    // Dashboard elements
    elements.glucoseValue = document.getElementById('glucose-value');
    elements.glucoseTrend = document.getElementById('glucose-trend');
    elements.glucoseStatus = document.getElementById('glucose-status');
    elements.glucoseDelta = document.getElementById('glucose-delta');
    elements.glucoseTime = document.getElementById('glucose-time');
    elements.statusCard = document.querySelector('.status-card');
    
    // Chart elements
    elements.chartCanvas = document.getElementById('glucose-chart');
    elements.historyRange = document.getElementById('history-range');
    elements.btnHistoryBack = document.getElementById('btn-history-back');
    elements.btnHistoryForward = document.getElementById('btn-history-forward');
    elements.timeButtons = document.querySelectorAll('.time-btn');
    
    // Settings elements
    elements.nightscoutUrl = document.getElementById('nightscout-url');
    elements.useToken = document.getElementById('use-token');
    elements.apiSecret = document.getElementById('api-secret');
    elements.apiToken = document.getElementById('api-token');
    elements.fieldApiSecret = document.getElementById('field-api-secret');
    elements.fieldApiToken = document.getElementById('field-api-token');
    elements.connectionStatus = document.getElementById('connection-status');
    elements.unit = document.getElementById('unit');
    elements.refreshInterval = document.getElementById('refresh-interval');
    elements.urgentLow = document.getElementById('urgent-low');
    elements.targetLow = document.getElementById('target-low');
    elements.targetHigh = document.getElementById('target-high');
    elements.urgentHigh = document.getElementById('urgent-high');
    elements.enableUrgentLowAlert = document.getElementById('enable-urgent-low-alert');
    elements.enableLowAlert = document.getElementById('enable-low-alert');
    elements.enableHighAlert = document.getElementById('enable-high-alert');
    elements.enableUrgentHighAlert = document.getElementById('enable-urgent-high-alert');
    elements.enableSoundAlerts = document.getElementById('enable-sound-alerts');
    elements.repeatAlertMinutes = document.getElementById('repeat-alert-minutes');
    elements.chartStyle = document.getElementById('chart-style');
    elements.chartTimeRange = document.getElementById('chart-time-range');
    elements.chartMaxHistory = document.getElementById('chart-max-history');
    elements.chartShowTarget = document.getElementById('chart-show-target');
    elements.chartShowNow = document.getElementById('chart-show-now');
    elements.colorInRange = document.getElementById('color-in-range');
    elements.colorHigh = document.getElementById('color-high');
    elements.colorLow = document.getElementById('color-low');
    elements.colorUrgent = document.getElementById('color-urgent');
    elements.startMinimized = document.getElementById('start-minimized');
    elements.autoStart = document.getElementById('auto-start');
    elements.saveStatus = document.getElementById('save-status');
    elements.unitDisplay = document.querySelector('.unit-display');
    
    // Buttons
    elements.btnTestConnection = document.getElementById('btn-test-connection');
    elements.btnTestNotification = document.getElementById('btn-test-notification');
    elements.btnSaveSettings = document.getElementById('btn-save-settings');
    elements.btnRefresh = document.getElementById('btn-refresh');
    elements.btnMinimize = document.getElementById('btn-minimize');
    elements.navButtons = document.querySelectorAll('.nav-btn');
    
    // Views and overlays
    elements.viewDashboard = document.getElementById('view-dashboard');
    elements.viewSettings = document.getElementById('view-settings');
    elements.overlayNotConfigured = document.getElementById('overlay-not-configured');
}

// Setup event listeners
function setupEventListeners() {
    // Navigation
    elements.navButtons.forEach(btn => {
        btn.addEventListener('click', () => switchView(btn.dataset.view));
    });
    
    // Header buttons
    elements.btnRefresh.addEventListener('click', refreshData);
    elements.btnMinimize.addEventListener('click', hideWindow);
    
    // Settings - Connection
    elements.useToken.addEventListener('change', toggleAuthMode);
    elements.btnTestConnection.addEventListener('click', testConnection);
    elements.btnTestNotification.addEventListener('click', testNotification);
    elements.btnSaveSettings.addEventListener('click', saveSettings);
    
    // Settings - Unit change
    elements.unit.addEventListener('change', updateUnitDisplay);
    
    // Chart controls
    elements.timeButtons.forEach(btn => {
        btn.addEventListener('click', () => {
            state.chartHours = parseInt(btn.dataset.hours);
            state.historyOffset = 0;
            updateTimeButtons();
            updateChart();
        });
    });
    
    elements.btnHistoryBack.addEventListener('click', () => {
        state.historyOffset += state.chartHours;
        const maxOffset = state.maxHistoryDays * 24 - state.chartHours;
        if (state.historyOffset > maxOffset) {
            state.historyOffset = maxOffset;
        }
        updateHistoryNav();
        updateChart();
    });
    
    elements.btnHistoryForward.addEventListener('click', () => {
        state.historyOffset -= state.chartHours;
        if (state.historyOffset < 0) {
            state.historyOffset = 0;
        }
        updateHistoryNav();
        updateChart();
    });
}

// Setup Wails event listeners
function setupWailsEvents() {
    if (typeof window.runtime !== 'undefined') {
        window.runtime.EventsOn('glucose:update', (status) => {
            updateGlucoseDisplay(status);
        });
        
        window.runtime.EventsOn('glucose:error', (error) => {
            console.error('Glucose error:', error);
            showError(error);
        });
    }
}

// Load settings from backend
async function loadSettings() {
    try {
        const settings = await window.go.app.App.GetSettings();
        state.settings = settings;
        populateSettingsForm(settings);
        applyColors(settings);
        state.chartHours = settings.chartTimeRange || 8;
        state.maxHistoryDays = settings.chartMaxHistory || 7;
        updateTimeButtons();
    } catch (err) {
        console.error('Failed to load settings:', err);
    }
}

// Check if app is configured
async function checkConfiguration() {
    try {
        state.isConfigured = await window.go.app.App.IsConfigured();
        
        // Hide overlay if configured or if currently in settings view
        const inSettings = elements.viewSettings.classList.contains('active');
        if (state.isConfigured || inSettings) {
            elements.overlayNotConfigured.classList.add('hidden');
        } else {
            elements.overlayNotConfigured.classList.remove('hidden');
        }
    } catch (err) {
        console.error('Failed to check configuration:', err);
    }
}

// Populate settings form
function populateSettingsForm(settings) {
    elements.nightscoutUrl.value = settings.nightscoutUrl || '';
    elements.useToken.checked = settings.useToken || false;
    elements.apiSecret.value = settings.apiSecret || '';
    elements.apiToken.value = settings.apiToken || '';
    toggleAuthMode();
    
    elements.unit.value = settings.unit || 'mg/dL';
    elements.refreshInterval.value = settings.refreshInterval || 60;
    
    elements.urgentLow.value = settings.urgentLow || 55;
    elements.targetLow.value = settings.targetLow || 70;
    elements.targetHigh.value = settings.targetHigh || 180;
    elements.urgentHigh.value = settings.urgentHigh || 250;
    
    elements.enableUrgentLowAlert.checked = settings.enableUrgentLowAlert !== false;
    elements.enableLowAlert.checked = settings.enableLowAlert !== false;
    elements.enableHighAlert.checked = settings.enableHighAlert !== false;
    elements.enableUrgentHighAlert.checked = settings.enableUrgentHighAlert !== false;
    elements.enableSoundAlerts.checked = settings.enableSoundAlerts !== false;
    elements.repeatAlertMinutes.value = settings.repeatAlertMinutes || 15;
    
    elements.chartStyle.value = settings.chartStyle || 'both';
    elements.chartTimeRange.value = settings.chartTimeRange || 8;
    elements.chartMaxHistory.value = settings.chartMaxHistory || 7;
    elements.chartShowTarget.checked = settings.chartShowTarget !== false;
    elements.chartShowNow.checked = settings.chartShowNow !== false;
    
    elements.colorInRange.value = settings.chartColorInRange || '#4ade80';
    elements.colorHigh.value = settings.chartColorHigh || '#facc15';
    elements.colorLow.value = settings.chartColorLow || '#f97316';
    elements.colorUrgent.value = settings.chartColorUrgent || '#ef4444';
    
    elements.startMinimized.checked = settings.startMinimized !== false;
    elements.autoStart.checked = settings.autoStart || false;
    
    updateUnitDisplay();
}

// Toggle between API secret and token auth
function toggleAuthMode() {
    const useToken = elements.useToken.checked;
    elements.fieldApiSecret.classList.toggle('hidden', useToken);
    elements.fieldApiToken.classList.toggle('hidden', !useToken);
}

// Update unit display in threshold section
function updateUnitDisplay() {
    const unit = elements.unit.value;
    elements.unitDisplay.textContent = `(${unit})`;
    
    // Convert threshold values if unit changed
    // TODO: Implement conversion logic
}

// Apply colors from settings to CSS variables
function applyColors(settings) {
    document.documentElement.style.setProperty('--color-in-range', settings.chartColorInRange || '#4ade80');
    document.documentElement.style.setProperty('--color-high', settings.chartColorHigh || '#facc15');
    document.documentElement.style.setProperty('--color-low', settings.chartColorLow || '#f97316');
    document.documentElement.style.setProperty('--color-urgent', settings.chartColorUrgent || '#ef4444');
}

// Test Nightscout connection
async function testConnection() {
    const url = elements.nightscoutUrl.value;
    const secret = elements.apiSecret.value;
    const token = elements.apiToken.value;
    const useToken = elements.useToken.checked;
    
    if (!url) {
        showConnectionStatus('Please enter a Nightscout URL', false);
        return;
    }
    
    elements.btnTestConnection.disabled = true;
    elements.btnTestConnection.textContent = 'Testing...';
    
    try {
        await window.go.app.App.TestConnection(url, secret, token, useToken);
        showConnectionStatus('Connection successful!', true);
    } catch (err) {
        showConnectionStatus(`Connection failed: ${err}`, false);
    } finally {
        elements.btnTestConnection.disabled = false;
        elements.btnTestConnection.textContent = 'Test Connection';
    }
}

// Show connection status
function showConnectionStatus(message, success) {
    elements.connectionStatus.textContent = message;
    elements.connectionStatus.className = 'connection-status ' + (success ? 'success' : 'error');
    
    setTimeout(() => {
        elements.connectionStatus.textContent = '';
        elements.connectionStatus.className = 'connection-status';
    }, 5000);
}

// Test notification
async function testNotification() {
    try {
        await window.go.app.App.SendTestNotification();
    } catch (err) {
        console.error('Test notification failed:', err);
    }
}

// Save settings
async function saveSettings() {
    const settings = {
        nightscoutUrl: elements.nightscoutUrl.value,
        useToken: elements.useToken.checked,
        apiSecret: elements.apiSecret.value,
        apiToken: elements.apiToken.value,
        unit: elements.unit.value,
        refreshInterval: parseInt(elements.refreshInterval.value),
        urgentLow: parseInt(elements.urgentLow.value),
        targetLow: parseInt(elements.targetLow.value),
        targetHigh: parseInt(elements.targetHigh.value),
        urgentHigh: parseInt(elements.urgentHigh.value),
        enableUrgentLowAlert: elements.enableUrgentLowAlert.checked,
        enableLowAlert: elements.enableLowAlert.checked,
        enableHighAlert: elements.enableHighAlert.checked,
        enableUrgentHighAlert: elements.enableUrgentHighAlert.checked,
        enableSoundAlerts: elements.enableSoundAlerts.checked,
        repeatAlertMinutes: parseInt(elements.repeatAlertMinutes.value),
        chartStyle: elements.chartStyle.value,
        chartTimeRange: parseInt(elements.chartTimeRange.value),
        chartMaxHistory: parseInt(elements.chartMaxHistory.value),
        chartShowTarget: elements.chartShowTarget.checked,
        chartShowNow: elements.chartShowNow.checked,
        chartColorInRange: elements.colorInRange.value,
        chartColorHigh: elements.colorHigh.value,
        chartColorLow: elements.colorLow.value,
        chartColorUrgent: elements.colorUrgent.value,
        startMinimized: elements.startMinimized.checked,
        autoStart: elements.autoStart.checked
    };
    
    elements.btnSaveSettings.disabled = true;
    elements.btnSaveSettings.textContent = 'Saving...';
    
    try {
        await window.go.app.App.SaveSettings(settings);
        state.settings = settings;
        applyColors(settings);
        state.maxHistoryDays = settings.chartMaxHistory;
        
        elements.saveStatus.textContent = 'Settings saved!';
        setTimeout(() => {
            elements.saveStatus.textContent = '';
        }, 3000);
        
        await checkConfiguration();
        if (state.isConfigured) {
            await refreshData();
        }
    } catch (err) {
        console.error('Failed to save settings:', err);
        elements.saveStatus.textContent = 'Error saving settings';
    } finally {
        elements.btnSaveSettings.disabled = false;
        elements.btnSaveSettings.textContent = 'Save Settings';
    }
}

// Refresh glucose data
async function refreshData() {
    if (!state.isConfigured) return;
    
    elements.btnRefresh.classList.add('loading');
    
    try {
        await window.go.app.App.ForceRefresh();
        const status = await window.go.app.App.GetCurrentStatus();
        if (status) {
            updateGlucoseDisplay(status);
        }
        await updateChart();
    } catch (err) {
        console.error('Failed to refresh:', err);
    } finally {
        elements.btnRefresh.classList.remove('loading');
    }
}

// Update glucose display
function updateGlucoseDisplay(status) {
    if (!status) return;
    
    const unit = state.settings?.unit || 'mg/dL';
    let valueText;
    
    if (unit === 'mmol/L') {
        valueText = status.valueMmol.toFixed(1);
    } else {
        valueText = status.value.toString();
    }
    
    elements.glucoseValue.textContent = valueText;
    elements.glucoseTrend.textContent = status.trend || '-';
    
    // Update status badge
    const statusText = formatStatus(status.status);
    elements.glucoseStatus.textContent = statusText;
    elements.glucoseStatus.className = `status-value badge badge-${status.status}`;
    
    // Update card styling
    elements.statusCard.className = `status-card status-${status.status}`;
    
    // Update delta
    if (status.delta !== undefined && status.delta !== 0) {
        const deltaPrefix = status.delta > 0 ? '+' : '';
        if (unit === 'mmol/L') {
            elements.glucoseDelta.textContent = `${deltaPrefix}${(status.delta / 18.0182).toFixed(1)}`;
        } else {
            elements.glucoseDelta.textContent = `${deltaPrefix}${status.delta}`;
        }
    } else {
        elements.glucoseDelta.textContent = '--';
    }
    
    // Update time
    if (status.time) {
        const time = new Date(status.time);
        elements.glucoseTime.textContent = formatTimeAgo(time);
    }
    
    state.currentStatus = status;
}

// Format status string
function formatStatus(status) {
    const statusMap = {
        'normal': 'In Range',
        'high': 'High',
        'low': 'Low',
        'urgent_high': 'Urgent High',
        'urgent_low': 'Urgent Low'
    };
    return statusMap[status] || status;
}

// Update chart
async function updateChart() {
    if (!state.isConfigured) return;
    
    try {
        const data = await window.go.app.App.GetChartData(state.chartHours, state.historyOffset);
        if (data) {
            if (!state.chart) {
                state.chart = new Chart(elements.chartCanvas, state.settings);
            }
            state.chart.update(data, state.settings);
        }
    } catch (err) {
        console.error('Failed to update chart:', err);
    }
}

// Update time range buttons
function updateTimeButtons() {
    elements.timeButtons.forEach(btn => {
        btn.classList.toggle('active', parseInt(btn.dataset.hours) === state.chartHours);
    });
}

// Update history navigation
function updateHistoryNav() {
    const maxOffset = state.maxHistoryDays * 24 - state.chartHours;
    
    elements.btnHistoryBack.disabled = state.historyOffset >= maxOffset;
    elements.btnHistoryForward.disabled = state.historyOffset <= 0;
    
    if (state.historyOffset === 0) {
        elements.historyRange.textContent = 'Now';
    } else {
        const hoursAgo = state.historyOffset;
        if (hoursAgo < 24) {
            elements.historyRange.textContent = `${hoursAgo}h ago`;
        } else {
            const daysAgo = Math.floor(hoursAgo / 24);
            elements.historyRange.textContent = `${daysAgo}d ago`;
        }
    }
}

// Switch view
function switchView(viewName) {
    elements.navButtons.forEach(btn => {
        btn.classList.toggle('active', btn.dataset.view === viewName);
    });
    
    elements.viewDashboard.classList.toggle('active', viewName === 'dashboard');
    elements.viewSettings.classList.toggle('active', viewName === 'settings');

    // Manage overlay visibility
    if (viewName === 'settings') {
        elements.overlayNotConfigured.classList.add('hidden');
    } else if (viewName === 'dashboard' && !state.isConfigured) {
        elements.overlayNotConfigured.classList.remove('hidden');
    }
    
    // Initialize chart when switching to dashboard
    if (viewName === 'dashboard' && state.isConfigured) {
        setTimeout(() => updateChart(), 100);
    }
}

// Hide window
function hideWindow() {
    if (typeof window.go !== 'undefined') {
        window.go.app.App.HideWindow();
    }
}

// Show error
function showError(message) {
    // Could implement a toast notification system here
    console.error(message);
}

// Make switchView global for overlay button
window.switchView = switchView;

// Initialize on DOM ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
} else {
    init();
}
