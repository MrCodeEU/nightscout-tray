// Nightscout Tray - Utility Functions

/**
 * Format a date as a relative time string (e.g., "5 minutes ago")
 * @param {Date} date - The date to format
 * @returns {string} - Formatted relative time
 */
export function formatTimeAgo(date) {
    if (!date || !(date instanceof Date) || isNaN(date)) {
        return '--';
    }
    
    const now = new Date();
    const diffMs = now - date;
    const diffSeconds = Math.floor(diffMs / 1000);
    const diffMinutes = Math.floor(diffSeconds / 60);
    const diffHours = Math.floor(diffMinutes / 60);
    const diffDays = Math.floor(diffHours / 24);
    
    if (diffSeconds < 60) {
        return 'just now';
    }
    if (diffMinutes === 1) {
        return '1 minute ago';
    }
    if (diffMinutes < 60) {
        return `${diffMinutes} minutes ago`;
    }
    if (diffHours === 1) {
        return '1 hour ago';
    }
    if (diffHours < 24) {
        return `${diffHours} hours ago`;
    }
    if (diffDays === 1) {
        return 'yesterday';
    }
    return `${diffDays} days ago`;
}

/**
 * Format a date as a localized date/time string
 * @param {Date} date - The date to format
 * @returns {string} - Formatted date/time
 */
export function formatDateTime(date) {
    if (!date || !(date instanceof Date) || isNaN(date)) {
        return '--';
    }
    
    return date.toLocaleString(undefined, {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
    });
}

/**
 * Format a time as HH:MM
 * @param {Date} date - The date to format
 * @returns {string} - Formatted time
 */
export function formatTime(date) {
    if (!date || !(date instanceof Date) || isNaN(date)) {
        return '--';
    }
    
    return date.toLocaleTimeString(undefined, {
        hour: '2-digit',
        minute: '2-digit'
    });
}

/**
 * Debounce a function
 * @param {Function} func - Function to debounce
 * @param {number} wait - Milliseconds to wait
 * @returns {Function} - Debounced function
 */
export function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

/**
 * Throttle a function
 * @param {Function} func - Function to throttle
 * @param {number} limit - Milliseconds between calls
 * @returns {Function} - Throttled function
 */
export function throttle(func, limit) {
    let inThrottle;
    return function(...args) {
        if (!inThrottle) {
            func.apply(this, args);
            inThrottle = true;
            setTimeout(() => inThrottle = false, limit);
        }
    };
}

/**
 * Convert mg/dL to mmol/L
 * @param {number} mgdl - Value in mg/dL
 * @returns {number} - Value in mmol/L
 */
export function mgdlToMmol(mgdl) {
    return mgdl / 18.0182;
}

/**
 * Convert mmol/L to mg/dL
 * @param {number} mmol - Value in mmol/L
 * @returns {number} - Value in mg/dL
 */
export function mmolToMgdl(mmol) {
    return mmol * 18.0182;
}

/**
 * Check if a value is within a range
 * @param {number} value - Value to check
 * @param {number} min - Minimum value
 * @param {number} max - Maximum value
 * @returns {boolean} - True if within range
 */
export function inRange(value, min, max) {
    return value >= min && value <= max;
}

/**
 * Clamp a value between min and max
 * @param {number} value - Value to clamp
 * @param {number} min - Minimum value
 * @param {number} max - Maximum value
 * @returns {number} - Clamped value
 */
export function clamp(value, min, max) {
    return Math.min(Math.max(value, min), max);
}

/**
 * Parse a hex color to RGB values
 * @param {string} hex - Hex color string (e.g., "#ff0000")
 * @returns {{r: number, g: number, b: number}} - RGB values
 */
export function hexToRgb(hex) {
    const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
    return result ? {
        r: parseInt(result[1], 16),
        g: parseInt(result[2], 16),
        b: parseInt(result[3], 16)
    } : null;
}

/**
 * Convert RGB to hex color
 * @param {number} r - Red value (0-255)
 * @param {number} g - Green value (0-255)
 * @param {number} b - Blue value (0-255)
 * @returns {string} - Hex color string
 */
export function rgbToHex(r, g, b) {
    return '#' + [r, g, b].map(x => {
        const hex = x.toString(16);
        return hex.length === 1 ? '0' + hex : hex;
    }).join('');
}

/**
 * Generate a unique ID
 * @returns {string} - Unique ID
 */
export function generateId() {
    return Date.now().toString(36) + Math.random().toString(36).substr(2);
}

/**
 * Sleep for a specified duration
 * @param {number} ms - Milliseconds to sleep
 * @returns {Promise} - Promise that resolves after the duration
 */
export function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
}
