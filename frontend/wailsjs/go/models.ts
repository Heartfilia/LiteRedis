export namespace config {
	
	export class AppSettings {
	    key_scan_count: number;
	    hash_load_count: number;
	    list_load_count: number;
	    set_load_count: number;
	    zset_load_count: number;
	    stream_load_count: number;
	    search_history_limit: number;
	    key_display_mode: string;
	    font_size_level: string;
	    watermark_enabled: boolean;
	    watermark_text: string;
	    watermark_size: number;
	    watermark_angle: number;
	    watermark_opacity: number;
	    watermark_density: number;
	    language: string;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key_scan_count = source["key_scan_count"];
	        this.hash_load_count = source["hash_load_count"];
	        this.list_load_count = source["list_load_count"];
	        this.set_load_count = source["set_load_count"];
	        this.zset_load_count = source["zset_load_count"];
	        this.stream_load_count = source["stream_load_count"];
	        this.search_history_limit = source["search_history_limit"];
	        this.key_display_mode = source["key_display_mode"];
	        this.font_size_level = source["font_size_level"];
	        this.watermark_enabled = source["watermark_enabled"];
	        this.watermark_text = source["watermark_text"];
	        this.watermark_size = source["watermark_size"];
	        this.watermark_angle = source["watermark_angle"];
	        this.watermark_opacity = source["watermark_opacity"];
	        this.watermark_density = source["watermark_density"];
	        this.language = source["language"];
	    }
	}
	export class SSHConfig {
	    host: string;
	    port: number;
	    user: string;
	    password: string;
	    private_key_path?: string;
	    passphrase?: string;
	
	    static createFrom(source: any = {}) {
	        return new SSHConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.password = source["password"];
	        this.private_key_path = source["private_key_path"];
	        this.passphrase = source["passphrase"];
	    }
	}
	export class ConnectionConfig {
	    id: string;
	    name: string;
	    group?: string;
	    host: string;
	    port: number;
	    password: string;
	    db: number;
	    is_cluster: boolean;
	    cluster_addrs?: string[];
	    proxy_enabled: boolean;
	    proxy_url?: string;
	    icon_color?: string;
	    ssh_enabled: boolean;
	    ssh?: SSHConfig;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.group = source["group"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.password = source["password"];
	        this.db = source["db"];
	        this.is_cluster = source["is_cluster"];
	        this.cluster_addrs = source["cluster_addrs"];
	        this.proxy_enabled = source["proxy_enabled"];
	        this.proxy_url = source["proxy_url"];
	        this.icon_color = source["icon_color"];
	        this.ssh_enabled = source["ssh_enabled"];
	        this.ssh = this.convertValues(source["ssh"], SSHConfig);
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
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
	export class CreateKeyRequest {
	    key: string;
	    type: string;
	    ttl: number;
	    string_value?: string;
	    field?: string;
	    value?: string;
	    list_value?: string;
	    member?: string;
	    score?: number;
	
	    static createFrom(source: any = {}) {
	        return new CreateKeyRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.type = source["type"];
	        this.ttl = source["ttl"];
	        this.string_value = source["string_value"];
	        this.field = source["field"];
	        this.value = source["value"];
	        this.list_value = source["list_value"];
	        this.member = source["member"];
	        this.score = source["score"];
	    }
	}
	export class StreamEntry {
	    id: string;
	    fields: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new StreamEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.fields = source["fields"];
	    }
	}
	export class ZSetMember {
	    member: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new ZSetMember(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.member = source["member"];
	        this.score = source["score"];
	    }
	}
	export class KeyValue {
	    key: string;
	    type: string;
	    ttl: number;
	    string_val?: string;
	    hash_val?: Record<string, string>;
	    list_val?: string[];
	    set_val?: string[];
	    zset_val?: ZSetMember[];
	    stream_val?: StreamEntry[];
	    has_more: boolean;
	    next_cursor: number;
	    next_offset: number;
	    total_count: number;
	
	    static createFrom(source: any = {}) {
	        return new KeyValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.type = source["type"];
	        this.ttl = source["ttl"];
	        this.string_val = source["string_val"];
	        this.hash_val = source["hash_val"];
	        this.list_val = source["list_val"];
	        this.set_val = source["set_val"];
	        this.zset_val = this.convertValues(source["zset_val"], ZSetMember);
	        this.stream_val = this.convertValues(source["stream_val"], StreamEntry);
	        this.has_more = source["has_more"];
	        this.next_cursor = source["next_cursor"];
	        this.next_offset = source["next_offset"];
	        this.total_count = source["total_count"];
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
	export class OperationResult {
	    success: boolean;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new OperationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	    }
	}
	export class RedisKey {
	    name: string;
	    type: string;
	    ttl: number;
	
	    static createFrom(source: any = {}) {
	        return new RedisKey(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.ttl = source["ttl"];
	    }
	}
	
	export class ScanResult {
	    keys: RedisKey[];
	    next_cursor: number;
	    has_more: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keys = this.convertValues(source["keys"], RedisKey);
	        this.next_cursor = source["next_cursor"];
	        this.has_more = source["has_more"];
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

export namespace main {
	
	export class ConnectResult {
	    success: boolean;
	    message?: string;
	    init_db: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.init_db = source["init_db"];
	    }
	}
	export class UpdateResult {
	    success: boolean;
	    message?: string;
	    release_url?: string;
	    asset_name?: string;
	    asset_path?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.release_url = source["release_url"];
	        this.asset_name = source["asset_name"];
	        this.asset_path = source["asset_path"];
	    }
	}
	export class VersionInfo {
	    version: string;
	    latest: string;
	    need_update: boolean;
	    release_url?: string;
	    checked_at?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new VersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.latest = source["latest"];
	        this.need_update = source["need_update"];
	        this.release_url = source["release_url"];
	        this.checked_at = source["checked_at"];
	        this.error = source["error"];
	    }
	}

}

