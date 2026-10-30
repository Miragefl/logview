export namespace fview {
	
	export class LineDTO {
	    seq: number;
	    time: string;
	    level: string;
	    thread: string;
	    traceId: string;
	    logger: string;
	    message: string;
	    raw: string;
	
	    static createFrom(source: any = {}) {
	        return new LineDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seq = source["seq"];
	        this.time = source["time"];
	        this.level = source["level"];
	        this.thread = source["thread"];
	        this.traceId = source["traceId"];
	        this.logger = source["logger"];
	        this.message = source["message"];
	        this.raw = source["raw"];
	    }
	}
	export class QueryMeta {
	    parseError: string;
	    highlightKeywords: string[];
	    timeRangeHint: string;
	
	    static createFrom(source: any = {}) {
	        return new QueryMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.parseError = source["parseError"];
	        this.highlightKeywords = source["highlightKeywords"];
	        this.timeRangeHint = source["timeRangeHint"];
	    }
	}
	export class TabInfo {
	    id: string;
	    label: string;
	    lines: number;
	    follow: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TabInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.lines = source["lines"];
	        this.follow = source["follow"];
	    }
	}

}

export namespace parser {
	
	export class FieldConfig {
	    Name: string;
	    Visible: boolean;
	    Width: number;
	
	    static createFrom(source: any = {}) {
	        return new FieldConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Visible = source["Visible"];
	        this.Width = source["Width"];
	    }
	}

}

