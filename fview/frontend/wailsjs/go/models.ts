export namespace frp {
	
	export class Conn {
	    name: string;
	    alias: string;
	    type: string;
	    serverAddr: string;
	    serverPort: number;
	    token: string;
	    serverName: string;
	    secretKey: string;
	    remotePort: number;
	    user: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new Conn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.alias = source["alias"];
	        this.type = source["type"];
	        this.serverAddr = source["serverAddr"];
	        this.serverPort = source["serverPort"];
	        this.token = source["token"];
	        this.serverName = source["serverName"];
	        this.secretKey = source["secretKey"];
	        this.remotePort = source["remotePort"];
	        this.user = source["user"];
	        this.path = source["path"];
	    }
	}

}

export namespace fview {
	
	export class FoldRange {
	    startSeq: number;
	    endSeq: number;
	    leader: string;
	
	    static createFrom(source: any = {}) {
	        return new FoldRange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.startSeq = source["startSeq"];
	        this.endSeq = source["endSeq"];
	        this.leader = source["leader"];
	    }
	}
	export class LineDTO {
	    seq: number;
	    time: string;
	    level: string;
	    thread: string;
	    traceId: string;
	    logger: string;
	    message: string;
	    raw: string;
	    source?: string;
	
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
	        this.source = source["source"];
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
	export class ResourceDTO {
	    kind: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ResourceDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	    }
	}
	export class TabInfo {
	    id: string;
	    label: string;
	    lines: number;
	    follow: boolean;
	    display: string;
	    info: string;
	
	    static createFrom(source: any = {}) {
	        return new TabInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.lines = source["lines"];
	        this.follow = source["follow"];
	        this.display = source["display"];
	        this.info = source["info"];
	    }
	}

}

export namespace main {
	
	export class DirEntryDTO {
	    name: string;
	    isDir: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DirEntryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.isDir = source["isDir"];
	    }
	}
	export class SSHHostDTO {
	    name: string;
	    fromSshConfig: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SSHHostDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.fromSshConfig = source["fromSshConfig"];
	    }
	}
	export class UpdateInfo {
	    current: string;
	    latest: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.url = source["url"];
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

