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

