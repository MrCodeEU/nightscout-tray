export namespace models {
	
	export class ChartEntry {
	    time: number;
	    value: number;
	    valueMg: number;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new ChartEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.value = source["value"];
	        this.valueMg = source["valueMg"];
	        this.status = source["status"];
	    }
	}
	export class ChartData {
	    entries: ChartEntry[];
	    targetLow: number;
	    targetHigh: number;
	    urgentLow: number;
	    urgentHigh: number;
	    timeRangeHours: number;
	    unit: string;
	
	    static createFrom(source: any = {}) {
	        return new ChartData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entries = this.convertValues(source["entries"], ChartEntry);
	        this.targetLow = source["targetLow"];
	        this.targetHigh = source["targetHigh"];
	        this.urgentLow = source["urgentLow"];
	        this.urgentHigh = source["urgentHigh"];
	        this.timeRangeHours = source["timeRangeHours"];
	        this.unit = source["unit"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class GlucoseStatus {
	    value: number;
	    valueMmol: number;
	    trend: string;
	    direction: string;
	    // Go type: time
	    time: any;
	    delta: number;
	    status: string;
	    staleMinutes: number;
	    isStale: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GlucoseStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.valueMmol = source["valueMmol"];
	        this.trend = source["trend"];
	        this.direction = source["direction"];
	        this.time = this.convertValues(source["time"], null);
	        this.delta = source["delta"];
	        this.status = source["status"];
	        this.staleMinutes = source["staleMinutes"];
	        this.isStale = source["isStale"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Settings {
	    nightscoutUrl: string;
	    apiSecret: string;
	    apiToken: string;
	    useToken: boolean;
	    unit: string;
	    refreshInterval: number;
	    targetLow: number;
	    targetHigh: number;
	    urgentLow: number;
	    urgentHigh: number;
	    enableHighAlert: boolean;
	    enableLowAlert: boolean;
	    enableUrgentHighAlert: boolean;
	    enableUrgentLowAlert: boolean;
	    enableSoundAlerts: boolean;
	    repeatAlertMinutes: number;
	    chartTimeRange: number;
	    chartMaxHistory: number;
	    chartStyle: string;
	    chartColorInRange: string;
	    chartColorHigh: string;
	    chartColorLow: string;
	    chartColorUrgent: string;
	    chartShowTarget: boolean;
	    chartShowNow: boolean;
	    startMinimized: boolean;
	    autoStart: boolean;
	    showInTaskbar: boolean;
	    windowWidth: number;
	    windowHeight: number;
	    windowX: number;
	    windowY: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nightscoutUrl = source["nightscoutUrl"];
	        this.apiSecret = source["apiSecret"];
	        this.apiToken = source["apiToken"];
	        this.useToken = source["useToken"];
	        this.unit = source["unit"];
	        this.refreshInterval = source["refreshInterval"];
	        this.targetLow = source["targetLow"];
	        this.targetHigh = source["targetHigh"];
	        this.urgentLow = source["urgentLow"];
	        this.urgentHigh = source["urgentHigh"];
	        this.enableHighAlert = source["enableHighAlert"];
	        this.enableLowAlert = source["enableLowAlert"];
	        this.enableUrgentHighAlert = source["enableUrgentHighAlert"];
	        this.enableUrgentLowAlert = source["enableUrgentLowAlert"];
	        this.enableSoundAlerts = source["enableSoundAlerts"];
	        this.repeatAlertMinutes = source["repeatAlertMinutes"];
	        this.chartTimeRange = source["chartTimeRange"];
	        this.chartMaxHistory = source["chartMaxHistory"];
	        this.chartStyle = source["chartStyle"];
	        this.chartColorInRange = source["chartColorInRange"];
	        this.chartColorHigh = source["chartColorHigh"];
	        this.chartColorLow = source["chartColorLow"];
	        this.chartColorUrgent = source["chartColorUrgent"];
	        this.chartShowTarget = source["chartShowTarget"];
	        this.chartShowNow = source["chartShowNow"];
	        this.startMinimized = source["startMinimized"];
	        this.autoStart = source["autoStart"];
	        this.showInTaskbar = source["showInTaskbar"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	        this.windowX = source["windowX"];
	        this.windowY = source["windowY"];
	    }
	}

}

