export namespace main {
	
	export class Config {
	    api_provider: string;
	    api_url: string;
	    api_key: string;
	    model_name: string;
	    max_tokens: number;
	    font_size: number;
	    hotkey: string;
	    display_mode: string;
	    prompt: string;
	    window_width: number;
	    window_height: number;
	    window_x: number;
	    window_y: number;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.api_provider = source["api_provider"];
	        this.api_url = source["api_url"];
	        this.api_key = source["api_key"];
	        this.model_name = source["model_name"];
	        this.max_tokens = source["max_tokens"];
	        this.font_size = source["font_size"];
	        this.hotkey = source["hotkey"];
	        this.display_mode = source["display_mode"];
	        this.prompt = source["prompt"];
	        this.window_width = source["window_width"];
	        this.window_height = source["window_height"];
	        this.window_x = source["window_x"];
	        this.window_y = source["window_y"];
	    }
	}
	export class HistoryEntry {
	    timestamp: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.content = source["content"];
	    }
	}

}

