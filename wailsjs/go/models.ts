export namespace main {
	
	export class ChromeProfileAccountInfoInput {
	    id: any;
	    access_token: string;
	
	    static createFrom(source: any = {}) {
	        return new ChromeProfileAccountInfoInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.access_token = source["access_token"];
	    }
	}
	export class ChromeProfileAccount {
	    id: string;
	    site_name: string;
	    site_url: string;
	    site_type: string;
	    api_key: string;
	    account_info: ChromeProfileAccountInfoInput;
	
	    static createFrom(source: any = {}) {
	        return new ChromeProfileAccount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.site_name = source["site_name"];
	        this.site_url = source["site_url"];
	        this.site_type = source["site_type"];
	        this.api_key = source["api_key"];
	        this.account_info = this.convertValues(source["account_info"], ChromeProfileAccountInfoInput);
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
	
	export class ChromeProfileTokenRequest {
	    accounts: ChromeProfileAccount[];
	
	    static createFrom(source: any = {}) {
	        return new ChromeProfileTokenRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accounts = this.convertValues(source["accounts"], ChromeProfileAccount);
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
	export class ChromeProfileTokenResult {
	    id: string;
	    site_name: string;
	    site_url: string;
	    tokens: any[];
	    error?: string;
	    resolved_access_token?: string;
	    resolved_user_id?: string;
	    storage_fields?: string[];
	    storage_origin?: string;
	
	    static createFrom(source: any = {}) {
	        return new ChromeProfileTokenResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.site_name = source["site_name"];
	        this.site_url = source["site_url"];
	        this.tokens = source["tokens"];
	        this.error = source["error"];
	        this.resolved_access_token = source["resolved_access_token"];
	        this.resolved_user_id = source["resolved_user_id"];
	        this.storage_fields = source["storage_fields"];
	        this.storage_origin = source["storage_origin"];
	    }
	}
	export class ChromeProfileTokenResponse {
	    results: ChromeProfileTokenResult[];
	    warnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ChromeProfileTokenResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.results = this.convertValues(source["results"], ChromeProfileTokenResult);
	        this.warnings = source["warnings"];
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
	
	export class DesktopLogContent {
	    path: string;
	    name: string;
	    content: string;
	    size: number;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new DesktopLogContent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.content = source["content"];
	        this.size = source["size"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class DesktopLogFileInfo {
	    groupKey: string;
	    groupLabel: string;
	    sourceKey: string;
	    sourceLabel: string;
	    name: string;
	    path: string;
	    size: number;
	    updatedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new DesktopLogFileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groupKey = source["groupKey"];
	        this.groupLabel = source["groupLabel"];
	        this.sourceKey = source["sourceKey"];
	        this.sourceLabel = source["sourceLabel"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.size = source["size"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class DesktopLogSnapshot {
	    files: DesktopLogFileInfo[];
	
	    static createFrom(source: any = {}) {
	        return new DesktopLogSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], DesktopLogFileInfo);
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
	export class ExtensionImportResult {
	    sourcePath: string;
	    storageKey: string;
	    accountCount: number;
	    payload: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new ExtensionImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourcePath = source["sourcePath"];
	        this.storageKey = source["storageKey"];
	        this.accountCount = source["accountCount"];
	        this.payload = source["payload"];
	    }
	}
	export class ManagedAppConfigAppliedFile {
	    appId: string;
	    fileId: string;
	    path: string;
	    backupPath: string;
	
	    static createFrom(source: any = {}) {
	        return new ManagedAppConfigAppliedFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appId = source["appId"];
	        this.fileId = source["fileId"];
	        this.path = source["path"];
	        this.backupPath = source["backupPath"];
	    }
	}
	export class ManagedAppConfigWrite {
	    appId: string;
	    fileId: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new ManagedAppConfigWrite(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appId = source["appId"];
	        this.fileId = source["fileId"];
	        this.content = source["content"];
	    }
	}
	export class ManagedAppConfigApplyRequest {
	    files: ManagedAppConfigWrite[];
	
	    static createFrom(source: any = {}) {
	        return new ManagedAppConfigApplyRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], ManagedAppConfigWrite);
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
	export class ManagedAppConfigApplyResult {
	    applied: ManagedAppConfigAppliedFile[];
	
	    static createFrom(source: any = {}) {
	        return new ManagedAppConfigApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.applied = this.convertValues(source["applied"], ManagedAppConfigAppliedFile);
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
	export class ManagedAppConfigFile {
	    appId: string;
	    appName: string;
	    fileId: string;
	    label: string;
	    path: string;
	    exists: boolean;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new ManagedAppConfigFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appId = source["appId"];
	        this.appName = source["appName"];
	        this.fileId = source["fileId"];
	        this.label = source["label"];
	        this.path = source["path"];
	        this.exists = source["exists"];
	        this.content = source["content"];
	    }
	}
	export class ManagedAppConfigSnapshot {
	    files: ManagedAppConfigFile[];
	
	    static createFrom(source: any = {}) {
	        return new ManagedAppConfigSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], ManagedAppConfigFile);
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

