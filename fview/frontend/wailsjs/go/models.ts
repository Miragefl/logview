export namespace frp {
	
	export class Conn {
	    name: string;
	    server: string;
	    sk: string;
	    proxy: string;
	    user: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new Conn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.server = source["server"];
	        this.sk = source["sk"];
	        this.proxy = source["proxy"];
	        this.user = source["user"];
	        this.path = source["path"];
	    }
	}
	export class Server {
	    name: string;
	    addr: string;
	    token: string;
	
	    static createFrom(source: any = {}) {
	        return new Server(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.addr = source["addr"];
	        this.token = source["token"];
	    }
	}

}

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
	export class RuleConfig {
	    Name: string;
	    Pattern: string;
	    Parse: string;
	    Fallback: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RuleConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Name = source["Name"];
	        this.Pattern = source["Pattern"];
	        this.Parse = source["Parse"];
	        this.Fallback = source["Fallback"];
	    }
	}

}

