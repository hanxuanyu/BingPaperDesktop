export namespace app {
	
	export class CurrentResult {
	    item: store.HistoryItem;
	    success: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new CurrentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.item = this.convertValues(source["item"], store.HistoryItem);
	        this.success = source["success"];
	        this.error = source["error"];
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

}

export namespace store {
	
	export class Config {
	    api_meta_url: string;
	    auto_apply: boolean;
	    overlay_metadata: boolean;
	    prefer_aspect_match: boolean;
	    force_uhd: boolean;
	    schedule_mode: string;
	    daily_time: string;
	    interval_minutes: number;
	    retain_days: number;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.api_meta_url = source["api_meta_url"];
	        this.auto_apply = source["auto_apply"];
	        this.overlay_metadata = source["overlay_metadata"];
	        this.prefer_aspect_match = source["prefer_aspect_match"];
	        this.force_uhd = source["force_uhd"];
	        this.schedule_mode = source["schedule_mode"];
	        this.daily_time = source["daily_time"];
	        this.interval_minutes = source["interval_minutes"];
	        this.retain_days = source["retain_days"];
	    }
	}
	export class HistoryItem {
	    key: string;
	    date: string;
	    title: string;
	    copyright: string;
	    chosen_variant: string;
	    image_path: string;
	    watermark_path: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new HistoryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.date = source["date"];
	        this.title = source["title"];
	        this.copyright = source["copyright"];
	        this.chosen_variant = source["chosen_variant"];
	        this.image_path = source["image_path"];
	        this.watermark_path = source["watermark_path"];
	        this.created_at = this.convertValues(source["created_at"], null);
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

}

